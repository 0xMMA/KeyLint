package settings

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

// AppPreset maps a source application name to a preferred document type for Pyramidize.
type AppPreset struct {
	SourceApp    string `json:"sourceApp"`
	DocumentType string `json:"documentType"`
}

// DefaultQualityThreshold is the default quality threshold for Pyramidize refinement.
const DefaultQualityThreshold = 0.65

// Settings is the top-level application settings structure persisted to disk.
type Settings struct {
	ActiveProvider  string   `json:"active_provider"` // "openai" | "claude" | "ollama" | "bedrock"
	Providers       Provider `json:"providers"`
	ShortcutKey             string `json:"shortcut_key"`              // LEGACY — migrated to ShortcutFix on load
	ShortcutMode            string `json:"shortcut_mode"`             // "double_tap" | "independent"
	ShortcutFix             string `json:"shortcut_fix"`              // e.g. "ctrl+g"
	ShortcutPyramidize      string `json:"shortcut_pyramidize"`       // e.g. "ctrl+shift+g" (independent mode only)
	ShortcutDoubleTapDelay  int    `json:"shortcut_double_tap_delay"` // ms, 100-500, default 200
	StartOnBoot     bool     `json:"start_on_boot"`
	ThemePreference string   `json:"theme_preference"` // "light" | "dark" | "system"
	CompletedSetup  bool     `json:"completed_setup"`
	LogLevel          string   `json:"log_level"`           // "off"|"trace"|"debug"|"info"|"warning"|"error"
	SensitiveLogging  bool     `json:"sensitive_logging"`  // logs full API payloads; never share the log file while enabled
	UpdateChannel     string   `json:"update_channel"`     // "" (auto-detect), "stable", or "pre-release"

	// Pyramidize settings
	AppPresets                []AppPreset `json:"app_presets"`
	PyramidizeQualityThreshold float64    `json:"pyramidize_quality_threshold"` // default 0.65
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
