# Low-Level Keyboard Hook + Configurable Shortcuts — Design Spec

**Date:** 2026-04-06
**Issue:** #30 — Shortcut flow broken on Windows (follow-up)
**Status:** Draft
**Supersedes:** `2026-04-06-shortcut-double-press-design.md` (partial — replaces backend input capture)

## Problem

The initial double-press detection implementation uses `RegisterHotKey` for input capture. `RegisterHotKey` fires `WM_HOTKEY` only when the full modifier+key combo is pressed from scratch. When the user holds Ctrl and taps G a second time, Windows does NOT fire a second `WM_HOTKEY` — instead the raw G keydown passes through to the foreground app, typing "g". This makes double-press detection unusable with natural hand gestures (hold Ctrl, tap G twice).

Additionally, the shortcut is hardcoded to `Ctrl+G` with no user configuration.

## Design

### Win32: Replace `RegisterHotKey` with `WH_KEYBOARD_LL`

Replace the entire `service_windows.go` implementation. Instead of `RegisterHotKey`, install a low-level keyboard hook via `SetWindowsHookEx(WH_KEYBOARD_LL, ...)`.

**How it works:**

- The hook callback receives every keydown/keyup event system-wide, before any app sees them.
- The service tracks modifier key state (Ctrl, Shift, Alt, Win) internally from keydown/keyup events.
- When a keydown matches a configured shortcut's trigger key and the active modifiers match, the service:
  1. Suppresses the keypress (returns 1 from the hook proc so the foreground app never sees it)
  2. Emits a `ShortcutEvent` on the channel with the matched action
- Non-matching keypresses pass through unchanged (return `CallNextHookEx`).

**Modifier tracking:**

The hook maintains a bitmask of currently-held modifier keys, updated on every keydown/keyup for VK_CONTROL, VK_SHIFT, VK_MENU (Alt), and VK_LWIN/VK_RWIN. This is necessary because `WH_KEYBOARD_LL` receives raw key events, not combo events.

**Thread model:**

`SetWindowsHookEx(WH_KEYBOARD_LL)` requires a message pump on the installing thread. The existing `runtime.LockOSThread()` + `GetMessageW` loop pattern is reused. The hook callback runs on the same thread.

### Double-Tap Detection

In double-tap mode, the trigger key (e.g., G) can be tapped once or twice while modifiers are held.

**State machine (runs inside the hook callback thread):**

```
idle → [modifier+trigger keydown] → suppress key, capture clipboard, start timer → waiting
waiting → [same trigger keydown, modifiers still held] → suppress key, cancel timer → emit pyramidize → idle
waiting → [timer expires] → emit fix → idle
waiting → [modifier released] → cancel timer, emit fix immediately → idle
```

**Key behaviors:**

- **First trigger keydown:** suppressed immediately (the foreground app never sees it), clipboard captured, 200ms timer starts.
- **Second trigger keydown (within window, modifiers still held):** suppressed, timer cancelled, `pyramidize` action emitted immediately.
- **Modifier released before timer expires:** timer cancelled, `fix` action emitted immediately. This feels natural — lifting off Ctrl means "I'm done."
- **Timer expires:** `fix` action emitted.
- **Non-trigger keys during the wait window:** pass through unchanged. The user can type other keys; only the configured trigger key is intercepted.

Since the hook callback and timer run on the same OS thread (message-pump thread), there are no concurrency issues with the state machine — all state mutations are single-threaded.

### Independent Mode

When double-tap mode is disabled, two separate shortcut bindings are active. Each fires its action immediately on keydown — no timer, no state machine. The hook checks each keydown against both bindings and emits the matching action.

### Shortcut Configuration

**Settings schema additions:**

```json
{
  "shortcut_mode": "double_tap",
  "shortcut_fix": "ctrl+g",
  "shortcut_pyramidize": "ctrl+shift+g",
  "shortcut_double_tap_delay": 200
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `shortcut_mode` | `"double_tap" \| "independent"` | `"double_tap"` | Detection mode |
| `shortcut_fix` | string | `"ctrl+g"` | Shortcut for silent fix. In double-tap mode, this is the base combo (single tap). |
| `shortcut_pyramidize` | string | `"ctrl+shift+g"` | Shortcut for pyramidize. Only used in independent mode. In double-tap mode, pyramidize is triggered by double-tapping the trigger key of `shortcut_fix`. |
| `shortcut_double_tap_delay` | int (ms) | `200` | Detection window. Range: 100–500. Only used in double-tap mode. |

**Key format:** lowercase, `+`-separated. Modifiers: `ctrl`, `shift`, `alt`, `win`. Trigger: single key name (`g`, `f8`, `;`, etc.). Examples: `ctrl+g`, `ctrl+shift+e`, `f8`, `alt+k`.

**Parsing:** A utility function parses the string into a struct with a modifier bitmask and a virtual key code. This is used both by the hook (to match keypresses) and by the settings UI (to display formatted key names).

### ShortcutEvent Changes

The `ShortcutEvent` struct gains an `Action` field:

```go
type ShortcutEvent struct {
    Source string // "hotkey" | "simulate"
    Action string // "fix" | "pyramidize"
}
```

This replaces the current design where `main.go` emits `shortcut:single` and `shortcut:double` — instead the event carries the semantic action directly. The Wails events become `shortcut:fix` and `shortcut:pyramidize`.

### `main.go` Changes

The shortcut goroutine simplifies. The `Detector` from the current implementation is removed — double-tap detection now lives inside the hook's message-pump thread (single-threaded, no channels needed for timing). The goroutine just reads classified events from the channel and emits the appropriate Wails event:

```
go func() {
    for event := range services.Shortcut.Triggered() {
        switch event.Action {
        case "fix":
            emit("shortcut:fix", "hotkey")
        case "pyramidize":
            window.Show().Focus()
            emit("shortcut:pyramidize", "hotkey")
        }
    }
}()
```

Clipboard capture (`CaptureSourceApp` + `CopyFromForeground`) moves into the shortcut service itself, called on the first trigger keydown before emitting the event. This keeps the timing tight — clipboard is captured at the OS hook level, not after a channel round-trip.

### Service Interface Changes

```go
type Service interface {
    Register(cfg ShortcutConfig) error
    Unregister()
    Triggered() <-chan ShortcutEvent
    UpdateConfig(cfg ShortcutConfig) error
}

