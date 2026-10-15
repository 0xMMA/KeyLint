package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"keylint/internal/logger"
)

// defaultOpenRouterBaseURL is OpenRouter's API host. openAISDKClient appends
// "/v1/", so chat completions go to https://openrouter.ai/api/v1/chat/completions
// and the catalogue is read from https://openrouter.ai/api/v1/models. Always
// passed explicitly, for the reason given at defaultOpenAIBaseURL.
const defaultOpenRouterBaseURL = "https://openrouter.ai/api"

// openRouterProvider carries the ID used in logs and the name used in errors.
var openRouterProvider = provider{id: ProviderOpenRouter, name: "OpenRouter"}

// OpenRouter speaks OpenAI's chat dialect, so a completion goes through the
// same SDK path as OpenAI and Ollama. Two things differ on the wire:
//
//   - Reasoning is the nested {"reasoning": {"effort": …}}, which OpenRouter
//     normalises across vendors; "none" turns reasoning off. OpenAI's top-level
//     reasoning_effort (openAIReasoningModels, keyed on bare OpenAI names) is
//     never sent here. Support varies per routed model, so what is sent is read
//     off that model's catalogue entry.
//   - max_tokens is sent: without it OpenRouter picks its own ceiling, and
//     Fix's 2048 is what keeps a hotkey fix short (and bounds the credit
//     OpenRouter reserves for the call).
//
// No app-attribution headers (HTTP-Referer, X-Title). OpenRouter uses them to
// rank apps; KeyLint strips machine fingerprint headers for the same reason it
// leaves these out — a provider gets what the call needs and nothing about
// who or what is making it. See fingerprintHeaders.
type openRouterClient struct {
	cfg Config
}

func newOpenRouter(cfg Config) Client { return &openRouterClient{cfg: cfg} }

func (c *openRouterClient) Complete(ctx context.Context, req Request) (Response, error) {
	if req.Model == "" {
		return Response{}, fmt.Errorf("%s: model is required", openRouterProvider.name)
	}
	effort := req.Effort
	if effort != "" && !IsEffortLevel(effort) {
		// A hand-edited settings file is the only way here; see the Anthropic
		// client.
		logger.Warn("llm: unknown effort level ignored", "feature", c.cfg.Feature, "effort", effort)
		effort = ""
	}

	// The catalogue is only needed to decide a reasoning option, so a call
	// with neither an effort nor NoThinking — Pyramidize on its default —
	// never asks for it. Fix on a fast-tier model does, and pays at most one
	// listing per cache lifetime.
	reasoning := ""
	if effort != "" || req.NoThinking {
		catalogue, live := openRouterCatalogue(ctx, c.cfg)
		entry, known := openRouterModel{}, false
		if live {
			entry, known = findOpenRouterModel(catalogue, req.Model)
		}
		reasoning = openRouterReasoning(effort, req.NoThinking, entry, known)
		if effort != "" && reasoning != effort {
			// Info, not Warn: the user chose a level and a model that does not
			// take it, and the request is still the right one to send.
			logger.Info("llm: effort not supported by this model, sent without it",
				"feature", c.cfg.Feature, "provider", openRouterProvider.id, "model", req.Model, "effort", effort)
		}
	}

	base := resolveBaseURL(c.cfg.BaseURL, defaultOpenRouterBaseURL)
	return completeViaOpenAI(ctx, c.cfg, openRouterProvider, base, req, chatExtras{reasoning: reasoning, maxTokens: req.MaxTokens})
}

// openRouterReasoning is the reasoning effort to send, or "" for none.
//
// known says whether the model's catalogue entry was found. When it was:
// an effort goes out only if the model exposes reasoning and lists that level
// (or lists none, which means every level); NoThinking becomes "none" only if
// the model reasons and may stop — "none" to a model whose reasoning is
// mandatory is refused. When it was not — no catalogue, or a model it does not
// list — a chosen effort is sent anyway (a refusal is retried without it, see
// completeViaOpenAI), and "none" is not: whether the model can stop reasoning
// is exactly what is unknown.
func openRouterReasoning(effort string, noThinking bool, entry openRouterModel, known bool) string {
	if !known {
		return effort
	}
	if entry.reasoning == nil {
		return ""
	}
	if effort != "" {
		if entry.reasoning.SupportedEfforts == nil || slices.Contains(*entry.reasoning.SupportedEfforts, effort) {
			return effort
		}
		return ""
	}
	if noThinking && !entry.reasoning.Mandatory {
		return "none"
	}
	return ""
}

// openRouterModel is one catalogue entry, with the parts KeyLint acts on.
type openRouterModel struct {
	ID      string
	Name    string
	outputs []string
	// reasoning is nil when the model exposes no effort selection.
	reasoning *openRouterReasoningInfo
}

// openRouterReasoningInfo is a catalogue entry's "reasoning" object.
type openRouterReasoningInfo struct {
	// Mandatory: the model always reasons, and refuses "none".
	Mandatory bool `json:"mandatory"`
	// SupportedEfforts nil (null or absent) means every level is accepted.
	SupportedEfforts *[]string `json:"supported_efforts"`
}

// openRouterChatModel reports whether a catalogue entry can answer the chat
// completion KeyLint makes.
//
// ":batch" IDs are OpenRouter's asynchronous Batch API — the same model,
// reachable only by submitting a batch job, not as a chat completion (77 of
// 463 entries on 2026-10-15). ":free" IDs are ordinary chat models and stay.
// A model that cannot output text cannot answer either; no such entry exists
// today, so this guards a future one. An entry that does not say what it
// outputs is kept.
func openRouterChatModel(id string, outputs []string) bool {
	if strings.HasSuffix(id, ":batch") {
		return false
	}
	return outputs == nil || slices.Contains(outputs, "text")
}

