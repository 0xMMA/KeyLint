package welcome

import "keylint/internal/features/settings"

// Service handles first-run detection and setup completion.
type Service struct {
	settings *settings.Service
}

// NewService creates a new WelcomeService.
func NewService(s *settings.Service) *Service {
	return &Service{settings: s}
}

// IsFirstRun returns true if the user has not completed the setup wizard.
func (s *Service) IsFirstRun() bool {
	return !s.settings.Get().CompletedSetup
}

// CompleteSetup marks the setup wizard as done.
//
// Through settings.Update, so a save that lands meanwhile (a provider switch)
// is not overwritten by settings read before it.
func (s *Service) CompleteSetup() error {
	return settings.Update(s.settings, func(c *settings.Settings) { c.CompletedSetup = true })
}
