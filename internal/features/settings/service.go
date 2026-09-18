package settings

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"keylint/internal/llm"
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

	// models caches per-provider listings; see ListModels.
	modelsMu sync.Mutex
	models   map[string]cachedModelList
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

	logger.Info("settings: loaded", "path", s.filePath)
	return nil
}

// Get returns a copy of the current settings.
func (s *Service) Get() Settings {
	return s.current
}

// Save persists the provided settings to disk.
func (s *Service) Save(updated Settings) error {
	// Ollama's list comes from whatever URL is configured.
	if updated.Providers.OllamaURL != s.current.Providers.OllamaURL {
		s.forgetModelList(llm.ProviderOllama)
	}
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
	if err := keyring.Set(appName, provider, key); err != nil {
		return err
	}
	// Only now: a write that failed leaves the old key in place, and dropping
	// the list for it would cost a listing round trip that changes nothing.
	s.forgetModelList(provider)
	return nil
}

// DeleteKey removes an API key for the given provider from the OS keyring.
func (s *Service) DeleteKey(provider string) error {
	if err := keyring.Delete(appName, provider); err != nil {
		return err
	}
	s.forgetModelList(provider)
	return nil
}

// modelListFailureTTL is how long a failed listing is remembered. Short,
// because the usual cause is something the user is about to fix — a key that
// is not pasted yet, a daemon that is not started.
const modelListFailureTTL = 30 * time.Second

// modelListTTL keeps a provider's model list for a while: the settings screen
// and the Pyramidize selector both ask for it, and a listing call costs a round
// trip to the provider (or a process spawn, for a local daemon that is down).
const modelListTTL = 10 * time.Minute

// modelListTimeout bounds one listing call so a wedged provider cannot hang the
// settings screen.
const modelListTimeout = 20 * time.Second

// cachedModelList is one provider's list and when it was fetched. How long it
// stays valid follows from the list's own Source; see ttl.
type cachedModelList struct {
	list    llm.ModelList
	fetched time.Time
}

// ListModels returns the models a provider can serve, cached for modelListTTL.
// A provider that cannot be reached yields the built-in list with
// source "static" rather than an error: a picker with the usual entries is more
// use than an empty one, and the source says which it is.
func (s *Service) ListModels(provider string) llm.ModelList {
	s.modelsMu.Lock()
	cached, ok := s.models[provider]
	s.modelsMu.Unlock()
	if ok && time.Since(cached.fetched) < cached.ttl() {
		return cached.list
	}

	ctx, cancel := context.WithTimeout(context.Background(), modelListTimeout)
	defer cancel()

	cfg, ok := s.modelListConfig(provider)
	if !ok {
		// No credential yet: the request would only earn a 401, and caching that
		// would keep the built-in list on screen after the user pastes a key.
		return llm.CuratedModels(provider, llm.ModelSourceNoCredentials)
	}

	list, err := llm.ListModels(ctx, provider, cfg)
	if err != nil {
		logger.Info("settings: model list unavailable, using the built-in one",
			"provider", provider, "err", err)
	}

	s.modelsMu.Lock()
	if s.models == nil {
		s.models = map[string]cachedModelList{}
	}
	s.models[provider] = cachedModelList{list: list, fetched: time.Now()}
	s.modelsMu.Unlock()
	return list
}

// ttl is how long this entry stays valid.
//
// Read off the source rather than stored alongside it, so the two cannot drift
// apart. A provider that was down and one that had nothing pulled are both
// states the user is about to change — a daemon started, a model pulled — and
// ten minutes of a stale answer after that is ten minutes of the picker being
// wrong.
func (c cachedModelList) ttl() time.Duration {
	switch c.list.Source {
	case llm.ModelSourceUnreachable, llm.ModelSourceEmpty:
		return modelListFailureTTL
	default:
		return modelListTTL
	}
}

// forgetModelList drops a provider's cached listing, so the next ask goes to the
// provider. Without this, pasting a key leaves the built-in list on screen for
// the rest of the TTL — which is the one flow model selection exists for.
func (s *Service) forgetModelList(provider string) {
	s.modelsMu.Lock()
	delete(s.models, provider)
	s.modelsMu.Unlock()
}

// modelListConfig gives a listing call the credentials and endpoint a
// completion would use. The Feature field only tags the log line; no model is
// resolved here, because a listing does not generate anything.
// The second return is false when the provider cannot be asked at all yet.
func (s *Service) modelListConfig(provider string) (llm.Config, bool) {
	cfg := llm.Config{Feature: "settings"}
	switch provider {
	case llm.ProviderOpenAI, llm.ProviderClaude:
		cfg.APIKey = s.GetKey(provider)
		if cfg.APIKey == "" {
			return cfg, false
		}
	case llm.ProviderOllama:
		cfg.BaseURL = s.Get().Providers.OllamaURL
	}
	return cfg, true
}

// claudeCodeStatusTimeout bounds the whole detection so the settings screen and
// the welcome wizard cannot be blocked by a wedged binary.
const claudeCodeStatusTimeout = 25 * time.Second

// GetClaudeCodeStatus reports whether the Claude Code CLI is installed on this
// machine and signed in, so the UI can offer it as a provider that needs no API
// key. Signing in happens in the user's own terminal through Anthropic's flow —
// KeyLint only looks, and never reads or stores credentials.
func (s *Service) GetClaudeCodeStatus() llm.ClaudeCodeStatus {
	ctx, cancel := context.WithTimeout(context.Background(), claudeCodeStatusTimeout)
	defer cancel()
	return llm.CheckClaudeCode(ctx, "")
}

// ResetToDefaults resets settings to their default values and saves to disk.
func (s *Service) ResetToDefaults() error {
	return s.Save(Default())
}
