package shortcut

import (
	"time"

	"keylint/internal/logger"
)

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
	// SetPaused temporarily disables shortcut detection (e.g. while recording a new shortcut).
	SetPaused(paused bool)
}

// eventBuffer is how many detected shortcuts can wait for the consumer. It
// only has to cover the consumer being busy for a moment — handling a
// Pyramidize press copies from the foreground window, which can take up to a
// second — so a handful of presses in that time all arrive.
const eventBuffer = 16

// deliver hands ev to the consumer without ever blocking the sender. On
// Windows the sender is the keyboard hook's own thread: blocked there, every
// keystroke on the machine waits with it, and Windows silently removes a
// low-level hook that keeps timing out. So when the buffer is full the event
// is dropped and logged instead — sixteen unhandled presses are a stuck
// consumer, not a user who needs a seventeenth.
func deliver(ch chan ShortcutEvent, ev ShortcutEvent) bool {
	select {
	case ch <- ev:
		return true
	default:
		logger.Warn("shortcut: event dropped, the consumer is not keeping up", "action", ev.Action, "source", ev.Source)
		return false
	}
}
