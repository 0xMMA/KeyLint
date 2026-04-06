package settings

import (
	"encoding/json"
	"os"
	"path/filepath"

	"keylint/internal/logger"

	keyring "github.com/zalando/go-keyring"
)

const appName = "KeyLint"

// envVars maps provider ID → environment variable name for the API key.
// Empty string means no standard env var for that provider.
var envVars = map[string]string{
	"openai":  "OPENAI_API_KEY",
	"claude":  "ANTHROPIC_API_KEY",
	"bedrock": "AWS_SECRET_ACCESS_KEY",
}

// Service handles loading and saving application settings.
type Service struct {
	filePath string
	current  Settings
}

// NewService creates a new SettingsService, loading existing settings from disk.
func NewService() (*Service, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(configDir, appName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	svc := &Service{
		filePath: filepath.Join(dir, "settings.json"),
		current:  Default(),
	}
	if err := svc.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return svc, nil
}

func (s *Service) load() error {
	data, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		logger.Info("settings: file not found, using defaults", "path", s.filePath)
		return err
	}
	if err != nil {
		logger.Error("settings: read failed", "path", s.filePath, "err", err)
		return err
	}
	if err := json.Unmarshal(data, &s.current); err != nil {
		logger.Error("settings: unmarshal failed", "err", err)
		return err
	}

	// Migrate legacy debug_logging → log_level.
	// Check whether the raw JSON contains "log_level"; if not, this is a legacy
	// file and we derive the level from the old "debug_logging" boolean.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		if _, hasLogLevel := raw["log_level"]; !hasLogLevel {
			if val, ok := raw["debug_logging"]; ok {
				var debugOn bool
				if json.Unmarshal(val, &debugOn) == nil && debugOn {
					s.current.LogLevel = "debug"
				} else {
					s.current.LogLevel = "off"
				}
			} else {
				s.current.LogLevel = "off"
			}
			logger.Info("settings: migrated debug_logging to log_level", "log_level", s.current.LogLevel)
		}
	}

	// Migrate legacy shortcut_key → shortcut_fix + defaults.
	if s.current.ShortcutFix == "" && s.current.ShortcutKey != "" {
		s.current.ShortcutFix = s.current.ShortcutKey
		s.current.ShortcutMode = "double_tap"
		s.current.ShortcutPyramidize = "ctrl+shift+g"
		s.current.ShortcutDoubleTapDelay = 200
		logger.Info("settings: migrated shortcut_key to shortcut_fix", "key", s.current.ShortcutFix)
	}

	logger.Info("settings: loaded", "path", s.filePath)
	return nil
}

// Get returns a copy of the current settings.
func (s *Service) Get() Settings {
	return s.current
}

// Save persists the provided settings to disk.
func (s *Service) Save(updated Settings) error {
	s.current = updated
	data, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.filePath, data, 0600); err != nil {
		return err
	}
	logger.Info("settings: saved", "path", s.filePath)
	return nil
}

// GetKeyStatus returns whether an API key is configured for the given provider,
// and where it comes from ("env", "keyring", or "none").
func (s *Service) GetKeyStatus(provider string) KeyStatus {
	if envVar, ok := envVars[provider]; ok && envVar != "" {
		if os.Getenv(envVar) != "" {
			return KeyStatus{IsSet: true, Source: "env"}
		}
	}
	_, err := keyring.Get(appName, provider)
	if err == nil {
		return KeyStatus{IsSet: true, Source: "keyring"}
	}
	return KeyStatus{IsSet: false, Source: "none"}
}

// GetKey returns the API key for the given provider.
// Priority: environment variable → OS keyring.
// Returns empty string if not configured.
func (s *Service) GetKey(provider string) string {
	if envVar, ok := envVars[provider]; ok && envVar != "" {
		if val := os.Getenv(envVar); val != "" {
			return val
		}
	}
	key, err := keyring.Get(appName, provider)
	if err == nil {
		return key
	}
	return ""
}

// SetKey stores an API key for the given provider in the OS keyring.
// Returns an error if the keyring is unavailable on this platform.
func (s *Service) SetKey(provider, key string) error {
	return keyring.Set(appName, provider, key)
}

// DeleteKey removes an API key for the given provider from the OS keyring.
func (s *Service) DeleteKey(provider string) error {
	return keyring.Delete(appName, provider)
}

// ResetToDefaults resets settings to their default values and saves to disk.
func (s *Service) ResetToDefaults() error {
	return s.Save(Default())
}
