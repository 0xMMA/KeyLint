package enhance

import (
	"fmt"
	"strings"

	"keylint/internal/llm"
)

// Kept out of the eval-tagged files on purpose: which provider a run measures,
// which judge scores it, and whether either needs a key decide what a baseline
// means, and that has to be covered by the normal test suite rather than only
// by a paid run. The Pyramidize suite carries the same rules; the two harnesses
// mirror each other so eval-aggregate.sh reads both.

// defaultEvalProvider is a constant rather than the machine's active provider:
// a baseline that changes with whoever runs it is not a baseline.
//
// The installed Claude Code CLI since 2026-10, so a run spends the owner's
// subscription rather than an API key. `--provider claude` still measures the API.
const defaultEvalProvider = llm.ProviderClaudeCode

// evalTarget resolves the provider and model a run measures. For the CLI the
// model is the alias users get ("haiku"); summary.json records what it
// resolved to.
func evalTarget(getenv func(string) string) (provider, model string) {
	provider = strings.TrimSpace(getenv("EVAL_PROVIDER"))
	if provider == "" {
		provider = defaultEvalProvider
	}
	model = strings.TrimSpace(getenv("EVAL_MODEL"))
	if model == "" {
		model = llm.DefaultModel(provider, llm.FeatureFix)
	}
	return provider, model
}

// JudgeConfig pins the instrument, exactly as the Pyramidize eval does — a
// judge that moves with what it measures reports nothing. Same provider and
// dated snapshot, so the two suites' judge columns are produced the same way.
type JudgeConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// Temperature is 0 where the provider honours a pin and nil — null in
	// summary.json — where it cannot: the Claude Code CLI has no flag for it.
	// null keys differently from 0 in eval-aggregate.sh, so a CLI-judged run is
	// never compared as equal to an API-judged one.
	Temperature *float64 `json:"temperature"`
	// TemperatureNote says why Temperature is null. Empty when it is pinned.
	TemperatureNote string `json:"temperatureNote,omitempty"`
	// ResolvedModel is the model ID that actually answered, filled in after a
	// run from what the provider reported.
	ResolvedModel string `json:"resolvedModel,omitempty"`
}

const (
	// The CLI since 2026-10, for the same reason as the pipeline. It takes the
	// dated ID and answers with it (checked 2026-10-04 against modelUsage), so
	// the instrument stays frozen; what it loses is the temperature pin.
	defaultJudgeProvider = llm.ProviderClaudeCode
	defaultJudgeModel    = "claude-sonnet-4-5-20250929"
	judgeTemperature     = 0.0
	judgeTemperatureNote = "not pinned: the Claude Code CLI has no temperature flag, so the judge samples at the CLI default"
)

// JudgeConfigFromEnv resolves the judge, pinned unless deliberately overridden.
func JudgeConfigFromEnv(getenv func(string) string) JudgeConfig {
	cfg := JudgeConfig{Provider: defaultJudgeProvider, Model: defaultJudgeModel}
	if v := strings.TrimSpace(getenv("EVAL_JUDGE_PROVIDER")); v != "" {
		cfg.Provider = v
	}
	if v := strings.TrimSpace(getenv("EVAL_JUDGE_MODEL")); v != "" {
		cfg.Model = v
	}
	if llm.SupportsTemperature(cfg.Provider) {
		cfg.Temperature = llm.Temp(judgeTemperature)
	} else {
		cfg.TemperatureNote = judgeTemperatureNote
	}
	return cfg
}

// TemperatureLabel is the judge's temperature for a log line.
func (c JudgeConfig) TemperatureLabel() string {
	if c.Temperature == nil {
		return "unpinned (CLI default)"
	}
	return fmt.Sprintf("%.1f", *c.Temperature)
}

// evalNeedsAPIKey reports whether any provider in a run authenticates with a
// key KeyLint supplies.
func evalNeedsAPIKey(providers ...string) bool {
	for _, p := range providers {
		if llm.UsesAPIKey(p) {
			return true
		}
	}
	return false
}

// evalEnvFromFile decides which .env entries a run takes into its environment:
// non-credentials fill only what the caller left empty, and credentials are
// taken — over the shell's — only when the pipeline or the judge is an API
// provider. A run entirely through the Claude Code CLI takes no key at all.
func evalEnvFromFile(file map[string]string, getenv func(string) string) map[string]string {
	set := map[string]string{}
	for key, value := range file {
		if strings.HasSuffix(key, "_API_KEY") {
			continue
		}
		if getenv(key) == "" {
			set[key] = value
		}
	}
	merged := func(key string) string {
		if v, ok := set[key]; ok {
			return v
		}
		return getenv(key)
	}
	provider, _ := evalTarget(merged)
	if !evalNeedsAPIKey(provider, JudgeConfigFromEnv(merged).Provider) {
		return set
	}
	for key, value := range file {
		if strings.HasSuffix(key, "_API_KEY") {
			set[key] = value
		}
	}
	return set
}
