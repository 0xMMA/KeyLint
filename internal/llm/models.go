package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// Features whose model is chosen separately. The silent fix wants a fast, cheap
// model; Pyramidize restructures a whole document and wants a stronger one.
const (
	FeatureFix        = "fix"
	FeaturePyramidize = "pyramidize"
)

// Where a model list came from, so a picker can say when it is showing the
// built-in list because the provider could not be reached.
const (
	// ModelSourceLive: the provider was asked and answered.
	ModelSourceLive = "live"
	// ModelSourceStatic: the provider could not be reached, so this is the
	// built-in list — something a user can act on by fixing the key or the URL.
	ModelSourceStatic = "static"
	// ModelSourceFixed: there is nothing to ask. The Claude Code CLI has no
	// model endpoint and its three aliases are the whole story, so flagging it
	// as "built-in" would be an alarm nobody can clear.
	ModelSourceFixed = "fixed"
)

// ModelInfo is one entry in a model picker.
type ModelInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ModelList is a picker's contents plus where they came from.
type ModelList struct {
	Models []ModelInfo `json:"models"`
	Source string      `json:"source"`
}

// defaultModels is what a feature uses when settings say nothing. These are the
// IDs KeyLint shipped before model selection existed — changing one is a
// quality decision that belongs with E3 (#34) and needs an eval run, not a
// refactor.
//
// The Anthropic fix model keeps its date suffix: the account's models endpoint
// lists claude-haiku-4-5-20251001 and no bare alias, so dropping it would be a
// guess about a model that is not advertised.
var defaultModels = map[string]map[string]string{
	ProviderClaude: {
		FeatureFix:        "claude-haiku-4-5-20251001",
		FeaturePyramidize: "claude-sonnet-4-6",
	},
	ProviderOpenAI: {
		FeatureFix:        "gpt-4o-mini",
		FeaturePyramidize: "gpt-5.2",
	},
	ProviderOllama: {
		FeatureFix:        "llama3.2",
		FeaturePyramidize: "llama3.2",
	},
	ProviderClaudeCode: {
		// Aliases, not pinned IDs: the CLI resolves them to the current
		// generation, which is what a subscription user expects.
		FeatureFix:        "haiku",
		FeaturePyramidize: "sonnet",
	},
}

// curatedModels is the fallback picker content when a provider cannot be
// listed — a local Ollama that is not running, an API key that is not set yet,
// or no network.
//
// The Anthropic entries were checked against the account's live models
// endpoint. The OpenAI entries are the ones the UI has been offering and could
// NOT be verified here: this machine's .env carries no OPENAI_API_KEY, so
// nothing called the models endpoint. Verify them before trusting this list.
var curatedModels = map[string][]ModelInfo{
	ProviderClaude: {
		{ID: "claude-opus-4-6", Label: "Opus 4.6"},
		{ID: "claude-sonnet-4-6", Label: "Sonnet 4.6"},
		{ID: "claude-haiku-4-5-20251001", Label: "Haiku 4.5"},
	},
	ProviderOpenAI: {
		{ID: "gpt-5.2", Label: "GPT-5.2"},
		{ID: "gpt-5.2-pro", Label: "GPT-5.2 Pro"},
		{ID: "gpt-4.1", Label: "GPT-4.1"},
		{ID: "gpt-4.1-mini", Label: "GPT-4.1 Mini"},
		{ID: "gpt-4o-mini", Label: "GPT-4o Mini"},
		{ID: "o3", Label: "o3"},
	},
	ProviderOllama: {
		{ID: "llama3.2", Label: "llama3.2"},
		{ID: "mistral", Label: "mistral"},
		{ID: "gemma3", Label: "gemma3"},
		{ID: "phi4", Label: "phi4"},
		{ID: "qwen2.5", Label: "qwen2.5"},
	},
	ProviderClaudeCode: claudeCodeAliases,
}

// claudeCodeAliases are the only values the CLI provider accepts. They are
// aliases by design: the CLI maps them to whatever the current generation is,
// and a pinned ID would defeat that.
var claudeCodeAliases = []ModelInfo{
	{ID: "opus", Label: "Opus"},
	{ID: "sonnet", Label: "Sonnet"},
	{ID: "haiku", Label: "Haiku"},
}

// DefaultModel returns the model a feature uses when nothing is configured.
func DefaultModel(provider, feature string) string {
	return defaultModels[provider][feature]
}

// CuratedModels is the built-in list for a provider, for when it cannot be
// asked. The source is always static.
func CuratedModels(provider string) ModelList {
	return ModelList{Models: curatedModels[provider], Source: ModelSourceStatic}
}

// IsClaudeCodeAlias reports whether m is one of the three aliases the picker
// offers. It is not a validity check: the CLI takes a full model ID too, and
// claudecode.go only notes the difference rather than refusing it. What the
// aliases buy is that they follow the generation, where a pinned ID freezes it
// — which is why the picker offers nothing else.
func IsClaudeCodeAlias(m string) bool {
	for _, alias := range claudeCodeAliases {
		if alias.ID == m {
			return true
		}
	}
	return false
}

