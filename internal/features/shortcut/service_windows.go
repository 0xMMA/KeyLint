//go:build windows

package shortcut

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
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
	user32              = syscall.NewLazyDLL("user32.dll")
	setWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	unhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	callNextHookEx      = user32.NewProc("CallNextHookEx")
	getMessage          = user32.NewProc("GetMessageW")
	setTimer            = user32.NewProc("SetTimer")
	killTimer           = user32.NewProc("KillTimer")
	postThreadMessage   = user32.NewProc("PostThreadMessageW")
	getThreadId         = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
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

// wmApp is the base for custom messages posted to the message loop.
const wmApp = 0x8000

const (
	wmAction = wmApp + 1 // WParam: 0=fix, 1=pyramidize
	wmReset  = wmApp + 2 // clear double-tap state on the pump thread
)

// Double-tap state phases. The trigger key is known to be physically down in
// tapWaitRelease and tapFiredWaitRelease — both are left only on its keyup —
// so the hook's own events are all that is needed to track it.
type tapPhase int

const (
	tapIdle             tapPhase = iota // no tap in progress
	tapWaitRelease                      // first tap received, timer running, waiting for trigger keyup (ignore auto-repeat)
	tapWaitSecond                       // trigger released, waiting for second tap or timer expiry
	tapFiredWaitRelease                 // fix already fired while the trigger is still held: absorb auto-repeat until keyup
)

type windowsService struct {
	ch       chan ShortcutEvent
	hookH    uintptr // hook handle
	threadID uint32
	paused   atomic.Bool // true = shortcut detection disabled (e.g. during recording)

	// Configuration — guarded by mu for hot-reload from UpdateConfig.
	mu              sync.Mutex
	mode            string
	fixCombo        KeyCombo
	pyramidizeCombo KeyCombo
	doubleTapDelay  uint32 // milliseconds

	// State — written only on the pump thread (hook callback and message loop
	// both run there), so no lock is needed. Other threads ask for a change by
	// posting wmReset instead of touching these fields.
	tapState       tapPhase // double-tap detection phase
	timerID        uintptr  // ID returned by SetTimer for the running double-tap timer (0 = none)
	mods           Modifier // currently held modifier keys
	indepPending   int      // independent mode: -1=none, 0=fix, 1=pyramidize (fire on keyup)
	indepPendingVK uint16   // trigger VK we're waiting for keyup on
}

// NewPlatformService returns the Windows WH_KEYBOARD_LL implementation.
func NewPlatformService() Service {
	return &windowsService{ch: make(chan ShortcutEvent, 2), indepPending: -1}
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
	// The timer belongs to the pump thread and can only be killed there. If the
	// loop is already gone, the timer died with the thread anyway.
	s.postReset()
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
	// Reset any in-progress detection state (on the pump thread).
	s.postReset()
	logger.Info("shortcut: config updated", "mode", cfg.Mode, "fix", cfg.FixCombo)
	return nil
}

