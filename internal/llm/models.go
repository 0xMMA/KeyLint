package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// Features whose model is chosen separately. The silent fix wants a fast, cheap
// model; Pyramidize restructures a whole document and wants a stronger one.
const (
	FeatureFix        = "fix"
	FeaturePyramidize = "pyramidize"
)

// Where a model list came from. Each value is a different situation for the
// user, so the picker can say which one it is and the cache can decide how long
// to keep it. "Not live" alone is not enough: a daemon with nothing pulled, a
// daemon that is not running and a provider with no key yet need three
// different sentences and two different lifetimes.
const (
	// ModelSourceLive: the provider was asked and listed models.
	ModelSourceLive = "live"
	// ModelSourceEmpty: the provider answered and listed nothing. A fresh
	// `ollama serve` with nothing pulled is the case that matters. The built-in
	// list is deliberately NOT offered here — it would be a menu of models this
	// machine cannot serve, each of which 404s at call time.
	ModelSourceEmpty = "empty"
	// ModelSourceUnusable: the provider listed models and this app can call none
	// of them — an OpenAI project key scoped to Responses-API-only models is the
	// case. Kept apart from empty because the provider did list something, and
	// telling the user it listed nothing would be false.
	ModelSourceUnusable = "unusable"
	// ModelSourceUnreachable: the provider could not be asked at all — daemon
	// down, wrong URL, network gone. The built-in list stands in, and the user
	// has something to act on.
	ModelSourceUnreachable = "unreachable"
	// ModelSourceNoCredentials: there is no key to ask with, so nothing was
	// asked. The built-in list stands in. Kept apart from unreachable because
	// the fix is a different one and the wording has to say so.
	ModelSourceNoCredentials = "no-credentials"
	// ModelSourceFixed: there is nothing to ask. The Claude Code CLI has no
	// model endpoint and its three aliases are the whole story, so flagging it
	// as "built-in" would be an alarm nobody can clear.
	ModelSourceFixed = "fixed"
)

// ModelInfo is one entry in a model picker.
type ModelInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Resolved is the model ID a family alias stands for right now, where that
	// is known — the Anthropic API's live listing. Empty for a pinned ID, and
	// for an alias whose resolution happens elsewhere (the Claude Code CLI).
	Resolved string `json:"resolved,omitempty"`
}

// ModelList is a picker's contents plus where they came from.
type ModelList struct {
	Models []ModelInfo `json:"models"`
	Source string      `json:"source"`
}

// defaultModels is what a feature uses when settings say nothing.
//
// Anthropic defaults are family aliases, not versions (roadmap E2 step 5): a
// user gets the newest Haiku or Sonnet their account can use without waiting
// for a KeyLint release. The Claude Code CLI resolves an alias itself; for the
// API, families.go resolves it against the account's model listing.
//
// Moving the API defaults off claude-haiku-4-5-20251001 and claude-sonnet-4-6
// was accepted on the roadmap knowing what it means: today "haiku" is Haiku 5.5
// and "sonnet" is Sonnet 5.5, both of which reason before answering — slower
// than the pinned IDs, and the reasoning counts against the output limit. An
// explicit ID saved in settings is untouched by any of this.
//
// OpenAI and Ollama have no family aliases and keep explicit IDs.
var defaultModels = map[string]map[string]string{
	ProviderClaude: {
		FeatureFix:        FamilyHaiku,
		FeaturePyramidize: FamilySonnet,
	},
	// OpenAI's GPT-6 generation (2026-09): Luna is the fast, cheap tier and
	// Astra the flagship. Both reason; Fix asks Luna for reasoning_effort
	// "none" (see openAIReasoningModels), Astra cannot be asked that.
	ProviderOpenAI: {
		FeatureFix:        "gpt-6-luna",
		FeaturePyramidize: "gpt-6-astra",
	},
	ProviderOllama: {
		FeatureFix:        "llama3.2",
		FeaturePyramidize: "llama3.2",
	},
	ProviderClaudeCode: {
		FeatureFix:        FamilyHaiku,
		FeaturePyramidize: FamilySonnet,
	},
}

