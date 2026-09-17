# Low-Level Keyboard Hook + Configurable Shortcuts — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `RegisterHotKey` with `WH_KEYBOARD_LL` low-level keyboard hook to support natural double-tap detection (hold modifier, tap trigger key twice) and configurable per-feature shortcuts.

**Architecture:** The Windows shortcut service is rewritten to use `SetWindowsHookEx(WH_KEYBOARD_LL)` with internal modifier tracking and a `SetTimer`-based double-tap state machine. A key combo parser converts string formats like `"ctrl+g"` to modifier bitmask + virtual key code. Settings gain per-feature shortcut fields with double-tap/independent mode toggle. The `Detector` goroutine-based approach is removed — all detection is single-threaded on the hook's message-pump thread.

**Tech Stack:** Go 1.26 (Win32 syscalls: user32.dll), Wails v3, Wire DI, Angular v21, PrimeNG v21, Vitest

**Spec:** `docs/superpowers/specs/2026-04-06-keyboard-hook-configurable-shortcuts-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `internal/features/shortcut/keycombo.go` | Parse key combo strings ↔ modifier bitmask + virtual key code |
| Create | `internal/features/shortcut/keycombo_test.go` | Unit tests for parser |
| Modify | `internal/features/shortcut/service.go` | Updated interface: `Register(ShortcutConfig)`, `UpdateConfig`, `ShortcutEvent.Action` |
| Rewrite | `internal/features/shortcut/service_windows.go` | `WH_KEYBOARD_LL` hook, modifier tracking, double-tap state machine |
| Modify | `internal/features/shortcut/service_linux.go` | Updated to match new interface |
| Delete | `internal/features/shortcut/detect.go` | No longer needed — detection in hook thread |
| Delete | `internal/features/shortcut/detect_test.go` | No longer needed |
| Modify | `internal/features/settings/model.go` | Add `ShortcutMode`, `ShortcutFix`, `ShortcutPyramidize`, `ShortcutDoubleTapDelay` |
| Modify | `internal/app/wire.go` | Update shortcut provider to pass config |
| Modify | `main.go` | Simplify shortcut goroutine, remove Detector + atomic, update event names |
| Modify | `frontend/src/app/core/wails.service.ts` | Rename `shortcutSingle$` → `shortcutFix$`, `shortcutDouble$` → `shortcutPyramidize$` |
| Modify | `frontend/src/testing/wails-mock.ts` | Update mock subjects |
| Modify | `frontend/src/app/core/message-bus.service.ts` | Update event type union |
| Modify | `frontend/src/app/features/fix/fix.component.ts` | Subscribe to `shortcutFix$` |
| Modify | `frontend/src/app/features/fix/fix.component.spec.ts` | Update tests |
| Modify | `frontend/src/app/features/text-enhancement/text-enhancement.component.ts` | Subscribe to `shortcutPyramidize$` |
| Modify | `frontend/src/app/features/text-enhancement/text-enhancement.component.spec.ts` | Update tests |
| Modify | `frontend/src/app/layout/shell.component.ts` | Subscribe to `shortcutPyramidize$` |
| Modify | `frontend/src/app/layout/shell.component.spec.ts` | Update tests |
| Create | `frontend/src/app/features/settings/shortcut-recorder/shortcut-recorder.component.ts` | Reusable key combo recorder |
| Create | `frontend/src/app/features/settings/shortcut-recorder/shortcut-recorder.component.spec.ts` | Tests |
| Modify | `frontend/src/app/features/settings/settings.component.ts` | Replace shortcut_key input with shortcuts section |
| Modify | `frontend/src/app/features/settings/settings.component.spec.ts` | Tests for shortcuts section |

---

### Task 1: Key combo parser — test and implementation

**Files:**
- Create: `internal/features/shortcut/keycombo.go`
- Create: `internal/features/shortcut/keycombo_test.go`

A pure Go utility that parses `"ctrl+g"` → `KeyCombo{Modifiers: ModCtrl, VK: 0x47}` and formats back to display string `"Ctrl + G"`. Also maps key names to Windows virtual key codes.

- [ ] **Step 1: Write the failing tests**

Create `internal/features/shortcut/keycombo_test.go`:

```go
package shortcut

import "testing"

func TestParseKeyCombo_CtrlG(t *testing.T) {
	kc, err := ParseKeyCombo("ctrl+g")
	if err != nil {
		t.Fatal(err)
	}
	if kc.Modifiers != ModCtrl {
		t.Fatalf("expected ModCtrl (%d), got %d", ModCtrl, kc.Modifiers)
	}
	if kc.VK != 0x47 {
		t.Fatalf("expected VK 0x47, got 0x%X", kc.VK)
	}
}

func TestParseKeyCombo_CtrlShiftE(t *testing.T) {
	kc, err := ParseKeyCombo("ctrl+shift+e")
	if err != nil {
		t.Fatal(err)
	}
	if kc.Modifiers != ModCtrl|ModShift {
		t.Fatalf("expected ModCtrl|ModShift (%d), got %d", ModCtrl|ModShift, kc.Modifiers)
	}
	if kc.VK != 0x45 {
		t.Fatalf("expected VK 0x45 (E), got 0x%X", kc.VK)
	}
}

func TestParseKeyCombo_StandaloneF8(t *testing.T) {
	kc, err := ParseKeyCombo("f8")
	if err != nil {
		t.Fatal(err)
	}
	if kc.Modifiers != 0 {
		t.Fatalf("expected no modifiers, got %d", kc.Modifiers)
	}
	if kc.VK != 0x77 {
		t.Fatalf("expected VK 0x77 (F8), got 0x%X", kc.VK)
	}
}

func TestParseKeyCombo_TripleModifier(t *testing.T) {
	kc, err := ParseKeyCombo("ctrl+shift+alt+k")
	if err != nil {
		t.Fatal(err)
	}
	if kc.Modifiers != ModCtrl|ModShift|ModAlt {
		t.Fatalf("expected all three modifiers, got %d", kc.Modifiers)
	}
	if kc.VK != 0x4B {
		t.Fatalf("expected VK 0x4B (K), got 0x%X", kc.VK)
	}
}

