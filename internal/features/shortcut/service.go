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
	// SetPaused temporarily disables shortcut detection (e.g. while recording a new shortcut).
	SetPaused(paused bool)
}