// curatedModels is the fallback picker content when a provider cannot be
// listed — a local Ollama that is not running, an API key that is not set yet,
// or no network.
//
// The Anthropic entries were checked against the account's live models
// endpoint on 2026-10-08. They are kept newest-first within each family: an
// alias that cannot be resolved live falls back to the first entry of its
// family here (curatedNewestOfFamily). The three aliases lead, because they are
// what most users want; Fable is not among them since not every account has
// it — see familyAliasesFor. The pinned IDs below them are for anyone who wants
// a model that does not change under them, Haiku 4.5 in particular: it is the
// one current model that answers without reasoning first.
//
// The OpenAI entries come from OpenAI's model pages
// (developers.openai.com/api/docs/models/<id>, read 2026-10-08), each of which
// lists Chat Completions as a supported endpoint. They could NOT be called from
// here: this machine's .env carries no OPENAI_API_KEY. Verify them against a
// live listing before trusting this list.
var curatedModels = map[string][]ModelInfo{
	ProviderClaude: {
		{ID: FamilyOpus, Label: familyLabel(FamilyOpus)},
		{ID: FamilySonnet, Label: familyLabel(FamilySonnet)},
		{ID: FamilyHaiku, Label: familyLabel(FamilyHaiku)},
		{ID: "claude-opus-5-5", Label: "Claude Opus 5.5"},
		{ID: "claude-sonnet-5-5", Label: "Claude Sonnet 5.5"},
		{ID: "claude-sonnet-4-6", Label: "Claude Sonnet 4.6"},
		{ID: "claude-haiku-5-5", Label: "Claude Haiku 5.5"},
		{ID: "claude-haiku-4-5-20251001", Label: "Claude Haiku 4.5"},
	},
	ProviderOpenAI: {
		{ID: "gpt-6-astra", Label: "GPT-6 Astra"},
		{ID: "gpt-6.1-sol", Label: "GPT-6.1 Sol"},
		{ID: "gpt-6-luna", Label: "GPT-6 Luna"},
		{ID: "gpt-5.6-terra", Label: "GPT-5.6 Terra"},
		{ID: "gpt-5.2", Label: "GPT-5.2"},
		// No -pro entry: those are Responses-API only and would 400 at the
		// /chat/completions call this app makes. See isChatModel. No o3 either:
		// its only snapshot shuts down on 2026-12-11.
		{ID: "gpt-4.1", Label: "GPT-4.1"},
		{ID: "gpt-4o-mini", Label: "GPT-4o Mini"},
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

// claudeCodeAliases are what the CLI picker offers. They are aliases by
// design: the CLI maps them to whatever the current generation is, and a
// pinned ID would defeat that.
//
// Fable is included because the CLI documents it as an alias (`claude --help`,
// 2.1.295: "Provide an alias for the latest model (e.g. 'fable', 'opus', or
// 'sonnet')") and one print-mode call on 2026-10-08 with `--model fable`
// answered from claude-fable-5-1. The CLI has no model listing, so whether a
// given subscription includes it cannot be checked up front; one that does not
// gets the CLI's own error at call time.
var claudeCodeAliases = []ModelInfo{
	{ID: FamilyOpus, Label: familyLabel(FamilyOpus)},
	{ID: FamilySonnet, Label: familyLabel(FamilySonnet)},
	{ID: FamilyHaiku, Label: familyLabel(FamilyHaiku)},
	{ID: FamilyFable, Label: familyLabel(FamilyFable)},
}

// DefaultModel returns the model a feature uses when nothing is configured.
func DefaultModel(provider, feature string) string {
	return defaultModels[provider][feature]
}

// CuratedModels is the built-in list for a provider, for when it cannot be
// asked. The caller passes the reason, because that is what the picker has to
// say and what decides how long the list is worth keeping.
//
// The slice is copied: the package-level list would otherwise be handed out for
// a caller to sort or append to.
func CuratedModels(provider, source string) ModelList {
	return ModelList{Models: slices.Clone(curatedModels[provider]), Source: source}
}

// ClaudeCodeAliases lists the alias IDs, for messages that name them. Derived
// from the same list the picker uses, so the two cannot say different things.
func ClaudeCodeAliases() []string {
	ids := make([]string, 0, len(claudeCodeAliases))
	for _, alias := range claudeCodeAliases {
		ids = append(ids, alias.ID)
	}
	return ids
}

// IsClaudeCodeAlias reports whether m is one of the aliases the picker
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

// ListModels asks a provider what it can serve.
//
// It never fails the caller: a provider that cannot be reached yields the
// built-in list, and one that lists nothing yields an empty list. The Source
// says which of those happened, so the picker can explain itself and the cache
// can decide how long to trust the answer. The error is returned alongside for
// logging, not as a reason to show nothing.
func ListModels(ctx context.Context, provider string, cfg Config) (ModelList, error) {
	var (
		models []ModelInfo
		listed int
		err    error
	)
	switch provider {
	case ProviderClaude:
		models, listed, err = listAnthropicModels(ctx, cfg)
	case ProviderOpenAI:
		models, listed, err = listOpenAIModels(ctx, cfg)
	case ProviderOllama:
		models, listed, err = listOllamaModels(ctx, cfg)
	case ProviderClaudeCode:
		// The CLI has no model endpoint, and these three are the whole story.
		return ModelList{Models: slices.Clone(claudeCodeAliases), Source: ModelSourceFixed}, nil
	default:
		return CuratedModels(provider, ModelSourceUnreachable), fmt.Errorf("unsupported provider: %q", provider)
	}

	if err != nil {
		return CuratedModels(provider, ModelSourceUnreachable), err
	}
	// Answered, but with nothing. Not a failure — there is no error to report
	// and nothing for the user to fix except pulling a model — so it must not
	// be folded into the unreachable case, which would both claim the provider
	// is down and offer models it cannot serve.
	if listed == 0 {
		return ModelList{Source: ModelSourceEmpty}, nil
	}
	// The provider listed models and the filter kept none of them. Saying "this
	// account lists no models" here would be false, and leaving the picker empty
	// would leave the user with nothing to choose, so the built-in list stands
	// in — the same trade as the unreachable case, for a different reason.
	if len(models) == 0 {
		return CuratedModels(provider, ModelSourceUnusable), nil
	}
	return ModelList{Models: models, Source: ModelSourceLive}, nil
}

// The listers return the models this app can use and, separately, how many the
// provider actually listed. The two differ only for OpenAI, whose account
// listing carries far more than chat models — and the difference matters:
// "listed nothing" and "listed nothing we can call" are different sentences.
func listAnthropicModels(ctx context.Context, cfg Config) ([]ModelInfo, int, error) {
	catalogue, err := fetchAnthropicCatalogue(ctx, cfg)
	if err != nil {
		return nil, 0, err
	}
	// A fresh listing is also what a completion resolves aliases against, so
	// the settings screen asking for it saves the next hotkey press a round
	// trip.
	storeCatalogue(cfg, catalogue, true)

	// The aliases lead, each naming what it resolves to today; the pinned IDs
	// follow in the provider's order.
	models := familyAliasesFor(catalogue)
	for _, entry := range catalogue {
		models = append(models, ModelInfo{ID: entry.ID, Label: entry.Label})
	}
	return models, len(catalogue), nil
}

func listOpenAIModels(ctx context.Context, cfg Config) ([]ModelInfo, int, error) {
	attempts := &httpAttempts{cfg: cfg, provider: openAIProvider}
	client := openAISDKClient(cfg, resolveBaseURL(cfg.BaseURL, defaultOpenAIBaseURL), attempts)

	page, err := client.Models.List(ctx)
	if err != nil {
		return nil, 0, mapOpenAIError(openAIProvider, attempts, "", err)
	}
	var models []ModelInfo
	for _, entry := range page.Data {
		if !isChatModel(entry.ID) {
			continue
		}
		models = append(models, ModelInfo{ID: entry.ID, Label: entry.ID})
	}
	return models, len(page.Data), nil
}

// isChatModel filters an OpenAI account listing down to what this app can use.
//
// KeyLint calls /chat/completions. An account lists far more than that —
// embeddings, speech, image and realtime models — and it also lists models that
// are reachable only through the Responses API. Both groups 400 at call time,
// which is worse than never offering them, so both are filtered out here.
//
// Unverified against a live account: there is no OPENAI_API_KEY in this
// project's environment, so the families below come from OpenAI's
// documentation rather than from a listing this code has seen. Anyone with a
// key should check it — a wrongly excluded family is a model a user cannot
// pick, which is quieter than a 400 but not harmless.
func isChatModel(id string) bool {
	family := false
	for _, prefix := range []string{"gpt-", "o1", "o3", "o4", "chatgpt-"} {
		if strings.HasPrefix(id, prefix) {
			family = true
			break
		}
	}
	if !family {
		return false
	}
	for _, marker := range []string{
		"-audio", "-realtime", "-transcribe", "-tts", "-image", "-instruct", "-moderation",
		// "-search" may be too broad: gpt-4o-search-preview does answer
		// /chat/completions, it just always searches. Pinned as an exclusion
		// because a model that silently web-searches is not what a grammar fix
		// or a restructure asks for — unverified against a live account.
		"-search",
		// Responses-API only: reachable, but not at /chat/completions. "-codex"
		// is a marker rather than a prefix because the family moved into the
		// gpt-* namespace (gpt-5.1-codex-max), where a prefix test cannot see it.
		"-pro", "-deep-research", "-codex",
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
func listOllamaModels(ctx context.Context, cfg Config) ([]ModelInfo, int, error) {
	base := strings.TrimSuffix(resolveBaseURL(cfg.BaseURL, defaultOllamaBaseURL), "/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/tags", nil)
	if err != nil {
		return nil, 0, transportError(ollamaProvider, err)
	}
	// Same reason as every completion call: net/http would otherwise send
	// "Go-http-client/1.1", which some gateways in front of an Ollama host treat
	// as a bot. See fingerprintHeaders in llm.go.
	req.Header.Set("User-Agent", userAgent)

	resp, err := cfg.httpClient().Do(req)
	if err != nil {
		return nil, 0, transportError(ollamaProvider, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, apiError(ollamaProvider, resp.StatusCode, "")
	}

	var payload struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, fmt.Errorf("%s unexpected response listing models: %w", ollamaProvider.name, err)
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
	return models, len(models), nil
}
