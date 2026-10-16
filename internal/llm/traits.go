package llm

import (
	"strconv"
	"strings"
)

// UsesAPIKey reports whether a provider authenticates with an API key that
// KeyLint supplies. The Claude Code CLI uses the account the user signed in
// with, and Ollama needs nothing; asking for a key for either would be wrong,
// and handing one to the CLI would bill a different account (see blockedCLIEnv).
//
// Bedrock depends on how the user connects it, so bedrockAuth is the mode from
// settings (providers.bedrock_auth): a key in key mode, none with this
// computer's AWS sign-in — whether that sign-in works can only be known by
// trying. Key mode is the default. Every other provider ignores bedrockAuth.
func UsesAPIKey(provider, bedrockAuth string) bool {
	switch provider {
	case ProviderClaude, ProviderOpenAI, ProviderOpenRouter:
		return true
	case ProviderBedrock:
		return bedrockAuth != BedrockAuthSignIn
	default:
		return false
	}
}

// SupportsTemperature reports whether a request to this provider and model
// may carry Request.Temperature.
//
// The Claude Code CLI has no flag for it, so a run through the CLI samples at
// the CLI's default. On the Anthropic API, Claude models up to the 4.6
// generation take one; from 4.7 on they reject a non-default temperature
// (Sonnet 5.5 and Haiku 5.5) or have no sampling parameters at all (Opus
// 4.7/4.8, Sonnet 5, Opus 5.x, Fable). A family alias resolves to the newest
// generation, and an ID this cannot read is treated as new: an eval must record
// "no pin" rather than claim one that never reached the model, or fail the
// call with a 400. Other providers are assumed to take one.
//
// OpenRouter falls under "assumed", and that is a guess nobody has checked: it
// passes temperature on to whichever host serves the model, so an
// anthropic/claude-*-5.5 there is presumably refused as it is on the Anthropic
// API. It only matters to evals, and OpenRouter is not eval'd until measured
// (roadmap E10) — whoever measures it decides this from a real call.
func SupportsTemperature(provider, model string) bool {
	switch provider {
	case ProviderClaudeCode:
		return false
	case ProviderClaude:
		major, minor, ok := claudeGeneration(model)
		return ok && (major < 4 || (major == 4 && minor <= 6))
	case ProviderBedrock:
		// The same models under Bedrock's IDs; an ARN says nothing, so it
		// counts as new.
		_, base := splitBedrockPrefix(model)
		major, minor, ok := claudeGeneration(strings.TrimPrefix(base, "anthropic."))
		return ok && (major < 4 || (major == 4 && minor <= 6))
	default:
		return true
	}
}

// claudeGeneration reads the version out of a Claude model ID:
// claude-sonnet-4-5-20250929 is 4.5, claude-sonnet-5 is 5.0, and the old
// claude-3-5-sonnet-20241022 naming is 3.5.
func claudeGeneration(model string) (major, minor int, ok bool) {
	parts := strings.Split(strings.TrimPrefix(model, "claude-"), "-")
	if len(parts) == 0 || parts[0] == model {
		return 0, 0, false
	}
	// Old naming puts the version first; new naming puts the family first.
	start := 1
	if isSmallNumber(parts[0]) {
		start = 0
	}
	if len(parts) <= start || !isSmallNumber(parts[start]) {
		return 0, 0, false
	}
	major, _ = strconv.Atoi(parts[start])
	if len(parts) > start+1 && isSmallNumber(parts[start+1]) {
		minor, _ = strconv.Atoi(parts[start+1])
	}
	return major, minor, true
}

// isSmallNumber is a version component (1–2 digits), not an 8-digit date.
func isSmallNumber(s string) bool {
	if len(s) == 0 || len(s) > 2 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