func (s *windowsService) SetPaused(paused bool) {
	s.paused.Store(paused)
	if paused {
		// Reset detection state when pausing (on the pump thread).
		s.postReset()
	}
	logger.Info("shortcut: paused", "paused", paused)
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

	// Pass through when paused (e.g. during shortcut recording in settings).
	if s.paused.Load() {
		ret, _, _ := callNextHookEx.Call(s.hookH, uintptr(nCode), wParam, lParam)
		return ret
	}

	// Pass through our own synthetic keypresses (from CopyFromForeground / PasteToForeground).
	if kb.DwExtraInfo == extraInfoTag {
		ret, _, _ := callNextHookEx.Call(s.hookH, uintptr(nCode), wParam, lParam)
		return ret
	}

	vk := uint16(kb.VKCode)
	isDown := wParam == wmKeyDown || wParam == wmSysKeyDown
	isUp := wParam == wmKeyUp || wParam == wmSysKeyUp

	// Track modifier state.
	if mod := vkToModifier(vk); mod != 0 {
		if isDown {
			s.mods |= mod
		} else if isUp {
			s.mods &^= mod
			// If modifier released while a fix is still pending, fire it immediately.
			// In tapFiredWaitRelease the fix already went out — keep absorbing the
			// held trigger key there instead of firing a second one.
			if s.tapState == tapWaitRelease || s.tapState == tapWaitSecond {
				logger.Debug("shortcut: modifier released during double-tap, firing fix")
				s.stopTimer()
				s.tapState = tapIdle
				s.postAction(0) // fix
			}
		}
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
		if isDown {
			if vk == fixKC.VK && s.mods == fixKC.Modifiers {
				logger.Debug("shortcut: independent fix match (keydown)", "vk", vk, "mods", s.mods)
				s.indepPending = 0
				s.indepPendingVK = vk
				return 1 // suppress — fire on keyup
			}
			if vk == pyrKC.VK && s.mods == pyrKC.Modifiers {
				logger.Debug("shortcut: independent pyramidize match (keydown)", "vk", vk, "mods", s.mods)
				s.indepPending = 1
				s.indepPendingVK = vk
				return 1 // suppress — fire on keyup
			}
		}
		if isUp && s.indepPending >= 0 && vk == s.indepPendingVK {
			action := s.indepPending
			s.indepPending = -1
			logger.Debug("shortcut: independent action (keyup)", "action", action)
			s.postAction(uintptr(action))
			return 1 // suppress
		}
	} else {
		// Double-tap mode: match fix combo's trigger key + modifiers.
		isTrigger := vk == fixKC.VK && s.mods == fixKC.Modifiers

		switch s.tapState {
		case tapIdle:
			if isDown && isTrigger {
				// First tap → suppress, start timer, wait for keyup before accepting second tap.
				if !s.startTimer(delay) {
					// Without a timer nothing would ever end the detection window,
					// so fire the plain fix now and absorb the key until its keyup.
					s.postAction(0) // fix
					s.tapState = tapFiredWaitRelease
				} else {
					logger.Debug("shortcut: first tap, timer started", "delay", delay)
					s.tapState = tapWaitRelease
				}
				return 1 // suppress
			}

		case tapWaitRelease:
			if isUp && vk == fixKC.VK {
				// Trigger key released after first tap → now accept second tap.
				logger.Debug("shortcut: trigger released, waiting for second tap")
				s.tapState = tapWaitSecond
			}
			// Suppress all trigger key events (including auto-repeat keydowns) during this phase.
			if vk == fixKC.VK {
				return 1 // suppress
			}

		case tapFiredWaitRelease:
			if isUp && vk == fixKC.VK {
				// The fix for this hold is done — the next press starts over.
				logger.Debug("shortcut: trigger released after fix, ready for a new tap")
				s.tapState = tapIdle
			}
			// Swallow the auto-repeat storm from the still-held trigger key.
			if vk == fixKC.VK {
				return 1 // suppress
			}

		case tapWaitSecond:
			if isDown && isTrigger {
				// Second tap → pyramidize!
				logger.Debug("shortcut: second tap detected, firing pyramidize")
				s.tapState = tapIdle
				s.stopTimer()
				s.postAction(1) // pyramidize
				return 1         // suppress
			}
		}
	}

	// Not a match — pass through.
	ret, _, _ := callNextHookEx.Call(s.hookH, uintptr(nCode), wParam, lParam)
	return ret
}

// startTimer arms the double-tap window and records the ID Windows hands back.
// With a NULL hWnd, SetTimer ignores the nIDEvent argument and generates its own
// ID — that returned ID is the only handle KillTimer and WM_TIMER wParam speak.
func (s *windowsService) startTimer(delay uint32) bool {
	s.stopTimer() // never leave a previous timer running — SetTimer timers repeat
	id, _, err := setTimer.Call(0, 0, uintptr(delay), 0)
	if id == 0 {
		logger.Warn("shortcut: SetTimer failed, double-tap detection disabled for this tap", "err", err)
		return false
	}
	s.timerID = id
	return true
}

// stopTimer kills the running double-tap timer, if any.
func (s *windowsService) stopTimer() {
	if s.timerID != 0 {
		killTimer.Call(0, s.timerID)
		s.timerID = 0
	}
}

func (s *windowsService) postAction(action uintptr) {
	postThreadMessage.Call(uintptr(s.threadID), wmAction, action, 0)
}

// postReset asks the pump thread to drop any in-progress detection. A
// SetTimer(NULL, …) timer belongs to the thread that created it, so KillTimer
// only works there — callers on other threads must go through the message loop.
func (s *windowsService) postReset() {
	if s.threadID == 0 {
		return
	}
	postThreadMessage.Call(uintptr(s.threadID), wmReset, 0, 0)
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
			// wParam carries the ID SetTimer returned; anything else is a stray
			// timer we no longer own and must not act on.
			if m.WParam != s.timerID {
				break
			}
			s.stopTimer()
			// Double-tap timer expired → fire fix.
			switch s.tapState {
			case tapWaitRelease:
				// Trigger key is still down: fire the fix, then absorb its
				// auto-repeat instead of reading it as a new first tap.
				logger.Debug("shortcut: timer expired while trigger held, firing fix")
				s.tapState = tapFiredWaitRelease
				s.postAction(0) // fix
			case tapWaitSecond:
				logger.Debug("shortcut: timer expired, firing fix")
				s.tapState = tapIdle
				s.postAction(0) // fix
			}
		case wmReset:
			s.stopTimer()
			s.tapState = tapIdle
			s.indepPending = -1
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