func findOpenRouterModel(models []openRouterModel, id string) (openRouterModel, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return openRouterModel{}, false
}

// maxOpenRouterCatalogueBytes bounds the read. The catalogue was 0.8 MB on
// 2026-10-15; anything near this limit is not the catalogue.
const maxOpenRouterCatalogueBytes = 32 << 20

// fetchOpenRouterCatalogue reads OpenRouter's public model catalogue once.
//
// Hand-rolled, as Ollama's tag listing is: the SDK's model type carries none of
// the reasoning fields, and this is a listing, not a completion. The
// catalogue needs no key, so none is sent — the key goes only where it is
// needed — and that is also why the picker can list it before a key is set.
func fetchOpenRouterCatalogue(ctx context.Context, cfg Config) ([]openRouterModel, error) {
	base := resolveBaseURL(cfg.BaseURL, defaultOpenRouterBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return nil, transportError(openRouterProvider, err)
	}
	// net/http would otherwise send "Go-http-client/1.1"; see fingerprintHeaders.
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := cfg.httpClient().Do(req)
	if err != nil {
		return nil, transportError(openRouterProvider, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(openRouterProvider, resp.StatusCode, "")
	}

	var payload struct {
		Data []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Architecture struct {
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
			Reasoning *openRouterReasoningInfo `json:"reasoning"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOpenRouterCatalogueBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s unexpected response listing models: %w", openRouterProvider.name, err)
	}
	models := make([]openRouterModel, 0, len(payload.Data))
	for _, entry := range payload.Data {
		if entry.ID == "" {
			continue
		}
		models = append(models, openRouterModel{
			ID:        entry.ID,
			Name:      entry.Name,
			outputs:   entry.Architecture.OutputModalities,
			reasoning: entry.Reasoning,
		})
	}
	return models, nil
}

// listOpenRouterModels is ListModels for OpenRouter: the whole catalogue less
// what a chat call cannot use, in OpenRouter's order, labelled with its names
// ("Anthropic: Claude Haiku 5.5"). IDs are explicit vendor/model; there is no
// family alias here (owner decision 2026-10-15).
func listOpenRouterModels(ctx context.Context, cfg Config) ([]ModelInfo, int, error) {
	catalogue, err := fetchOpenRouterCatalogue(ctx, cfg)
	if err != nil {
		return nil, 0, err
	}
	// The settings screen asking is also what a completion would ask for, so
	// the next hotkey press is spared the round trip.
	storeOpenRouterCatalogue(cfg, catalogue, true)

	var models []ModelInfo
	for _, entry := range catalogue {
		if !openRouterChatModel(entry.ID, entry.outputs) {
			continue
		}
		label := entry.Name
		if label == "" {
			label = entry.ID
		}
		models = append(models, ModelInfo{ID: entry.ID, Label: label})
	}
	return models, len(catalogue), nil
}

// The completion path's catalogue cache, on the Anthropic catalogue's pattern
// and lifetimes (catalogueTTL, catalogueFailureTTL): a hotkey press must not
// pay a 0.8 MB listing every time. Keyed by endpoint only — the catalogue is
// public and the same for every key.
type cachedOpenRouterCatalogue struct {
	models  []openRouterModel
	ok      bool
	fetched time.Time
}

var openRouterCatalogueCache = struct {
	sync.Mutex
	entries map[string]cachedOpenRouterCatalogue
}{entries: map[string]cachedOpenRouterCatalogue{}}

func openRouterCatalogueKey(cfg Config) string {
	return resolveBaseURL(cfg.BaseURL, defaultOpenRouterBaseURL)
}

func storeOpenRouterCatalogue(cfg Config, models []openRouterModel, ok bool) {
	openRouterCatalogueCache.Lock()
	openRouterCatalogueCache.entries[openRouterCatalogueKey(cfg)] = cachedOpenRouterCatalogue{models: models, ok: ok, fetched: time.Now()}
	openRouterCatalogueCache.Unlock()
}

// openRouterCatalogue returns the catalogue, from cache when fresh. ok is false
// when it could not be had; the caller then treats every model as unknown.
func openRouterCatalogue(ctx context.Context, cfg Config) (models []openRouterModel, ok bool) {
	key := openRouterCatalogueKey(cfg)
	openRouterCatalogueCache.Lock()
	cached, found := openRouterCatalogueCache.entries[key]
	openRouterCatalogueCache.Unlock()
	if found {
		ttl := catalogueFailureTTL
		if cached.ok {
			ttl = catalogueTTL
		}
		if time.Since(cached.fetched) < ttl {
			return cached.models, cached.ok
		}
	}

	listCtx, cancel := context.WithTimeout(ctx, catalogueTimeout)
	defer cancel()
	models, err := fetchOpenRouterCatalogue(listCtx, cfg)
	if err != nil {
		// The caller's own cancellation is not a fact about the endpoint.
		if ctx.Err() == nil {
			storeOpenRouterCatalogue(cfg, nil, false)
		}
		logger.Info("llm: OpenRouter catalogue unavailable, reasoning options decided without it",
			"feature", cfg.Feature, "err", err)
		return nil, false
	}
	storeOpenRouterCatalogue(cfg, models, true)
	return models, true
}
