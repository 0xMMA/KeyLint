package settings

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"keylint/internal/llm"
	"keylint/internal/logger"

	keyring "github.com/zalando/go-keyring"
)

const appName = "KeyLint"

// errIsolated is what a service built with NewServiceFrom answers to a request
// that would reach outside its explicit configuration.
var errIsolated = errors.New("settings: this service has an explicit key source and does not use the keyring")

// envVars maps provider ID → environment variable name for the API key.
// Empty string means no standard env var for that provider.
var envVars = map[string]string{
	"openai":  "OPENAI_API_KEY",
	"claude":  "ANTHROPIC_API_KEY",
	"bedrock": "AWS_SECRET_ACCESS_KEY",
}

// Service handles loading and saving application settings.
type Service struct {
	// filePath is empty for a service built with NewServiceFrom: it has no file
	// to read or write, which is what makes it isolated.
	filePath string
	// keys overrides where API keys come from, and isolated says whether this
	// service was built that way at all. Two fields rather than a nil check:
	// NewServiceFrom(cfg, nil) must not silently fall back to the environment
	// and the keyring, which is exactly what an isolated service exists to
	// avoid, and a nil lookup is an easy thing for a caller to pass.
	keys     KeyLookup
	isolated bool

	// currentMu guards current. Wails serves every RPC on its own goroutine, so
	// saving settings and loading a model list genuinely run at the same time —
	// the listing path reads the Ollama URL out of current while Save replaces
	// it. Get copies the reference fields too, because callers do write into
	// them: SetAppPreset edits AppPresets[i] in the value it was handed.
	currentMu sync.RWMutex
	current   Settings

	// claudeCode caches the CLI probe; see GetClaudeCodeStatus. Spawning
	// processes on every screen that asks is what #55 was about.
	claudeCodeMu sync.Mutex
	claudeCode   llm.ClaudeCodeStatus
	claudeCodeAt time.Time
	// claudeCodeProbe is non-nil while a probe is running; callers that arrive
	// meanwhile wait on it instead of starting their own.
	claudeCodeProbe chan struct{}
	// probeClaudeCode is the probe itself, swappable so a test can count calls
	// without spawning anything. nil means the real one.
	probeClaudeCode func(context.Context) llm.ClaudeCodeStatus

	// models caches per-provider listings; see ListModels.
	modelsMu sync.Mutex
	models   map[string]cachedModelList
	// modelGeneration counts invalidations per provider. A listing that started
	// before one must not write its result afterwards; see ListModels and
	// forgetModelList. Per provider rather than global, so pasting an OpenAI key
	// does not throw away an Anthropic listing that is already on its way back.
	modelGeneration map[string]uint64
}

// KeyLookup resolves a provider's API key.
//
// Production uses the environment-then-keyring lookup; an eval run or a test
// passes one that reads nothing else, so a machine's keyring and the developer's
// own settings cannot change what a measurement produces.
type KeyLookup func(provider string) string

