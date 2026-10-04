package pyramidize

import (
	"strings"

	"keylint/internal/llm"
)

// Kept out of the eval-tagged files on purpose: which provider a run measures,
// and whether it needs a key at all, decide what a baseline means, and that has
// to be covered by the normal test suite rather than only by a paid run.

// defaultEvalProvider is what a run measures when nothing says otherwise. A
// constant rather than the machine's active provider: a baseline that changes
// with whoever runs it is not a baseline.
//
// The installed Claude Code CLI since 2026-10, so a run spends the owner's
// subscription rather than an API key. `--provider claude` (EVAL_PROVIDER=claude)
// still measures the API.
const defaultEvalProvider = llm.ProviderClaudeCode

// evalTarget resolves the provider and model a run measures. The model is
// passed to the pipeline as an explicit override, so a run does not depend on
// whichever model the developer picked in the GUI — and for the CLI it is an
// alias ("sonnet"), so summary.json also records what it resolved to.
func evalTarget(getenv func(string) string) (provider, model string) {
	provider = strings.TrimSpace(getenv("EVAL_PROVIDER"))
	if provider == "" {
		provider = defaultEvalProvider
	}
	model = strings.TrimSpace(getenv("EVAL_MODEL"))
	if model == "" {
		model = llm.DefaultModel(provider, llm.FeaturePyramidize)
	}
	return provider, model
}

// evalNeedsAPIKey reports whether any provider in a run authenticates with a
// key KeyLint supplies. When neither does, the eval loads no key from .env at
// all: the Claude Code client strips one from the CLI's environment anyway, but
// a key that is never read cannot be the one that answered.
func evalNeedsAPIKey(providers ...string) bool {
	for _, p := range providers {
		if llm.UsesAPIKey(p) {
			return true
		}
	}
	return false
}

// evalEnvFromFile decides which .env entries a run takes into its environment.
//
// Everything that is not a credential fills only what the caller left empty:
// .env is a place to keep secrets, not a second opinion on what to measure, and
// a blanket overload once let a line in the file beat `--model X` from the
// command line. Credentials win over the shell — but only when the run has an
// API provider in it, pipeline or judge. A run that goes entirely through the
// Claude Code CLI takes no key at all.
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
	// The providers are read through what .env just filled in, so EVAL_PROVIDER
	// kept in the file decides the question the same way it decides the run.
	merged := func(key string) string {
		if v, ok := set[key]; ok {
			return v
		}
		return getenv(key)
	}
	provider, _ := evalTarget(merged)
	if !evalNeedsAPIKey(provider, judgeConfigFrom(merged).Provider) {
		return set
	}
	for key, value := range file {
		if strings.HasSuffix(key, "_API_KEY") {
			set[key] = value
		}
	}
	return set
}
