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

// doubleTapTimerID is the SetTimer ID for the double-tap detection window.
const doubleTapTimerID = 1

// wmApp is the base for custom messages posted to the message loop.
const wmApp = 0x8000

const (
	wmAction = wmApp + 1 // WParam: 0=fix, 1=pyramidize
)

// Double-tap state phases.
type tapPhase int

const (
	tapIdle         tapPhase = iota // no tap in progress
	tapWaitRelease                  // first tap received, waiting for trigger keyup (ignore auto-repeat)
	tapWaitSecond                   // trigger released, waiting for second tap or timer expiry
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

	// Double-tap state (only accessed on the hook thread — no lock needed).
	tapState tapPhase // current detection phase
	mods     Modifier // currently held modifier keys
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
	s.tapState = tapIdle
	logger.Info("shortcut: config updated", "mode", cfg.Mode, "fix", cfg.FixCombo)
	return nil
}

func (s *windowsService) SetPaused(paused bool) {
	s.paused.Store(paused)
	if paused {
		// Reset double-tap state when pausing.
		if s.threadID != 0 {
			killTimer.Call(0, doubleTapTimerID)
		}
		s.tapState = tapIdle
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
			// If modifier released during double-tap detection, fire fix immediately.
			if s.tapState != tapIdle {
				logger.Debug("shortcut: modifier released during double-tap, firing fix")
				s.tapState = tapIdle
				killTimer.Call(0, doubleTapTimerID)
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
				logger.Debug("shortcut: independent fix match", "vk", vk, "mods", s.mods)
				s.postAction(0) // fix
				return 1         // suppress
			}
			if vk == pyrKC.VK && s.mods == pyrKC.Modifiers {
				logger.Debug("shortcut: independent pyramidize match", "vk", vk, "mods", s.mods)
				s.postAction(1) // pyramidize
				return 1         // suppress
			}
		}
	} else {
		// Double-tap mode: match fix combo's trigger key + modifiers.
		isTrigger := vk == fixKC.VK && s.mods == fixKC.Modifiers

		switch s.tapState {
		case tapIdle:
			if isDown && isTrigger {
				// First tap → suppress, start timer, wait for keyup before accepting second tap.
				logger.Debug("shortcut: first tap, starting timer", "delay", delay)
				s.tapState = tapWaitRelease
				setTimer.Call(0, doubleTapTimerID, uintptr(delay), 0)
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

		case tapWaitSecond:
			if isDown && isTrigger {
				// Second tap → pyramidize!
				logger.Debug("shortcut: second tap detected, firing pyramidize")
				s.tapState = tapIdle
				killTimer.Call(0, doubleTapTimerID)
				s.postAction(1) // pyramidize
				return 1         // suppress
			}
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
			// Double-tap timer expired → fire fix.
			if s.tapState != tapIdle {
				logger.Debug("shortcut: timer expired, firing fix")
				s.tapState = tapIdle
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
