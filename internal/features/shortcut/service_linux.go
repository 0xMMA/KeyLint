//go:build !windows

package shortcut

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

func (s *linuxService) Register() error   { return nil }
func (s *linuxService) Unregister()        {}
func (s *linuxService) Triggered() <-chan ShortcutEvent { return s.ch }

// Simulate fires a synthetic shortcut event (used by --simulate-shortcut and dev UI).
func (s *linuxService) Simulate() {
	s.ch <- ShortcutEvent{Source: "simulate"}
}