// EnvOnlyKeys reads API keys from the environment and nowhere else. `.env` is
// loaded into the environment before this is called, so it is the eval's key
// source: no keyring, no prompt, no fallback.
func EnvOnlyKeys(provider string) string {
	if envVar, ok := envVars[provider]; ok && envVar != "" {
		return os.Getenv(envVar)
	}
	return ""
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

// NewServiceFrom builds a service around an explicit configuration and key
// source. It reads no settings file and touches no keyring, so a run against it
// measures the configuration it was given rather than whatever the machine
// happens to hold.
//
// Saving is in-memory only: there is no file to write, and writing to the
// user's real one would be the opposite of what this constructor is for.
func NewServiceFrom(cfg Settings, keys KeyLookup) *Service {
	if keys == nil {
		// No lookup means no keys — never the ambient ones.
		keys = func(string) string { return "" }
	}
	return &Service{current: cfg.clone(), keys: keys, isolated: true}
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
//
// The copy reaches into the reference fields: a struct copy would share the
// AppPresets array and the Models map with every other caller and with the
// service's own state, and callers do edit what they are given — SetAppPreset
// assigns into AppPresets[i] before handing the result back to Save. Sharing
// them means that edit lands in this service's settings without a Save, and
// races with any concurrent read.
func (s *Service) Get() Settings {
	s.currentMu.RLock()
	defer s.currentMu.RUnlock()
	return s.current.clone()
}

// clone deep-copies the fields that are references. Every other field is a
// value and is copied by the struct assignment itself; a new reference field
// added to Settings belongs here too.
func (s Settings) clone() Settings {
	out := s
	out.AppPresets = slices.Clone(s.AppPresets)
	if s.Models != nil {
		out.Models = maps.Clone(s.Models)
	}
	return out
}

// Save persists the provided settings to disk.
func (s *Service) Save(updated Settings) error {
	s.currentMu.Lock()
	ollamaMoved := updated.Providers.OllamaURL != s.current.Providers.OllamaURL
	// Cloned on the way in as well: the caller still holds `updated` and may
	// edit its presets afterwards, which would otherwise mutate live settings.
	s.current = updated.clone()
	s.currentMu.Unlock()

	// Ollama's list comes from whatever URL is configured.
	if ollamaMoved {
		s.forgetModelList(llm.ProviderOllama)
	}
	if s.filePath == "" {
		// Built with NewServiceFrom: the update is kept in memory, because the
		// only file this could write to is the user's real one.
		return nil
	}
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
	if s.isolated {
		if s.keys(provider) != "" {
			return KeyStatus{IsSet: true, Source: "env"}
		}
		return KeyStatus{IsSet: false, Source: "none"}
	}
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
	if s.isolated {
		return s.keys(provider)
	}
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
	if s.isolated {
		return errIsolated
	}
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
	if s.isolated {
		return errIsolated
	}
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

// ListModels returns the models a provider can serve, cached per provider.
//
// It never returns an error: a provider that cannot be reached, one with no key
// and one whose listing this app cannot use all yield the built-in list, and a
// provider that listed nothing yields an empty one. ModelList.Source says which
// happened, so the picker can explain itself; ttl decides how long that answer
// is worth keeping.
func (s *Service) ListModels(provider string) llm.ModelList {
	// Two attempts, because an invalidation that lands mid-listing retires the
	// answer on its way back: the key or the URL it asked with is gone. Asking
	// once more with what replaced it is the only way to return a list that
	// describes the current settings rather than the previous ones. Bounded, so
	// a user holding down Save cannot keep this call alive.
	for attempt := 0; attempt < 2; attempt++ {
		s.modelsMu.Lock()
		cached, ok := s.models[provider]
		generation := s.modelGeneration[provider]
		s.modelsMu.Unlock()
		if ok && time.Since(cached.fetched) < cached.ttl() {
			return cached.list
		}

		cfg, ok := s.modelListConfig(provider)
		if !ok {
			// No credential yet: the request would only earn a 401, and caching
			// that would keep the built-in list on screen after the user pastes
			// a key.
			return llm.CuratedModels(provider, llm.ModelSourceNoCredentials)
		}

		ctx, cancel := context.WithTimeout(context.Background(), modelListTimeout)
		list, err := llm.ListModels(ctx, provider, cfg)
		cancel()
		if err != nil {
			logger.Info("settings: model list unavailable, using the built-in one",
				"provider", provider, "err", err)
		}

		s.modelsMu.Lock()
		if s.modelGeneration[provider] != generation {
			// Retired mid-flight. Neither cache it nor hand it back — it answers
			// a question about settings the user has already replaced.
			s.modelsMu.Unlock()
			continue
		}
		if s.models == nil {
			s.models = map[string]cachedModelList{}
		}
		s.models[provider] = cachedModelList{list: list, fetched: time.Now()}
		s.modelsMu.Unlock()
		return list
	}

	// Invalidated twice while listing — the user is mid-edit. The built-in list
	// keeps the picker usable and expires in 30 seconds, by which time whatever
	// they are typing has settled.
	return llm.CuratedModels(provider, llm.ModelSourceUnreachable)
}

// ttl is how long this entry stays valid.
//
// Read off the source rather than stored alongside it, so the two cannot drift
// apart. Only a settled answer earns the long lifetime: the provider listed its
// models, or there is no endpoint to ask and nothing can change. Everything
// else describes something the user is in the middle of fixing — starting a
// daemon, pulling a model, pasting a key, widening a project scope — and ten
// minutes of the old answer after that is ten minutes of a wrong picker.
//
// The default is deliberately the short one, so a source added later errs
// towards asking again rather than towards a stale picker.
func (c cachedModelList) ttl() time.Duration {
	switch c.list.Source {
	case llm.ModelSourceLive, llm.ModelSourceFixed:
		return modelListTTL
	default:
		return modelListFailureTTL
	}
}

// forgetModelList drops a provider's cached listing, so the next ask goes to the
// provider. Without this, pasting a key leaves the built-in list on screen for
// the rest of the TTL — which is the one flow model selection exists for.
func (s *Service) forgetModelList(provider string) {
	s.modelsMu.Lock()
	delete(s.models, provider)
	// A listing that is already in flight asked with the credentials or the URL
	// this invalidation just retired. Moving this provider's generation on makes
	// it drop its answer instead of writing it back over the deletion.
	if s.modelGeneration == nil {
		s.modelGeneration = map[string]uint64{}
	}
	s.modelGeneration[provider]++
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

// claudeCodeStatusTimeoutForTest lets a test exercise the timeout path without
// waiting out the real deadline.
var claudeCodeStatusTimeoutForTest = claudeCodeStatusTimeout

// claudeCodeStatusTTL is how long a probe result is reused.
//
// The probe spawns processes, and four screens ask for it — the Pyramidize page
// on load and on every provider change, the settings card, and the welcome
// wizard. A minute is long enough that opening those in sequence costs one
// probe, and short enough that someone who signs in elsewhere and comes back
// sees it without hunting for the re-check button.
const claudeCodeStatusTTL = 60 * time.Second

// GetClaudeCodeStatus reports whether the Claude Code CLI is installed on this
// machine and signed in, so the UI can offer it as a provider that needs no API
// key. Signing in happens in the user's own terminal through Anthropic's flow —
// KeyLint only looks, and never reads or stores credentials.
//
// force skips the cache. The re-check button passes it, because a user pressing
// it has just done something they expect to be noticed; everything else takes
// the cached answer.
func (s *Service) GetClaudeCodeStatus(force bool) llm.ClaudeCodeStatus {
	for {
		s.claudeCodeMu.Lock()

		if !force {
			cached, fetchedAt := s.claudeCode, s.claudeCodeAt
			if !fetchedAt.IsZero() && time.Since(fetchedAt) < claudeCodeTTL(cached) {
				s.claudeCodeMu.Unlock()
				return cached
			}
		}

		// Somebody is already probing. Wait for them rather than starting a
		// second one: four screens opening at once on a cold cache would
		// otherwise spawn four probes, which is the cost #55 is about.
		//
		// force is deliberately NOT cleared here. A waiter that asked for a
		// fresh answer loops round and starts its own probe, because the one it
		// waited for may have begun up to claudeCodeStatusTimeout before the
		// button was pressed — which is exactly the staleness the button exists
		// to escape.
		if inFlight := s.claudeCodeProbe; inFlight != nil {
			s.claudeCodeMu.Unlock()
			<-inFlight
			continue
		}

		done := make(chan struct{})
		s.claudeCodeProbe = done
		s.claudeCodeMu.Unlock()

		status, timedOut := s.runClaudeCodeProbe()

		// Deferred, so a panic in the probe cannot leave every later caller
		// waiting on a channel nobody will ever close.
		func() {
			s.claudeCodeMu.Lock()
			defer func() {
				s.claudeCodeProbe = nil
				s.claudeCodeMu.Unlock()
				close(done)
			}()
			if timedOut {
				// A cut-short probe says nothing. Caching it would turn one slow
				// spawn into a minute of "not signed in", and the welcome wizard
				// has no re-check button to escape that with.
				return
			}
			s.claudeCode, s.claudeCodeAt = status, time.Now()
		}()
		return status
	}
}

// claudeCodeNegativeTTL is how long a "not there / not signed in" answer is
// reused. Short, because it is the answer a user is about to change — they are
// installing the CLI or signing in, and coming back to a stale no is the one
// thing this cache must not cause.
const claudeCodeNegativeTTL = 10 * time.Second

// ttl is how long a cached answer stays good. A working CLI is a settled fact;
// anything else is a situation in progress.
func claudeCodeTTL(status llm.ClaudeCodeStatus) time.Duration {
	if status.Installed && status.LoggedIn {
		return claudeCodeStatusTTL
	}
	return claudeCodeNegativeTTL
}

// runClaudeCodeProbe is the probe itself, bounded so a wedged binary cannot hold
// a screen open indefinitely.
func (s *Service) runClaudeCodeProbe() (status llm.ClaudeCodeStatus, timedOut bool) {
	ctx, cancel := context.WithTimeout(context.Background(), claudeCodeStatusTimeoutForTest)
	defer cancel()
	if s.probeClaudeCode != nil {
		status = s.probeClaudeCode(ctx)
	} else {
		status = llm.CheckClaudeCode(ctx, "")
	}
	// CheckClaudeCode has no error return: a spawn cut short by the deadline
	// comes back as "installed, not signed in", which is indistinguishable from
	// the real thing. The context is where that difference survives.
	return status, ctx.Err() != nil
}

// ResetToDefaults resets settings to their default values and saves to disk.
func (s *Service) ResetToDefaults() error {
	return s.Save(Default())
}