type ShortcutConfig struct {
    Mode            string        // "double_tap" | "independent"
    FixCombo        string        // e.g. "ctrl+g"
    PyramidizeCombo string        // e.g. "ctrl+shift+g"
    DoubleTapDelay  time.Duration // e.g. 200ms
}
```

`Register` now takes a config. `UpdateConfig` allows hot-reloading shortcuts when settings change (reinstalls the hook with new bindings — no app restart needed).

### Frontend Changes

**`wails.service.ts`:** Rename `shortcutSingle$` → `shortcutFix$`, `shortcutDouble$` → `shortcutPyramidize$`. These map to `shortcut:fix` and `shortcut:pyramidize` Wails events.

**`wails-mock.ts`:** Update mock subjects to match.

**`fix.component.ts`:** Subscribe to `shortcutFix$`.

**`text-enhancement.component.ts`:** Subscribe to `shortcutPyramidize$`.

**`shell.component.ts`:** Navigate to `/enhance` on `shortcutPyramidize$`.

**`message-bus.service.ts`:** Update event type union.

### Settings UI

The existing `shortcut_key` text input in the General tab is replaced with an expanded shortcuts section.

**Double-tap mode ON (default):**

- Toggle row: "Double-tap mode" label + hint ("Hold your modifier keys, tap the trigger key once for Fix, twice for Pyramidize.") + toggle switch.
- Form group: "Shortcut" label + shortcut recorder field showing formatted key combo (e.g., "Ctrl + G") + "Record..." button. Below: hint "Single tap → Fix · Double tap → Pyramidize".
- Form group: "Double-tap delay" label + slider (100–500ms) showing live value. Below: hint "How long to wait for a second tap. Lower = faster but harder to trigger."

**Double-tap mode OFF:**

- Same toggle row, hint changes to "Assign separate shortcuts for each action."
- Form group: "Fix shortcut" + recorder + hint "Silently fixes clipboard text."
- Form group: "Pyramidize shortcut" + recorder + hint "Opens the Pyramidize editor with clipboard text."
- Delay slider hidden.

**Shortcut recorder:** A compact input-like element. Click "Record..." → it enters capture mode (visual indicator, e.g., pulsing border), captures the next key combo, displays it formatted, exits capture mode. Pressing Escape cancels recording.

The recorder is a reusable subcomponent: `shortcut-recorder/shortcut-recorder.component.ts` colocated under the settings folder.

## Testing

### Go Unit Tests

- **Key format parser:** parse `"ctrl+g"` → modifiers + vkCode, roundtrip, edge cases (standalone `f8`, triple modifier `ctrl+shift+alt+k`).
- **Config validation:** reject invalid combos, empty strings, unknown key names.
- **Double-tap state machine:** not directly unit-testable since it lives in the hook callback (single-threaded, OS-level). Tested via integration/manual on Windows.

### Frontend Vitest Tests

- **Shortcut recorder component:** renders, enters/exits capture mode, displays formatted combo, cancel on Escape.
- **Settings shortcuts section:** toggle between modes, correct fields shown/hidden, slider range.
- **WailsService:** `shortcutFix$` and `shortcutPyramidize$` emit correctly.
- **Component subscriptions:** fix subscribes to `shortcutFix$`, text-enhancement to `shortcutPyramidize$`, shell navigates on `shortcutPyramidize$`.

### Manual QA (Windows)

- Hold Ctrl, tap G once → silent fix fires after delay.
- Hold Ctrl, tap G twice quickly → pyramidize opens.
- Hold Ctrl, tap G once, release Ctrl quickly → fix fires immediately (no delay).
- Switch to independent mode, verify both shortcuts fire independently.
- Record new shortcut combo in settings, verify it takes effect without restart.
- Verify suppressed keys don't leak to foreground app.

## Acceptance Criteria

- [ ] `RegisterHotKey` replaced with `WH_KEYBOARD_LL` low-level hook
- [ ] Double-tap mode: hold modifier + tap trigger once = fix, twice = pyramidize
- [ ] Modifier release during detection window cancels timer and fires fix immediately
- [ ] Independent mode: two separate shortcuts, no timer
- [ ] Settings UI with mode toggle, shortcut recorder, and delay slider
- [ ] Hot-reload: changing shortcuts in settings takes effect without app restart
- [ ] Suppressed keys never reach the foreground app
- [ ] All existing frontend tests updated and passing
- [ ] Shortcut recorder component with capture mode
