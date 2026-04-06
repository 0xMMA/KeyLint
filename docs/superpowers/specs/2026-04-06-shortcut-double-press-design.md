# Shortcut Double-Press Detection — Design Spec

**Date:** 2026-04-06
**Issue:** #30 — Shortcut flow broken on Windows
**Status:** Draft

## Problem

Every Ctrl+G press emits the same `shortcut:triggered` event. There is no single vs double press detection. Both `FixComponent` and `TextEnhancementComponent` independently subscribe to this event, so behavior depends on which route is active rather than user intent. This causes:

1. Pyramidize captures clipboard on single press if `/enhance` is the active route
2. Window doesn't focus on double press (no double press concept exists)
3. Silent fix leaks into Pyramidize state when the UI was previously open but hidden

## Design

### Go Backend: State Machine in `main.go`

The shortcut goroutine becomes a state machine with a 200ms double-press detection window.

**States:**

```
idle → [press] → waiting (start 200ms timer, capture clipboard immediately)
waiting → [press within 200ms] → double detected → focus window → emit shortcut:double → idle
waiting → [timer expires] → single detected → emit shortcut:single → idle
```

**Key behaviors:**

- **Clipboard capture happens on the first press**, before the timer starts. This is critical because the source app still has focus at this point. `CaptureSourceApp()` and `CopyFromForeground()` run immediately.
- **The second press in a double-press is consumed.** It does not restart the timer or trigger a new cycle.
- **200ms threshold** matches the v1 Tauri implementation. This introduces a 200ms delay before single-press silent fix fires, which is imperceptible in the fix-and-paste-back flow.
- **Window focus on double press:** The Go handler calls `window.Show()` and brings the window to front before emitting `shortcut:double`.

**Events emitted:**

| Event | Payload | When |
|---|---|---|
| `shortcut:single` | source app name (string) | 200ms timer expires with no second press |
| `shortcut:double` | source app name (string) | Second press arrives within 200ms |

The old `shortcut:triggered` event is removed entirely.

### Frontend: Two Observables

**`wails.service.ts`:**

- Remove `shortcutTriggered` / `shortcutTriggered$`
- Add `shortcutSingle$`: Observable listening to `shortcut:single`
- Add `shortcutDouble$`: Observable listening to `shortcut:double`

**`fix.component.ts`:**

- Subscribe to `shortcutSingle$` instead of `shortcutTriggered$`
- No other logic changes needed

**`text-enhancement.component.ts`:**

- Subscribe to `shortcutDouble$` instead of `shortcutTriggered$`
- No other logic changes needed — it already loads clipboard and resets UI

**`shell.component.ts`:**

- Add subscription to `shortcutDouble$` that navigates to `/enhance`
- This lives in ShellComponent because it's always mounted regardless of the current route. TextEnhancementComponent may not be mounted when the double press arrives.

### State Machine Implementation Detail

The state machine lives in the shortcut goroutine in `main.go`. It uses a `time.Timer` for the detection window.

```
var pressTimer *time.Timer
var pendingSource string

for {
    select {
    case event := <-shortcutCh:
        if pressTimer != nil {
            // Second press within window → double
            pressTimer.Stop()
            pressTimer = nil
            focusWindow()
            emit("shortcut:double", pendingSource)
        } else {
            // First press → capture clipboard, start timer
            captureSourceApp()
            copyFromForeground()
            pendingSource = event.Source
            pressTimer = time.AfterFunc(200ms, func() {
                pressTimer = nil
                emit("shortcut:single", pendingSource)
            })
        }
    }
}
```

This is pseudocode — the real implementation will handle the `AfterFunc` callback thread-safely since it runs on a separate goroutine.

## Testing

### Go Unit Tests

Test the state machine logic directly (no real hotkeys needed):

- **Single press:** send one event, verify `shortcut:single` emitted after ~200ms, `shortcut:double` not emitted
- **Double press:** send two events within 200ms, verify `shortcut:double` emitted immediately, `shortcut:single` not emitted
- **Triple press:** send three rapid events, verify double fires on second press, third press starts a new cycle
- **Slow double press:** send two events 300ms apart, verify two `shortcut:single` events (not a double)

### Frontend Vitest Tests

- **`WailsService`:** mock Wails events for `shortcut:single` and `shortcut:double`, verify correct observables emit
- **`FixComponent`:** verify subscribes to `shortcutSingle$` only, calls `fixClipboard()` on emit
- **`TextEnhancementComponent`:** verify subscribes to `shortcutDouble$` only, loads clipboard on emit
- **`ShellComponent`:** verify navigates to `/enhance` on `shortcutDouble$`

### Not Tested

- E2E: global hotkeys cannot be triggered from Playwright
- Real timing on Windows: relies on manual QA

## Acceptance Criteria (from #30)

- [ ] Single Ctrl+G press always triggers silent fix, regardless of active route or window visibility
- [ ] Double Ctrl+G press (within ~200ms) opens Pyramidize UI, brings window to front, and loads clipboard into editor
- [ ] Pyramidize page does not capture clipboard on single press
- [ ] Window correctly receives focus on double press
