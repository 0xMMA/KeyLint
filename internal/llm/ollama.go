package llm

import (
	"context"
	"strings"
)

// defaultOllamaBaseURL is where a local Ollama daemon listens.
const defaultOllamaBaseURL = "http://localhost:11434"

// ollamaProvider carries the ID used in logs and the name used in errors.
var ollamaProvider = provider{id: ProviderOllama, name: "Ollama"}

// ollamaAPIKey is a placeholder for a daemon that needs no credential — the
// OpenAI SDK sends an Authorization header unconditionally and Ollama ignores
// it. A real key (Ollama Cloud, or an auth proxy in front of the daemon) is
// kept if the caller supplied one.
const ollamaAPIKey = "ollama"

type ollamaClient struct {
	cfg Config
}

func newOllama(cfg Config) Client { return &ollamaClient{cfg: cfg} }

// Complete talks to Ollama's OpenAI-compatible endpoint under /v1 rather than
// its native /api/generate. That is what lets it share the OpenAI client — and
// it means the system prompt is a real system message now instead of being
// glued onto the front of the user text.
func (c *ollamaClient) Complete(ctx context.Context, req Request) (Response, error) {
	cfg := c.cfg
	if cfg.APIKey == "" {
		cfg.APIKey = ollamaAPIKey
	}
	// Unlike OpenAI, the SDK's own default host is wrong here, so the base URL
	// is always set — to the configured daemon or to localhost.
	base := resolveBaseURL(cfg.BaseURL, defaultOllamaBaseURL)
	// Ollama's own documentation gives the endpoint as ".../v1", so that is what
	// a user pastes into the settings field. The client appends /v1 itself, and
	// /v1/v1/chat/completions is a 404 nobody can diagnose from the message.
	base = strings.TrimSuffix(base, "/v1")
	return completeViaOpenAI(ctx, cfg, ollamaProvider, base, req)
}