func TestParseKeyCombo_Invalid(t *testing.T) {
	_, err := ParseKeyCombo("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
	_, err = ParseKeyCombo("ctrl+")
	if err == nil {
		t.Fatal("expected error for trailing +")
	}
	_, err = ParseKeyCombo("ctrl+shift")
	if err == nil {
		t.Fatal("expected error for modifiers-only combo")
	}
	_, err = ParseKeyCombo("ctrl+unknownkey")
	if err == nil {
		t.Fatal("expected error for unknown key name")
	}
}

func TestKeyCombo_DisplayString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ctrl+g", "Ctrl + G"},
		{"ctrl+shift+e", "Ctrl + Shift + E"},
		{"f8", "F8"},
		{"alt+f4", "Alt + F4"},
	}
	for _, tt := range tests {
		kc, err := ParseKeyCombo(tt.input)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.input, err)
		}
		got := kc.DisplayString()
		if got != tt.expected {
			t.Errorf("DisplayString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestKeyCombo_String(t *testing.T) {
	kc, _ := ParseKeyCombo("ctrl+shift+g")
	got := kc.String()
	if got != "ctrl+shift+g" {
		t.Errorf("String() = %q, want %q", got, "ctrl+shift+g")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/features/shortcut/ -run TestParseKeyCombo -v`
Expected: compilation error — `ParseKeyCombo`, `KeyCombo`, `ModCtrl` not defined.

- [ ] **Step 3: Write the implementation**

Create `internal/features/shortcut/keycombo.go`:

```go
package shortcut

import (
	"fmt"
	"strings"
)

// Modifier bitmask flags matching Win32 modifier virtual key codes.
type Modifier uint8

const (
	ModCtrl  Modifier = 1 << iota // VK_CONTROL (0x11)
	ModShift                      // VK_SHIFT   (0x10)
	ModAlt                        // VK_MENU    (0x12)
	ModWin                        // VK_LWIN    (0x5B)
)

// KeyCombo represents a parsed keyboard shortcut (modifier bitmask + trigger key).
type KeyCombo struct {
	Modifiers Modifier
	VK        uint16 // Windows virtual key code
	KeyName   string // lowercase key name, e.g. "g", "f8"
}

// modifierNames maps string names to modifier flags.
var modifierNames = map[string]Modifier{
	"ctrl":  ModCtrl,
	"shift": ModShift,
	"alt":   ModAlt,
	"win":   ModWin,
}

// modifierDisplay is the display-order list of modifiers.
var modifierDisplay = []struct {
	Flag Modifier
	Name string
}{
	{ModCtrl, "Ctrl"},
	{ModShift, "Shift"},
	{ModAlt, "Alt"},
	{ModWin, "Win"},
}

// keyNames maps lowercase key names to Windows virtual key codes.
var keyNames = map[string]uint16{
	// Letters A-Z
	"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45,
	"f": 0x46, "g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A,
	"k": 0x4B, "l": 0x4C, "m": 0x4D, "n": 0x4E, "o": 0x4F,
	"p": 0x50, "q": 0x51, "r": 0x52, "s": 0x53, "t": 0x54,
	"u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58, "y": 0x59, "z": 0x5A,
	// Digits 0-9
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
	// Function keys F1-F12
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73,
	"f5": 0x74, "f6": 0x75, "f7": 0x76, "f8": 0x77,
	"f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
	// Punctuation / special
	";": 0xBA, "=": 0xBB, ",": 0xBC, "-": 0xBD, ".": 0xBE,
	"/": 0xBF, "`": 0xC0, "[": 0xDB, "\\": 0xDC, "]": 0xDD, "'": 0xDE,
	"space": 0x20, "enter": 0x0D, "tab": 0x09, "escape": 0x1B,
	"backspace": 0x08, "delete": 0x2E, "insert": 0x2D,
	"home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22,
	"up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27,
}

// vkToName is the reverse map, built on init.
var vkToName map[uint16]string

func init() {
	vkToName = make(map[uint16]string, len(keyNames))
	for name, vk := range keyNames {
		// Prefer shorter name if duplicate VK (shouldn't happen but defensive).
		if existing, ok := vkToName[vk]; !ok || len(name) < len(existing) {
			vkToName[vk] = name
		}
	}
}

// ParseKeyCombo parses a shortcut string like "ctrl+g" or "f8" into a KeyCombo.
func ParseKeyCombo(s string) (KeyCombo, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return KeyCombo{}, fmt.Errorf("empty key combo")
	}
	parts := strings.Split(s, "+")
	var mods Modifier
	var triggerKey string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return KeyCombo{}, fmt.Errorf("empty part in key combo %q", s)
		}
		if mod, ok := modifierNames[part]; ok {
			mods |= mod
		} else {
			if triggerKey != "" {
				return KeyCombo{}, fmt.Errorf("multiple trigger keys in %q: %q and %q", s, triggerKey, part)
			}
			triggerKey = part
		}
	}

	if triggerKey == "" {
		return KeyCombo{}, fmt.Errorf("no trigger key in %q (modifiers only)", s)
	}

	vk, ok := keyNames[triggerKey]
	if !ok {
		return KeyCombo{}, fmt.Errorf("unknown key name %q", triggerKey)
	}

	return KeyCombo{Modifiers: mods, VK: vk, KeyName: triggerKey}, nil
}

// DisplayString returns a human-readable representation like "Ctrl + Shift + G".
func (kc KeyCombo) DisplayString() string {
	var parts []string
	for _, md := range modifierDisplay {
		if kc.Modifiers&md.Flag != 0 {
			parts = append(parts, md.Name)
		}
	}
	parts = append(parts, strings.ToUpper(kc.KeyName))
	return strings.Join(parts, " + ")
}

