package settings

import (
	"strings"

	"keylint/internal/llm"
)

// Provider holds non-secret configuration for AI providers.
// API keys are stored in the OS keyring, NOT here.
// OllamaURL and AWSRegion are non-secret and remain in settings.
type Provider struct {
	OllamaURL string `json:"ollama_url"`
	AWSRegion string `json:"aws_region"`
}

// KeyStatus describes whether a key is configured and where it comes from.
type KeyStatus struct {
	IsSet  bool   `json:"is_set"`
	Source string `json:"source"` // "env", "keyring", or "none"
}

// FeatureModels is the model chosen per feature for one provider. An empty
// string means "use the built-in default" — see llm.DefaultModel.
type FeatureModels struct {
	Fix        string `json:"fix"`
	Pyramidize string `json:"pyramidize"`
}

// For returns the model configured for a feature, or "" when none is.
func (m FeatureModels) For(feature string) string {
	switch feature {
	case llm.FeatureFix:
		return m.Fix
	case llm.FeaturePyramidize:
		return m.Pyramidize
	}
	return ""
}

// AppPreset maps a source application name to a preferred document type for Pyramidize.
type AppPreset struct {
	SourceApp    string `json:"sourceApp"`
	DocumentType string `json:"documentType"`
}

// DefaultQualityThreshold is the default quality threshold for Pyramidize refinement.
const DefaultQualityThreshold = 0.65

// ModelFor resolves the model for a provider and feature: what the user chose,
// else the built-in default.
func (s Settings) ModelFor(provider, feature string) string {
	if chosen := strings.TrimSpace(s.Models[provider].For(feature)); chosen != "" {
		return chosen
	}
	return llm.DefaultModel(provider, feature)
}

// Settings is the top-level application settings structure persisted to disk.
type Settings struct {
	ActiveProvider   string   `json:"active_provider"` // "openai" | "claude" | "claude-code" | "ollama" | "bedrock"
	Providers        Provider `json:"providers"`
	ShortcutKey      string   `json:"shortcut_key"` // e.g. "ctrl+g"
	StartOnBoot      bool     `json:"start_on_boot"`
	ThemePreference  string   `json:"theme_preference"` // "light" | "dark" | "system"
	CompletedSetup   bool     `json:"completed_setup"`
	LogLevel         string   `json:"log_level"`         // "off"|"trace"|"debug"|"info"|"warning"|"error"
	SensitiveLogging bool     `json:"sensitive_logging"` // logs full API payloads; never share the log file while enabled
	UpdateChannel    string   `json:"update_channel"`    // "" (auto-detect), "stable", or "pre-release"

	// Models holds the model chosen per provider and feature. An absent key or
	// an empty string means the built-in default, so older settings files need
	// no migration.
	Models map[string]FeatureModels `json:"models"`

	// Pyramidize settings
	AppPresets                 []AppPreset `json:"app_presets"`
	PyramidizeQualityThreshold float64     `json:"pyramidize_quality_threshold"` // default 0.65
}

// Default returns a Settings with sensible defaults.
func Default() Settings {
	return Settings{
		ActiveProvider:             "openai",
		ShortcutKey:                "ctrl+g",
		ThemePreference:            "dark",
		LogLevel:                   "off",
		PyramidizeQualityThreshold: DefaultQualityThreshold,
	}
}
