package settings

import (
	"strings"

	"keylint/internal/llm"
)

// Provider holds non-secret configuration for AI providers.
// API keys are stored in the OS keyring, NOT here.
// OllamaURL and the AWS Bedrock connection are non-secret and remain in settings.
type Provider struct {
	OllamaURL string `json:"ollama_url"`
	// AWSRegion is the AWS Bedrock region; "" until the user chooses one.
	AWSRegion string `json:"aws_region"`
	// BedrockAuth is how KeyLint connects to AWS Bedrock: llm.BedrockAuthKey
	// (a Bedrock API key in the keyring; also what "" means) or
	// llm.BedrockAuthSignIn (this computer's AWS sign-in; nothing stored).
	BedrockAuth string `json:"bedrock_auth"`
	// AWSProfile names the AWS profile sign-in mode uses; "" is the default
	// credential chain.
	AWSProfile string `json:"aws_profile"`
	// BedrockRouting is where AWS may process the text: llm.BedrockRoutingGeo
	// (the region's geography; also what "" means) or llm.BedrockRoutingGlobal
	// (anywhere). The user's explicit choice on the AWS Bedrock card.
	BedrockRouting string `json:"bedrock_routing"`
}

// Bedrock is the AWS Bedrock connection as the llm client takes it, with the
// defaults filled in: key mode, keep in the geography.
func (s Settings) Bedrock() llm.BedrockConfig {
	p := s.Providers
	b := llm.BedrockConfig{
		Region:  strings.TrimSpace(p.AWSRegion),
		Auth:    llm.BedrockAuthKey,
		Profile: strings.TrimSpace(p.AWSProfile),
		Routing: llm.BedrockRoutingGeo,
	}
	if p.BedrockAuth == llm.BedrockAuthSignIn {
		b.Auth = llm.BedrockAuthSignIn
	}
	if p.BedrockRouting == llm.BedrockRoutingGlobal {
		b.Routing = llm.BedrockRoutingGlobal
	}
	return b
}

// UsesAPIKey reports whether the provider needs a key as these settings
// configure it — for Bedrock that follows the connection mode.
func (s Settings) UsesAPIKey(provider string) bool {
	return llm.UsesAPIKey(provider, s.Bedrock().Auth)
}

// KeyStatus describes whether a key is configured and where it comes from.
type KeyStatus struct {
	IsSet  bool   `json:"is_set"`
	Source string `json:"source"` // "env", "keyring", or "none"
}

// FeatureModels is the model and effort chosen per feature for one provider.
// An empty model means "use the built-in default" — see llm.DefaultModel. An
// empty effort means "send none", which leaves the model on its own default.
//
// Both live per provider, so switching to another provider and back restores
// what was chosen for each. Older settings files carry no effort fields and
// read as "", which is the behaviour they had — no migration.
type FeatureModels struct {
	Fix        string `json:"fix"`
	Pyramidize string `json:"pyramidize"`

	FixEffort        string `json:"fix_effort"`
	PyramidizeEffort string `json:"pyramidize_effort"`
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

// EffortFor returns the effort configured for a feature, or "" when none is.
func (m FeatureModels) EffortFor(feature string) string {
	switch feature {
	case llm.FeatureFix:
		return m.FixEffort
	case llm.FeaturePyramidize:
		return m.PyramidizeEffort
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

// EffortFor resolves the effort for a provider and feature: a level the
// providers know, or "" — model default — for anything else. A hand-edited
// value that is not a level reads as unset rather than reaching a provider as
// a guaranteed 400.
func (s Settings) EffortFor(provider, feature string) string {
	if effort := strings.TrimSpace(s.Models[provider].EffortFor(feature)); llm.IsEffortLevel(effort) {
		return effort
	}
	return ""
}

// Settings is the top-level application settings structure persisted to disk.
type Settings struct {
	ActiveProvider         string   `json:"active_provider"` // "openai" | "claude" | "openrouter" | "bedrock" | "claude-code" | "ollama"
	Providers              Provider `json:"providers"`
	ShortcutKey            string   `json:"shortcut_key"`              // LEGACY — never applied (Ctrl+G was hard-wired); ignored, kept so old files round-trip
	ShortcutMode           string   `json:"shortcut_mode"`             // "double_tap" | "independent"
	ShortcutFix            string   `json:"shortcut_fix"`              // e.g. "ctrl+g"
	ShortcutPyramidize     string   `json:"shortcut_pyramidize"`       // e.g. "ctrl+shift+g" (independent mode only)
	ShortcutDoubleTapDelay int      `json:"shortcut_double_tap_delay"` // ms, 100-500, default 200
	StartOnBoot            bool     `json:"start_on_boot"`
	ThemePreference        string   `json:"theme_preference"` // "light" | "dark" | "system"
	CompletedSetup         bool     `json:"completed_setup"`
	LogLevel               string   `json:"log_level"`         // "off"|"trace"|"debug"|"info"|"warning"|"error"
	SensitiveLogging       bool     `json:"sensitive_logging"` // logs full API payloads; never share the log file while enabled
	UpdateChannel          string   `json:"update_channel"`    // "" (auto-detect), "stable", or "pre-release"

	// DeveloperOptions unlocks the dev channel in Settings → About: installable
	// builds of open pull requests and of main. Off for everyone until the
	// owner turns it on by tapping the version, and off again with a switch.
	DeveloperOptions bool `json:"developer_options"`

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
		ShortcutMode:               "double_tap",
		ShortcutFix:                "ctrl+g",
		ShortcutPyramidize:         "ctrl+shift+g",
		ShortcutDoubleTapDelay:     200,
		ThemePreference:            "dark",
		LogLevel:                   "off",
		PyramidizeQualityThreshold: DefaultQualityThreshold,
	}
}