// String returns the canonical lowercase format like "ctrl+shift+g".
func (kc KeyCombo) String() string {
	var parts []string
	for _, md := range modifierDisplay {
		if kc.Modifiers&md.Flag != 0 {
			parts = append(parts, strings.ToLower(md.Name))
		}
	}
	parts = append(parts, kc.KeyName)
	return strings.Join(parts, "+")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/features/shortcut/ -run "TestParseKeyCombo|TestKeyCombo" -v`
Expected: all 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/features/shortcut/keycombo.go internal/features/shortcut/keycombo_test.go
git commit -m "feat(shortcut): add key combo parser with VK code mapping (#30)"
```

---

### Task 2: Update service interface, event model, and settings model

**Files:**
- Modify: `internal/features/shortcut/service.go`
- Modify: `internal/features/settings/model.go`

- [ ] **Step 1: Update ShortcutEvent and Service interface**

Replace `internal/features/shortcut/service.go` entirely:

```go
package shortcut

import "time"

// ShortcutEvent carries the payload emitted when a shortcut fires.
type ShortcutEvent struct {
	Source string // "hotkey" | "simulate"
	Action string // "fix" | "pyramidize"
}

// ShortcutConfig holds the configuration for shortcut detection.
type ShortcutConfig struct {
	Mode            string        // "double_tap" | "independent"
	FixCombo        string        // e.g. "ctrl+g"
	PyramidizeCombo string        // e.g. "ctrl+shift+g"
	DoubleTapDelay  time.Duration // e.g. 200ms
}

// Service is the platform-agnostic interface for global shortcut handling.
// Platform-specific implementations are in service_windows.go / service_linux.go.
type Service interface {
	// Register activates the global shortcut listener with the given configuration.
	Register(cfg ShortcutConfig) error
	// Unregister deactivates the listener.
	Unregister()
	// Triggered returns a channel that receives an event each time a shortcut fires.
	Triggered() <-chan ShortcutEvent
	// UpdateConfig hot-reloads the shortcut configuration without restarting the app.
	UpdateConfig(cfg ShortcutConfig) error
}
```

- [ ] **Step 2: Update Settings model**

In `internal/features/settings/model.go`, add the new shortcut fields to the `Settings` struct (after `ShortcutKey`):

```go
	ShortcutKey             string `json:"shortcut_key"`             // LEGACY — migrated to ShortcutFix on load
	ShortcutMode            string `json:"shortcut_mode"`            // "double_tap" | "independent"
	ShortcutFix             string `json:"shortcut_fix"`             // e.g. "ctrl+g"
	ShortcutPyramidize      string `json:"shortcut_pyramidize"`      // e.g. "ctrl+shift+g" (independent mode only)
	ShortcutDoubleTapDelay  int    `json:"shortcut_double_tap_delay"` // ms, 100-500, default 200
```

Update `Default()` to include the new fields:

```go
func Default() Settings {
	return Settings{
		ActiveProvider:             "openai",
		ShortcutKey:                "ctrl+g",
		ShortcutMode:               "double_tap",
		ShortcutFix:                "ctrl+g",
		ShortcutPyramidize:         "ctrl+shift+g",
		ShortcutDoubleTapDelay:     200,
		ThemePreference:            "dark",
		LogLevel:                   "off",
		PyramidizeQualityThreshold: DefaultQualityThreshold,
	}
}
```

- [ ] **Step 3: Add settings migration for legacy shortcut_key**

In `internal/features/settings/service.go`, add migration logic after the existing `debug_logging` migration (around line 82). When loading settings, if `shortcut_fix` is empty but `shortcut_key` has a value, derive the new fields:

```go
		// Migrate legacy shortcut_key → shortcut_fix + defaults.
		if s.current.ShortcutFix == "" && s.current.ShortcutKey != "" {
			s.current.ShortcutFix = s.current.ShortcutKey
			s.current.ShortcutMode = "double_tap"
			s.current.ShortcutPyramidize = "ctrl+shift+g"
			s.current.ShortcutDoubleTapDelay = 200
			logger.Info("settings: migrated shortcut_key to shortcut_fix", "key", s.current.ShortcutFix)
		}
```

- [ ] **Step 4: Verify compilation**

Run: `go build -o bin/KeyLint .`
Expected: compilation errors — `service_windows.go` and `service_linux.go` don't match new interface yet. That's expected — fixed in Tasks 3 and 4.

Run: `go test ./internal/features/shortcut/ -run "TestParseKeyCombo|TestKeyCombo" -v`
Expected: parser tests still pass (they don't depend on the interface).

- [ ] **Step 5: Commit**

```bash
git add internal/features/shortcut/service.go internal/features/settings/model.go internal/features/settings/service.go
git commit -m "feat(shortcut): update service interface and settings model for configurable shortcuts (#30)"
```

---

### Task 3: Update Linux stub for new interface

**Files:**
- Modify: `internal/features/shortcut/service_linux.go`

- [ ] **Step 1: Update the Linux stub**

Replace `internal/features/shortcut/service_linux.go` entirely:

```go
//go:build !windows

package shortcut

import "keylint/internal/logger"

// linuxService is a no-op shortcut service for Linux.
// On Linux, shortcuts are simulated via --simulate-shortcut CLI flag or the
// dev-tools UI button, which manually sends on the channel.
type linuxService struct {
	ch chan ShortcutEvent
}

// NewPlatformService returns the Linux stub implementation.
func NewPlatformService() Service {
	return &linuxService{
		ch: make(chan ShortcutEvent, 1),
	}
}

func (s *linuxService) Register(cfg ShortcutConfig) error {
	logger.Info("shortcut: register (no-op on Linux)", "fix", cfg.FixCombo)
	return nil
}
func (s *linuxService) Unregister() {}
func (s *linuxService) Triggered() <-chan ShortcutEvent { return s.ch }
func (s *linuxService) UpdateConfig(cfg ShortcutConfig) error {
	logger.Info("shortcut: config updated (no-op on Linux)", "fix", cfg.FixCombo)
	return nil
}

// Simulate fires a synthetic shortcut event (used by --simulate-shortcut and dev UI).
func (s *linuxService) Simulate() {
	s.ch <- ShortcutEvent{Source: "simulate", Action: "fix"}
}
```

- [ ] **Step 2: Verify compilation on Linux**

Run: `go build -o bin/KeyLint .`
Expected: still fails — `main.go` calls `Register()` without config arg and references `Detector`. Fixed in Task 5.

Run: `go vet ./internal/features/shortcut/`
Expected: pass (checks the stub compiles in isolation).

- [ ] **Step 3: Commit**

```bash
git add internal/features/shortcut/service_linux.go
git commit -m "feat(shortcut): update Linux stub for new service interface (#30)"
```

---

### Task 4: Windows low-level keyboard hook implementation

**Files:**
- Rewrite: `internal/features/shortcut/service_windows.go`

This is the core of the feature. Replaces `RegisterHotKey` with `WH_KEYBOARD_LL`. The hook callback tracks modifier state, matches key combos, and runs the double-tap state machine. All state is single-threaded (hook thread = message-pump thread).

- [ ] **Step 1: Write the new Windows service**

Replace `internal/features/shortcut/service_windows.go` entirely:

```go
//go:build windows

package shortcut

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"keylint/internal/logger"
)

const (
	whKeyboardLL = 13
	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
	wmTimer      = 0x0113

	vkLControl = 0xA2
	vkRControl = 0xA3
	vkLShift   = 0xA0
	vkRShift   = 0xA1
	vkLMenu    = 0xA4 // Left Alt
	vkRMenu    = 0xA5 // Right Alt
	vkLWin     = 0x5B
	vkRWin     = 0x5C

	// Tag for self-generated input (CopyFromForeground sends Ctrl+C).
	// Checked in the hook to avoid intercepting our own synthetic keypresses.
	extraInfoTag = 0x4B4C // "KL" in hex
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	setWindowsHookEx = user32.NewProc("SetWindowsHookExW")
	unhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	callNextHookEx   = user32.NewProc("CallNextHookEx")
	getMessage       = user32.NewProc("GetMessageW")
	setTimer         = user32.NewProc("SetTimer")
	killTimer        = user32.NewProc("KillTimer")
	postThreadMessage = user32.NewProc("PostThreadMessageW")
	getThreadId      = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
)

// kbdLLHookStruct mirrors the Win32 KBDLLHOOKSTRUCT.
type kbdLLHookStruct struct {
	VKCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      [2]int32
}

// doubleTapTimerID is the SetTimer ID for the double-tap detection window.
const doubleTapTimerID = 1

// wmApp is the base for custom messages posted to the message loop.
const wmApp = 0x8000

const (
	wmAction = wmApp + 1 // WParam: 0=fix, 1=pyramidize
)

type windowsService struct {
	ch       chan ShortcutEvent
	hookH    uintptr // hook handle
	threadID uint32

	// Configuration — guarded by mu for hot-reload from UpdateConfig.
	mu              sync.Mutex
	mode            string
	fixCombo        KeyCombo
	pyramidizeCombo KeyCombo
	doubleTapDelay  uint32 // milliseconds

	// Double-tap state (only accessed on the hook thread — no lock needed).
	waiting bool // true = first tap received, waiting for second or timeout
	mods    Modifier // currently held modifier keys
}

// NewPlatformService returns the Windows WH_KEYBOARD_LL implementation.
func NewPlatformService() Service {
	return &windowsService{ch: make(chan ShortcutEvent, 2)}
}

func (s *windowsService) Register(cfg ShortcutConfig) error {
	if err := s.applyConfig(cfg); err != nil {
		return err
	}

	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()

		tid, _, _ := getThreadId.Call()
		s.threadID = uint32(tid)

		hookProc := syscall.NewCallback(s.hookCallback)
		h, _, err := setWindowsHookEx.Call(whKeyboardLL, hookProc, 0, 0)
		if h == 0 {
			logger.Error("shortcut: SetWindowsHookEx failed", "err", err)
			ready <- fmt.Errorf("SetWindowsHookEx failed: %w", err)
			return
		}
		s.hookH = h
		logger.Info("shortcut: WH_KEYBOARD_LL hook installed")
		ready <- nil

		s.messageLoop()
	}()
	return <-ready
}

func (s *windowsService) Unregister() {
	if s.hookH != 0 {
		unhookWindowsHookEx.Call(s.hookH)
		s.hookH = 0
		logger.Info("shortcut: hook uninstalled")
	}
}

func (s *windowsService) Triggered() <-chan ShortcutEvent { return s.ch }

func (s *windowsService) UpdateConfig(cfg ShortcutConfig) error {
	if err := s.applyConfig(cfg); err != nil {
		return err
	}
	// Reset any in-progress double-tap detection.
	if s.threadID != 0 {
		killTimer.Call(0, doubleTapTimerID)
	}
	logger.Info("shortcut: config updated", "mode", cfg.Mode, "fix", cfg.FixCombo)
	return nil
}

func (s *windowsService) applyConfig(cfg ShortcutConfig) error {
	fixKC, err := ParseKeyCombo(cfg.FixCombo)
	if err != nil {
		return fmt.Errorf("invalid fix combo %q: %w", cfg.FixCombo, err)
	}

	var pyrKC KeyCombo
	if cfg.Mode == "independent" {
		pyrKC, err = ParseKeyCombo(cfg.PyramidizeCombo)
		if err != nil {
			return fmt.Errorf("invalid pyramidize combo %q: %w", cfg.PyramidizeCombo, err)
		}
	}

	delay := uint32(cfg.DoubleTapDelay / time.Millisecond)
	if delay < 100 {
		delay = 100
	}
	if delay > 500 {
		delay = 500
	}

	s.mu.Lock()
	s.mode = cfg.Mode
	s.fixCombo = fixKC
	s.pyramidizeCombo = pyrKC
	s.doubleTapDelay = delay
	s.mu.Unlock()

	return nil
}

// hookCallback is called by Windows for every keyboard event system-wide.
// It must return quickly. Returning 1 suppresses the key; returning CallNextHookEx passes it.
func (s *windowsService) hookCallback(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode < 0 {
		ret, _, _ := callNextHookEx.Call(s.hookH, uintptr(nCode), wParam, lParam)
		return ret
	}

	kb := (*kbdLLHookStruct)(unsafe.Pointer(lParam))

	// Pass through our own synthetic keypresses (from CopyFromForeground / PasteToForeground).
	if kb.DwExtraInfo == extraInfoTag {
		ret, _, _ := callNextHookEx.Call(s.hookH, uintptr(nCode), wParam, lParam)
		return ret
	}

	vk := uint16(kb.VKCode)
	isDown := wParam == wmKeyDown || wParam == wmSysKeyDown

	// Track modifier state.
	if mod := vkToModifier(vk); mod != 0 {
		if isDown {
			s.mods |= mod
		} else {
			s.mods &^= mod
			// If modifier released while waiting for double-tap, fire fix immediately.
			if s.waiting {
				s.waiting = false
				killTimer.Call(0, doubleTapTimerID)
				s.postAction(0) // fix
			}
		}
		ret, _, _ := callNextHookEx.Call(s.hookH, uintptr(nCode), wParam, lParam)
		return ret
	}

	if !isDown {
		ret, _, _ := callNextHookEx.Call(s.hookH, uintptr(nCode), wParam, lParam)
		return ret
	}

	// Read current config.
	s.mu.Lock()
	mode := s.mode
	fixKC := s.fixCombo
	pyrKC := s.pyramidizeCombo
	delay := s.doubleTapDelay
	s.mu.Unlock()

	if mode == "independent" {
		// Independent mode: check both combos, fire immediately.
		if vk == fixKC.VK && s.mods == fixKC.Modifiers {
			s.postAction(0) // fix
			return 1         // suppress
		}
		if vk == pyrKC.VK && s.mods == pyrKC.Modifiers {
			s.postAction(1) // pyramidize
			return 1         // suppress
		}
	} else {
		// Double-tap mode: match fix combo's trigger key + modifiers.
		if vk == fixKC.VK && s.mods == fixKC.Modifiers {
			if s.waiting {
				// Second tap within window → pyramidize.
				s.waiting = false
				killTimer.Call(0, doubleTapTimerID)
				s.postAction(1) // pyramidize
			} else {
				// First tap → start timer.
				s.waiting = true
				setTimer.Call(0, doubleTapTimerID, uintptr(delay), 0)
			}
			return 1 // suppress
		}
	}

	// Not a match — pass through.
	ret, _, _ := callNextHookEx.Call(s.hookH, uintptr(nCode), wParam, lParam)
	return ret
}

func (s *windowsService) postAction(action uintptr) {
	postThreadMessage.Call(uintptr(s.threadID), wmAction, action, 0)
}

func (s *windowsService) messageLoop() {
	logger.Info("shortcut: message loop started")
	var m msg
	for {
		ret, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 {
			break
		}
		switch m.Message {
		case wmTimer:
			// Double-tap timer expired → fix.
			if s.waiting {
				s.waiting = false
				killTimer.Call(0, doubleTapTimerID)
				s.postAction(0) // fix
			}
		case wmAction:
			action := "fix"
			if m.WParam == 1 {
				action = "pyramidize"
			}
			logger.Info("shortcut: action detected", "action", action)
			s.ch <- ShortcutEvent{Source: "hotkey", Action: action}
		}
	}
}

// Simulate fires a synthetic shortcut event (used by --simulate-shortcut and dev UI).
func (s *windowsService) Simulate() {
	s.ch <- ShortcutEvent{Source: "simulate", Action: "fix"}
}

// vkToModifier maps virtual key codes to modifier flags.
func vkToModifier(vk uint16) Modifier {
	switch vk {
	case vkLControl, vkRControl:
		return ModCtrl
	case vkLShift, vkRShift:
		return ModShift
	case vkLMenu, vkRMenu:
		return ModAlt
	case vkLWin, vkRWin:
		return ModWin
	default:
		return 0
	}
}
```

- [ ] **Step 2: Tag CopyFromForeground with extraInfoTag**

In `internal/features/clipboard/paste_windows.go`, update the `pasteInput` structs in `CopyFromForeground()` and `PasteToForeground()` to set `dwExtraInfo` to the tag so the keyboard hook passes them through.

In `CopyFromForeground()` (around line 48), change the inputs array:

```go
	inputs := [4]pasteInput{
		{inputType: inputKeyboard, wVk: vkControl, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkC, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkC, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkControl, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
	}
```

In `PasteToForeground()` (around line 74), same pattern:

```go
	inputs := [4]pasteInput{
		{inputType: inputKeyboard, wVk: vkControl, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkV, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkV, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
		{inputType: inputKeyboard, wVk: vkControl, dwFlags: keyEventKeyUp, dwExtraInfo: 0x4B4C},
	}
```

- [ ] **Step 3: Verify it compiles (on Linux, cross-check vet)**

Run: `go vet ./internal/features/shortcut/`
Run: `go vet ./internal/features/clipboard/`
Expected: both pass. Full build still fails due to main.go — fixed in Task 5.

- [ ] **Step 4: Commit**

```bash
git add internal/features/shortcut/service_windows.go internal/features/clipboard/paste_windows.go
git commit -m "feat(shortcut): replace RegisterHotKey with WH_KEYBOARD_LL low-level hook (#30)"
```

---

### Task 5: Simplify main.go — remove Detector, update event names

**Files:**
- Modify: `main.go`

Remove the Detector, the atomic.Bool, and the two goroutines. Replace with a single goroutine that reads classified events and emits Wails events. Pass config to `Register()`.

- [ ] **Step 1: Update event registrations**

Replace lines 35-36:

```go
	application.RegisterEvent[string]("shortcut:fix")
	application.RegisterEvent[string]("shortcut:pyramidize")
```

- [ ] **Step 2: Update Register() call with config**

Replace lines 123-130 (the Register block):

```go
	// Register the global shortcut (no-op on Linux).
	shortcutCfg := shortcut.ShortcutConfig{
		Mode:            cfg.ShortcutMode,
		FixCombo:        cfg.ShortcutFix,
		PyramidizeCombo: cfg.ShortcutPyramidize,
		DoubleTapDelay:  time.Duration(cfg.ShortcutDoubleTapDelay) * time.Millisecond,
	}
	if err := services.Shortcut.Register(shortcutCfg); err != nil {
		log.Printf("warn: shortcut registration failed: %v", err)
		logger.Warn("shortcut: registration failed", "err", err)
	} else {
		logger.Info("shortcut: registered", "mode", cfg.ShortcutMode, "fix", cfg.ShortcutFix)
	}
	wailsApp.OnShutdown(func() { services.Shortcut.Unregister() })
```

- [ ] **Step 3: Replace the entire shortcut goroutine section**

Remove lines 133-171 (the Detector, atomic.Bool, both goroutines) and replace with:

```go
	// Forward classified shortcut events to the frontend.
	go func() {
		for event := range services.Shortcut.Triggered() {
			logger.Info("shortcut: action", "action", event.Action, "source", event.Source)
			pyramidizeSvc.CaptureSourceApp()
			if err := services.Clipboard.CopyFromForeground(); err != nil {
				logger.Warn("shortcut: CopyFromForeground failed", "err", err)
			}
			switch event.Action {
			case "fix":
				wailsApp.Event.Emit("shortcut:fix", event.Source)
			case "pyramidize":
				window.Show().Focus()
				wailsApp.Event.Emit("shortcut:pyramidize", event.Source)
			}
		}
	}()
```

- [ ] **Step 4: Clean up imports**

Remove `"sync/atomic"` from imports (no longer used). Remove `"keylint/internal/features/shortcut"` only if the `shortcut.` prefix is no longer referenced — but it IS still used for `shortcut.ShortcutConfig`, so keep it. The `time` import is still used.

- [ ] **Step 5: Verify build and Go tests**

Run: `go build -o bin/KeyLint .`
Expected: compiles cleanly.

Run: `go test ./internal/...`
Expected: all pass (detect_test.go still runs but will be removed in Task 6).

- [ ] **Step 6: Commit**

```bash
git add main.go
git commit -m "feat(shortcut): simplify main.go — remove Detector, use classified events (#30)"
```

---

### Task 6: Remove Detector (no longer needed)

**Files:**
- Delete: `internal/features/shortcut/detect.go`
- Delete: `internal/features/shortcut/detect_test.go`

- [ ] **Step 1: Delete the files**

```bash
rm internal/features/shortcut/detect.go internal/features/shortcut/detect_test.go
```

- [ ] **Step 2: Verify build and tests**

Run: `go build -o bin/KeyLint .`
Expected: compiles cleanly (nothing imports Detector anymore).

Run: `go test ./internal/features/shortcut/ -v`
Expected: only keycombo tests run and pass.

- [ ] **Step 3: Commit**

```bash
git add -A internal/features/shortcut/detect.go internal/features/shortcut/detect_test.go
git commit -m "chore(shortcut): remove Detector — detection now in WH_KEYBOARD_LL hook (#30)"
```

---

### Task 7: Frontend — rename observables to shortcutFix$/shortcutPyramidize$

**Files:**
- Modify: `frontend/src/app/core/wails.service.ts`
- Modify: `frontend/src/testing/wails-mock.ts`
- Modify: `frontend/src/app/core/message-bus.service.ts`
- Modify: `frontend/src/app/features/fix/fix.component.ts:107`
- Modify: `frontend/src/app/features/fix/fix.component.spec.ts:113-126`
- Modify: `frontend/src/app/features/text-enhancement/text-enhancement.component.ts:1096`
- Modify: `frontend/src/app/features/text-enhancement/text-enhancement.component.spec.ts:253-331`
- Modify: `frontend/src/app/layout/shell.component.ts:118`
- Modify: `frontend/src/app/layout/shell.component.spec.ts:114-121`
- Modify: `frontend/src/app/core/wails.service.spec.ts`

This is a mechanical global rename: `shortcutSingle` → `shortcutFix`, `shortcutDouble` → `shortcutPyramidize`, `shortcut:single` → `shortcut:fix`, `shortcut:double` → `shortcut:pyramidize`.

- [ ] **Step 1: Update wails.service.ts**

Replace all occurrences:
- `shortcutSingle` → `shortcutFix` (subjects and observables)
- `shortcutDouble` → `shortcutPyramidize`
- `'shortcut:single'` → `'shortcut:fix'`
- `'shortcut:double'` → `'shortcut:pyramidize'`
- Update JSDoc comments accordingly

- [ ] **Step 2: Update wails-mock.ts**

Replace all occurrences:
- `shortcutSingle$` → `shortcutFix$`
- `shortcutDouble$` → `shortcutPyramidize$`
- `_shortcutSingle$` → `_shortcutFix$`
- `_shortcutDouble$` → `_shortcutPyramidize$`

- [ ] **Step 3: Update message-bus.service.ts**

Replace the event type union:

```typescript
export type BusEvent =
  | { type: 'shortcut:fix'; source: string }
  | { type: 'shortcut:pyramidize'; source: string }
  | { type: 'enhancement:complete'; text: string }
  | { type: 'enhancement:error'; message: string };
```

- [ ] **Step 4: Update wails.service.spec.ts**

Replace `'shortcut:single'` → `'shortcut:fix'` in the MessageBusService test.

- [ ] **Step 5: Update fix.component.ts and spec**

In `fix.component.ts:107`:
```typescript
    this.sub = this.wails.shortcutFix$.subscribe(() => {
```

In `fix.component.spec.ts`:
- Test name: `'shortcutFix$ triggers fixClipboard'`
- Subject: `wailsMock._shortcutFix$.next('hotkey')`
- Unsubscribe test: `wailsMock._shortcutFix$.next('hotkey')`

- [ ] **Step 6: Update text-enhancement.component.ts and spec**

In `text-enhancement.component.ts:1096`:
```typescript
    this.sub = this.wails.shortcutPyramidize$.subscribe(async () => {
```

In spec: all `shortcutDouble$` → `shortcutPyramidize$`, all `_shortcutDouble$` → `_shortcutPyramidize$`.

- [ ] **Step 7: Update shell.component.ts and spec**

In `shell.component.ts:118`:
```typescript
      this.wails.shortcutPyramidize$.subscribe(() => {
```

In spec: `shortcutDouble$` → `shortcutPyramidize$`, `_shortcutDouble$` → `_shortcutPyramidize$`.

- [ ] **Step 8: Run all frontend tests**

Run: `cd frontend && npm test`
Expected: 132 tests, 0 failures.

- [ ] **Step 9: Verify no stale references**

Run: `grep -r "shortcutSingle\|shortcutDouble\|shortcut:single\|shortcut:double" frontend/src/ --include="*.ts"`
Expected: no results.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/
git commit -m "refactor(frontend): rename shortcut observables to shortcutFix$/shortcutPyramidize$ (#30)"
```

---

### Task 8: Frontend — update Settings model for new shortcut fields

**Files:**
- Modify: `frontend/src/app/core/wails.service.ts` (BROWSER_MODE_DEFAULTS)
- Modify: `frontend/src/testing/wails-mock.ts` (defaultSettings)

- [ ] **Step 1: Update BROWSER_MODE_DEFAULTS in wails.service.ts**

Add the new fields (around line 30):

```typescript
const BROWSER_MODE_DEFAULTS: Settings = {
  active_provider: 'claude',
  providers: { ollama_url: '', aws_region: '' },
  shortcut_key: 'ctrl+g',
  shortcut_mode: 'double_tap',
  shortcut_fix: 'ctrl+g',
  shortcut_pyramidize: 'ctrl+shift+g',
  shortcut_double_tap_delay: 200,
  start_on_boot: false,
  theme_preference: 'dark',
  completed_setup: false,
  log_level: 'off',
  sensitive_logging: false,
  update_channel: '',
  app_presets: [],
  pyramidize_quality_threshold: 0.65,
};
```

- [ ] **Step 2: Update defaultSettings in wails-mock.ts**

Add the same fields to the mock defaults:

```typescript
export const defaultSettings: Settings = {
  active_provider: 'openai',
  providers: { ollama_url: '', aws_region: '' },
  shortcut_key: 'ctrl+g',
  shortcut_mode: 'double_tap',
  shortcut_fix: 'ctrl+g',
  shortcut_pyramidize: 'ctrl+shift+g',
  shortcut_double_tap_delay: 200,
  start_on_boot: false,
  theme_preference: 'dark',
  completed_setup: false,
  log_level: 'off',
  sensitive_logging: false,
  update_channel: '',
  app_presets: [],
  pyramidize_quality_threshold: 0.65,
};
```

- [ ] **Step 3: Run tests**

Run: `cd frontend && npm test`
Expected: all pass (new fields don't break anything — they're just data).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/core/wails.service.ts frontend/src/testing/wails-mock.ts
git commit -m "feat(settings): add shortcut configuration fields to frontend defaults (#30)"
```

---

### Task 9: Frontend — shortcut recorder component

**Files:**
- Create: `frontend/src/app/features/settings/shortcut-recorder/shortcut-recorder.component.ts`
- Create: `frontend/src/app/features/settings/shortcut-recorder/shortcut-recorder.component.spec.ts`

A reusable component that captures keyboard shortcuts. Shows the current combo formatted (e.g., "Ctrl + G"). Clicking "Record..." enters capture mode, next key combo is captured and emitted, Escape cancels.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/app/features/settings/shortcut-recorder/shortcut-recorder.component.spec.ts`:

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { ShortcutRecorderComponent } from './shortcut-recorder.component';

describe('ShortcutRecorderComponent', () => {
  let fixture: ComponentFixture<ShortcutRecorderComponent>;
  let component: ShortcutRecorderComponent;
  let el: HTMLElement;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ShortcutRecorderComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ShortcutRecorderComponent);
    component = fixture.componentInstance;
    component.value = 'ctrl+g';
    fixture.detectChanges();
    await fixture.whenStable();
    el = fixture.nativeElement;
  });

  it('renders the formatted key combo', () => {
    const display = el.querySelector('[data-testid="combo-display"]');
    expect(display?.textContent?.trim()).toBe('Ctrl + G');
  });

  it('shows Record button', () => {
    expect(el.querySelector('[data-testid="record-btn"]')).toBeTruthy();
  });

  it('enters recording mode on Record click', () => {
    el.querySelector<HTMLButtonElement>('[data-testid="record-btn"]')?.click();
    fixture.detectChanges();
    expect(component.recording).toBe(true);
    expect(el.querySelector('[data-testid="recording-indicator"]')).toBeTruthy();
  });

  it('exits recording mode on Escape', () => {
    component.recording = true;
    fixture.detectChanges();
    const event = new KeyboardEvent('keydown', { key: 'Escape' });
    el.querySelector<HTMLElement>('[data-testid="recorder-field"]')?.dispatchEvent(event);
    fixture.detectChanges();
    expect(component.recording).toBe(false);
  });

  it('captures a key combo and emits valueChange', () => {
    const spy = vi.fn();
    component.valueChange.subscribe(spy);
    component.recording = true;
    fixture.detectChanges();

    const event = new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false });
    el.querySelector<HTMLElement>('[data-testid="recorder-field"]')?.dispatchEvent(event);
    fixture.detectChanges();

    expect(spy).toHaveBeenCalledWith('ctrl+k');
    expect(component.recording).toBe(false);
  });

  it('ignores modifier-only keypresses during recording', () => {
    const spy = vi.fn();
    component.valueChange.subscribe(spy);
    component.recording = true;
    fixture.detectChanges();

    const event = new KeyboardEvent('keydown', { key: 'Control', ctrlKey: true });
    el.querySelector<HTMLElement>('[data-testid="recorder-field"]')?.dispatchEvent(event);
    fixture.detectChanges();

    expect(spy).not.toHaveBeenCalled();
    expect(component.recording).toBe(true);
  });
});
```

- [ ] **Step 2: Write the component**

Create `frontend/src/app/features/settings/shortcut-recorder/shortcut-recorder.component.ts`:

```typescript
import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonModule } from 'primeng/button';

const MODIFIER_KEYS = new Set(['Control', 'Shift', 'Alt', 'Meta']);

const KEY_TO_NAME: Record<string, string> = {
  ' ': 'space', 'Enter': 'enter', 'Tab': 'tab', 'Escape': 'escape',
  'Backspace': 'backspace', 'Delete': 'delete', 'Insert': 'insert',
  'Home': 'home', 'End': 'end', 'PageUp': 'pageup', 'PageDown': 'pagedown',
  'ArrowUp': 'up', 'ArrowDown': 'down', 'ArrowLeft': 'left', 'ArrowRight': 'right',
};

function formatCombo(combo: string): string {
  if (!combo) return '';
  const parts = combo.split('+');
  return parts.map(p => {
    if (p === 'ctrl') return 'Ctrl';
    if (p === 'shift') return 'Shift';
    if (p === 'alt') return 'Alt';
    if (p === 'win') return 'Win';
    return p.length === 1 ? p.toUpperCase() : p.charAt(0).toUpperCase() + p.slice(1).toUpperCase();
  }).join(' + ');
}

@Component({
  selector: 'app-shortcut-recorder',
  standalone: true,
  imports: [CommonModule, ButtonModule],
  template: `
    <div class="recorder-wrapper" [class.recording]="recording" data-testid="recorder-field"
         tabindex="0" (keydown)="onKeyDown($event)">
      @if (recording) {
        <span class="recording-text" data-testid="recording-indicator">Press a key combo...</span>
      } @else {
        <span class="combo-text" data-testid="combo-display">{{ displayValue }}</span>
      }
      <p-button
        data-testid="record-btn"
        [label]="recording ? 'Cancel' : 'Record...'"
        [severity]="recording ? 'danger' : 'secondary'"
        size="small"
        (onClick)="toggleRecording()"
      />
    </div>
  `,
  styles: [`
    .recorder-wrapper {
      display: flex;
      align-items: center;
      justify-content: space-between;
      border: 1px solid var(--p-content-border-color);
      border-radius: 6px;
      padding: 0.5rem 0.75rem;
      background: var(--p-content-hover-background);
      min-height: 2.5rem;
      outline: none;
    }
    .recorder-wrapper.recording {
      border-color: var(--p-primary-color);
      box-shadow: 0 0 0 1px var(--p-primary-color);
    }
    .combo-text {
      font-family: monospace;
      font-size: 0.9rem;
      color: var(--p-text-color);
    }
    .recording-text {
      font-size: 0.85rem;
      color: var(--p-text-muted-color);
      animation: pulse 1.5s ease-in-out infinite;
    }
    @keyframes pulse {
      0%, 100% { opacity: 1; }
      50% { opacity: 0.5; }
    }
  `],
})
export class ShortcutRecorderComponent {
  @Input() value = '';
  @Output() valueChange = new EventEmitter<string>();

  recording = false;

  get displayValue(): string {
    return formatCombo(this.value);
  }

  toggleRecording(): void {
    this.recording = !this.recording;
  }

  onKeyDown(event: KeyboardEvent): void {
    if (!this.recording) return;

    event.preventDefault();
    event.stopPropagation();

    if (event.key === 'Escape') {
      this.recording = false;
      return;
    }

    // Ignore modifier-only presses — wait for a trigger key.
    if (MODIFIER_KEYS.has(event.key)) return;

    const parts: string[] = [];
    if (event.ctrlKey) parts.push('ctrl');
    if (event.shiftKey) parts.push('shift');
    if (event.altKey) parts.push('alt');
    if (event.metaKey) parts.push('win');

    // Map the key to our canonical name.
    let keyName = KEY_TO_NAME[event.key] ?? event.key.toLowerCase();
    // Function keys come as "F1", "F12" etc.
    if (/^f\d{1,2}$/i.test(event.key)) {
      keyName = event.key.toLowerCase();
    }

    parts.push(keyName);
    const combo = parts.join('+');

    this.value = combo;
    this.valueChange.emit(combo);
    this.recording = false;
  }
}
```

- [ ] **Step 3: Run tests**

Run: `cd frontend && npx vitest run src/app/features/settings/shortcut-recorder/shortcut-recorder.component.spec.ts`
Expected: all 6 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/features/settings/shortcut-recorder/
git commit -m "feat(settings): add shortcut recorder component (#30)"
```

---

### Task 10: Frontend — settings UI shortcuts section

**Files:**
- Modify: `frontend/src/app/features/settings/settings.component.ts`

Replace the existing shortcut_key text input (line 59-62) with the expanded shortcuts section.

- [ ] **Step 1: Add ShortcutRecorderComponent to imports**

In the `@Component` decorator's `imports` array, add `ShortcutRecorderComponent`:

```typescript
import { ShortcutRecorderComponent } from './shortcut-recorder/shortcut-recorder.component';
```

And add `ShortcutRecorderComponent` and `SliderModule` to the `imports` array. Also add:

```typescript
import { SliderModule } from 'primeng/slider';
```

- [ ] **Step 2: Replace the shortcut form group in the template**

Replace the shortcut_key form group (lines 59-62):

```html
                <div class="form-group">
                  <label>Shortcut Key</label>
                  <input data-testid="shortcut-input" pInputText [(ngModel)]="settings.shortcut_key" placeholder="ctrl+g" />
                </div>
```

With:

```html
                <!-- Shortcuts section -->
                <div class="form-group" data-testid="shortcut-mode-section">
                  <div class="toggle-row">
                    <div class="toggle-label-group">
                      <label>Double-tap mode</label>
                      <small class="hint-text">
                        @if (settings.shortcut_mode === 'double_tap') {
                          Hold your modifier keys, tap the trigger key once for Fix, twice for Pyramidize.
                        } @else {
                          Assign separate shortcuts for each action.
                        }
                      </small>
                    </div>
                    <p-toggle-switch
                      [ngModel]="settings.shortcut_mode === 'double_tap'"
                      (ngModelChange)="settings.shortcut_mode = $event ? 'double_tap' : 'independent'"
                    />
                  </div>
                </div>

                @if (settings.shortcut_mode === 'double_tap') {
                  <div class="form-group" data-testid="shortcut-fix-section">
                    <label>Shortcut</label>
                    <app-shortcut-recorder
                      [value]="settings.shortcut_fix"
                      (valueChange)="settings.shortcut_fix = $event"
                    />
                    <small class="hint-text">Single tap → Fix · Double tap → Pyramidize</small>
                  </div>
                  <div class="form-group" data-testid="shortcut-delay-section">
                    <label>Double-tap delay: {{ settings.shortcut_double_tap_delay }}ms</label>
                    <p-slider
                      [(ngModel)]="settings.shortcut_double_tap_delay"
                      [min]="100"
                      [max]="500"
                      [step]="25"
                    />
                    <small class="hint-text">How long to wait for a second tap. Lower = faster but harder to trigger.</small>
                  </div>
                } @else {
                  <div class="form-group" data-testid="shortcut-fix-section">
                    <label>Fix shortcut</label>
                    <app-shortcut-recorder
                      [value]="settings.shortcut_fix"
                      (valueChange)="settings.shortcut_fix = $event"
                    />
                    <small class="hint-text">Silently fixes clipboard text.</small>
                  </div>
                  <div class="form-group" data-testid="shortcut-pyramidize-section">
                    <label>Pyramidize shortcut</label>
                    <app-shortcut-recorder
                      [value]="settings.shortcut_pyramidize"
                      (valueChange)="settings.shortcut_pyramidize = $event"
                    />
                    <small class="hint-text">Opens the Pyramidize editor with clipboard text.</small>
                  </div>
                }
```

- [ ] **Step 3: Run tests**

Run: `cd frontend && npm test`
Expected: all tests pass. The settings spec should continue working — the shortcut_key field is removed from the template but the old tests that reference `[data-testid="shortcut-input"]` need updating.

- [ ] **Step 4: Update settings test for new shortcut section**

In `settings.component.spec.ts`, replace any test referencing `data-testid="shortcut-input"` with:

```typescript
  it('renders shortcut mode toggle in general tab', async () => {
    const fixture = await createAndWait();
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('[data-testid="shortcut-mode-section"]')).toBeTruthy();
    expect(el.querySelector('[data-testid="shortcut-fix-section"]')).toBeTruthy();
  });
```

- [ ] **Step 5: Run tests again**

Run: `cd frontend && npm test`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/app/features/settings/
git commit -m "feat(settings): add configurable shortcuts UI with mode toggle and recorder (#30)"
```

---

### Task 11: Hot-reload — notify backend when shortcuts change

**Files:**
- Modify: `main.go`

When settings are saved and shortcut config changed, call `UpdateConfig` on the shortcut service so the hook reloads without app restart.

- [ ] **Step 1: Add settings change listener**

After the shortcut registration block in `main.go`, add a listener for settings changes. Subscribe to the `settings:changed` Wails event and call `UpdateConfig`:

```go
	// Hot-reload shortcuts when settings change.
	wailsApp.Event.On("settings:changed", func(ev *application.CustomEvent) {
		newCfg := services.Settings.Get()
		newShortcutCfg := shortcut.ShortcutConfig{
			Mode:            newCfg.ShortcutMode,
			FixCombo:        newCfg.ShortcutFix,
			PyramidizeCombo: newCfg.ShortcutPyramidize,
			DoubleTapDelay:  time.Duration(newCfg.ShortcutDoubleTapDelay) * time.Millisecond,
		}
		if err := services.Shortcut.UpdateConfig(newShortcutCfg); err != nil {
			logger.Warn("shortcut: hot-reload failed", "err", err)
		}
	})
```

- [ ] **Step 2: Verify build**

Run: `go build -o bin/KeyLint .`
Expected: compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat(shortcut): hot-reload shortcuts on settings change (#30)"
```

---

### Task 12: Full test suite verification

**Files:** None (verification only)

- [ ] **Step 1: Run all frontend tests**

Run: `cd frontend && npm test`
Expected: 0 failures.

- [ ] **Step 2: Run all Go tests**

Run: `go test ./internal/...`
Expected: all pass, keycombo tests pass, no detect tests (removed).

- [ ] **Step 3: Verify full build**

Run: `go build -o bin/KeyLint .`
Expected: compiles cleanly.

- [ ] **Step 4: Search for stale references**

Run: `grep -r "shortcutSingle\|shortcutDouble\|shortcut:single\|shortcut:double\|shortcutTriggered\|shortcut:triggered" frontend/src/ --include="*.ts"`
Expected: no results.

Run: `grep -r "RegisterHotKey\|UnregisterHotKey" internal/ --include="*.go"`
Expected: no results (fully replaced with WH_KEYBOARD_LL).

Run: `grep -r "detect\.go\|Detector\|PressResult" internal/ --include="*.go"`
Expected: no results (fully removed).

- [ ] **Step 5: Verify detect.go is gone**

Run: `ls internal/features/shortcut/detect*.go 2>/dev/null`
Expected: no output (files deleted).

- [ ] **Step 6: Commit any fixups if needed**
