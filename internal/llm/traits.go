package llm

// UsesAPIKey reports whether a provider authenticates with an API key that
// KeyLint supplies. The Claude Code CLI uses the account the user signed in
// with, and Ollama needs nothing; asking for a key for either would be wrong,
// and handing one to the CLI would bill a different account (see blockedCLIEnv).
func UsesAPIKey(provider string) bool {
	switch provider {
	case ProviderClaude, ProviderOpenAI:
		return true
	default:
		return false
	}
}

// SupportsTemperature reports whether a provider honours Request.Temperature.
// The Claude Code CLI has no flag for it, so a run through the CLI samples at
// whatever the CLI's default is — and an eval must say so rather than record a
// pin that never reached the model.
func SupportsTemperature(provider string) bool {
	return provider != ProviderClaudeCode
}