// ListModels asks a provider what it can serve. On any failure it returns the
// curated list with source "static" and the error, so a caller can show the
// list and mention why it is the built-in one.
func ListModels(ctx context.Context, provider string, cfg Config) (ModelList, error) {
	var (
		models []ModelInfo
		err    error
	)
	switch provider {
	case ProviderClaude:
		models, err = listAnthropicModels(ctx, cfg)
	case ProviderOpenAI:
		models, err = listOpenAIModels(ctx, cfg)
	case ProviderOllama:
		models, err = listOllamaModels(ctx, cfg)
	case ProviderClaudeCode:
		// The CLI has no model endpoint, and these three are the whole story.
		return ModelList{Models: claudeCodeAliases, Source: ModelSourceFixed}, nil
	default:
		return CuratedModels(provider), fmt.Errorf("unsupported provider: %q", provider)
	}

	if err != nil || len(models) == 0 {
		return CuratedModels(provider), err
	}
	return ModelList{Models: models, Source: ModelSourceLive}, nil
}

func listAnthropicModels(ctx context.Context, cfg Config) ([]ModelInfo, error) {
	attempts := &httpAttempts{cfg: cfg, provider: anthropicProvider}
	client := (&anthropicClient{cfg: cfg}).client(attempts)

	// Without a limit the SDK asks for 20 and this code never follows the
	// cursor; a workspace with more would silently lose the oldest entries,
	// which are exactly the pinned IDs people configure.
	page, err := client.Models.List(ctx, anthropic.ModelListParams{Limit: anthropic.Int(1000)})
	if err != nil {
		return nil, mapAnthropicError(attempts, "", err)
	}
	var models []ModelInfo
	for _, entry := range page.Data {
		label := entry.DisplayName
		if label == "" {
			label = entry.ID
		}
		models = append(models, ModelInfo{ID: entry.ID, Label: label})
	}
	return models, nil
}

func listOpenAIModels(ctx context.Context, cfg Config) ([]ModelInfo, error) {
	attempts := &httpAttempts{cfg: cfg, provider: openAIProvider}
	client := openAISDKClient(cfg, resolveBaseURL(cfg.BaseURL, defaultOpenAIBaseURL), attempts)

	page, err := client.Models.List(ctx)
	if err != nil {
		return nil, mapOpenAIError(openAIProvider, attempts, "", err)
	}
	var models []ModelInfo
	for _, entry := range page.Data {
		if !isChatModel(entry.ID) {
			continue
		}
		models = append(models, ModelInfo{ID: entry.ID, Label: entry.ID})
	}
	return models, nil
}

// isChatModel filters an OpenAI account listing down to what this app can use.
// An account lists embeddings, speech, image and realtime models alongside the
// chat ones, and offering a model that 400s at call time is worse than omitting
// it. The prefix test alone is not enough: several gpt-* families are not chat
// models either.
func isChatModel(id string) bool {
	family := false
	for _, prefix := range []string{"gpt-", "o1", "o3", "o4", "chatgpt-", "codex-"} {
		if strings.HasPrefix(id, prefix) {
			family = true
			break
		}
	}
	if !family {
		return false
	}
	for _, marker := range []string{
		"-audio", "-realtime", "-transcribe", "-tts", "-image", "-search", "-instruct", "-moderation",
	} {
		if strings.Contains(id, marker) {
			return false
		}
	}
	return true
}

// listOllamaModels asks the daemon what is pulled locally. This is Ollama's
// native endpoint rather than the OpenAI-compatible one, which is why it is
// hand-rolled: /v1/models exists but reports less, and this is a listing call,
// not a completion.
func listOllamaModels(ctx context.Context, cfg Config) ([]ModelInfo, error) {
	base := strings.TrimSuffix(resolveBaseURL(cfg.BaseURL, defaultOllamaBaseURL), "/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/tags", nil)
	if err != nil {
		return nil, transportError(ollamaProvider, err)
	}
	// Same reason as every completion call: net/http would otherwise send
	// "Go-http-client/1.1", which some gateways in front of an Ollama host treat
	// as a bot. See fingerprintHeaders in llm.go.
	req.Header.Set("User-Agent", userAgent)

	resp, err := cfg.httpClient().Do(req)
	if err != nil {
		return nil, transportError(ollamaProvider, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(ollamaProvider, resp.StatusCode, "")
	}

	var payload struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s unexpected response listing models: %w", ollamaProvider.name, err)
	}

	var models []ModelInfo
	for _, entry := range payload.Models {
		id := entry.Model
		if id == "" {
			id = entry.Name
		}
		if id == "" {
			continue
		}
		models = append(models, ModelInfo{ID: id, Label: id})
	}
	return models, nil
}
