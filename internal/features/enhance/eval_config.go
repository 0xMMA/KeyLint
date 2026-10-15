package enhance

import (
	"context"
	"fmt"
	"slices"
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

// fixThinkingLabel is the thinking a Fix run records, and keys on: what the
// Fix service does about reasoning on this route (see Enhance; the eval sets no
// effort). Through the Claude Code CLI the model thinks — the CLI's default,
// and the CLI has no switch KeyLint uses ("on"). On an API route Fix asks a
// fast-tier model to answer without reasoning ("off"); any other model is left
// to the provider ("n/a"). In the configKey so a change to that rule reads
// "not comparable" rather than as a prompt effect.
func fixThinkingLabel(provider, model string) string {
	switch {
	case provider == llm.ProviderClaudeCode:
		return "on"
	case llm.IsFastTier(model):
		return "off"
	default:
		return "n/a"
	}
}

// JudgeConfig is one judge, pinned exactly as the Pyramidize eval pins its
// judges — a judge that moves with what it measures reports nothing. Same
// provider and models, so the two suites' judge columns are produced the same
// way.
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
	// The CLI since 2026-10, for the same reason as the pipeline; what the
	// judges lose there is the temperature pin.
	defaultJudgeProvider      = llm.ProviderClaudeCode
	judgeTemperature          = 0.0
	judgeTemperatureNote      = "not pinned: the Claude Code CLI has no temperature flag, so the judge samples at the CLI default (and thinks first, as the CLI does by default)"
	judgeModelTemperatureNote = "not pinned: this model takes no temperature on the API (the 4.7 generation and later reject a non-default one), so the judge samples at the model default"
)

// defaultJudgeModels: two judges since 2026-10-11, the same pair as the
// Pyramidize suite, reported separately and never averaged. Full model IDs —
// Haiku 5.5 and Sonnet 5.5 have no dated form — and each run records the ID
// that answered for each judge.
var defaultJudgeModels = []string{"claude-haiku-5-5", "claude-sonnet-5-5"}

// JudgesFromEnv resolves the judge set, pinned unless deliberately overridden:
// EVAL_JUDGES (comma-separated model IDs) chooses the set, EVAL_JUDGE_MODEL
// names a single judge, EVAL_JUDGE_PROVIDER moves all of them. Both model
// variables together, an empty entry or a duplicate are refused.
func JudgesFromEnv(getenv func(string) string) ([]JudgeConfig, error) {
	models := defaultJudgeModels
	list := strings.TrimSpace(getenv("EVAL_JUDGES"))
	single := strings.TrimSpace(getenv("EVAL_JUDGE_MODEL"))
	switch {
	case list != "" && single != "":
		return nil, fmt.Errorf("EVAL_JUDGES and EVAL_JUDGE_MODEL are both set; use EVAL_JUDGES")
	case list != "":
		models = nil
		for _, m := range strings.Split(list, ",") {
			m = strings.TrimSpace(m)
			if m == "" {
				return nil, fmt.Errorf("EVAL_JUDGES %q has an empty entry", list)
			}
			if slices.Contains(models, m) {
				return nil, fmt.Errorf("EVAL_JUDGES names %q twice", m)
			}
			models = append(models, m)
		}
	case single != "":
		models = []string{single}
	}
	provider := judgeProviderFrom(getenv)
	judges := make([]JudgeConfig, len(models))
	for i, m := range models {
		judges[i] = JudgeConfig{Provider: provider, Model: m}
		switch {
		case llm.SupportsTemperature(provider, m):
			judges[i].Temperature = llm.Temp(judgeTemperature)
		case provider == llm.ProviderClaudeCode:
			judges[i].TemperatureNote = judgeTemperatureNote
		default:
			// An API judge whose model takes no temperature: the 5.5 judges.
			judges[i].TemperatureNote = judgeModelTemperatureNote
		}
	}
	return judges, nil
}

// judgeProviderFrom is the provider every judge runs on.
func judgeProviderFrom(getenv func(string) string) string {
	if v := strings.TrimSpace(getenv("EVAL_JUDGE_PROVIDER")); v != "" {
		return v
	}
	return defaultJudgeProvider
}

// TemperatureLabel is the judge's temperature for a log line.
func (c JudgeConfig) TemperatureLabel() string {
	if c.Temperature == nil {
		return "unpinned (model default)"
	}
	return fmt.Sprintf("%.1f", *c.Temperature)
}

// evalNeedsAPIKey reports whether any provider in a run authenticates with a
// key KeyLint supplies.
func evalNeedsAPIKey(providers ...string) bool {
	for _, p := range providers {
		// "" is Bedrock's default mode, a key: an eval configures no Bedrock
		// connection, so there is no other mode to read.
		if llm.UsesAPIKey(p, "") {
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
		if strings.HasSuffix(key, "_API_KEY") || !isEvalSetting(key) {
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
	if !evalNeedsAPIKey(provider, judgeProviderFrom(merged)) {
		return set
	}
	for key, value := range file {
		// An empty line copied from .env.example must not blank a key the shell
		// exports: credentials from the file win, but only real ones.
		if strings.HasSuffix(key, "_API_KEY") && strings.TrimSpace(value) != "" {
			set[key] = value
		}
	}
	return set
}

// evalCLIVersion is the Claude Code CLI's version when the pipeline or the judge
// runs through it, and "" otherwise.
func evalCLIVersion(providers ...string) string {
	for _, p := range providers {
		if p == llm.ProviderClaudeCode {
			return llm.ClaudeCodeVersion(context.Background(), "")
		}
	}
	return ""
}

// isEvalSetting is what the eval takes from .env besides credentials: its own
// EVAL_* and KEYLINT_* switches. Anything else stays out of the test process —
// and so out of the Claude Code CLI's environment, which inherits it.
func isEvalSetting(key string) bool {
	return strings.HasPrefix(key, "EVAL_") || strings.HasPrefix(key, "KEYLINT_")
}

// cliVersionSpan is what a run records as the CLI version: the version read at
// its start, or "start->end" when the CLI changed while the run was going. The
// version is reported next to a verdict, not keyed: the CLI updates itself.
func cliVersionSpan(start, end string) string {
	if start == end {
		return start
	}
	return start + "->" + end
}
