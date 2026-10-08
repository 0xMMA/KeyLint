package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"keylint/internal/logger"
)

// Model families. As a model setting each one means "the newest model of this
// family the account can use", so a new generation reaches users without a
// KeyLint release (roadmap E2 step 5).
//
// The Claude Code CLI takes these names natively. The Anthropic API has no such
// alias, so this file resolves one there; OpenAI and Ollama keep explicit IDs.
const (
	FamilyOpus   = "opus"
	FamilySonnet = "sonnet"
	FamilyHaiku  = "haiku"
	FamilyFable  = "fable"
)

// modelFamilies in picker order.
var modelFamilies = []string{FamilyOpus, FamilySonnet, FamilyHaiku, FamilyFable}

// IsModelFamily reports whether m is a family alias rather than a model ID.
func IsModelFamily(m string) bool {
	return slices.Contains(modelFamilies, m)
}

// familyLabel is how the picker names an alias: what it follows, not which
// model that is today. The resolved ID travels separately in ModelInfo.Resolved.
func familyLabel(family string) string {
	return strings.ToUpper(family[:1]) + family[1:] + " (latest)"
}

// familyPrefix is what every model ID of a family starts with on the Anthropic
// API: claude-sonnet-5-5, claude-haiku-4-5-20251001, …
func familyPrefix(family string) string {
	return "claude-" + family + "-"
}

// familyFallbackOnly backs an alias that the built-in picker list does not
// offer. Fable is only offered where the account's live list carries it, so it
// stays out of curatedModels; but a user who chose it while it was listed and
// whose listing is now unreachable must still get a model ID, not an error.
var familyFallbackOnly = []ModelInfo{
	{ID: "claude-fable-5-1", Label: "Fable 5.1"},
}

// anthropicModel is one entry of the account's model listing, with the parts
// KeyLint acts on: when it appeared, and which effort levels it accepts.
type anthropicModel struct {
	ID      string
	Label   string
	Created time.Time
	// effortKnown is false when the listing did not report capabilities at
	// all — an older API version or a proxy. Effort is then sent as asked and a
	// rejection is handled at call time; see anthropicClient.Complete.
	effortKnown bool
	effort      map[string]bool
	// thinkingOffKnown / thinkingOff say whether the model accepts
	// thinking: {type: "disabled"}. Haiku 5.5 does, Sonnet 5.5 and Opus 5.5
	// answer a 400. The SDK version in go.mod has no field for it yet, so it is
	// read from the raw capability object.
	thinkingOffKnown bool
	thinkingOff      bool
}

// acceptsEffort reports whether the model takes this level. Unknown capability
// counts as yes: refusing to send would silently ignore the user's setting on
// every model the listing failed to describe.
func (m anthropicModel) acceptsEffort(level string) bool {
	if !m.effortKnown {
		return true
	}
	return m.effort[level]
}

// acceptsThinkingOff reports whether the model can be asked not to reason.
// Unknown counts as yes, for the same reason as acceptsEffort; a 400 is then
// handled at call time.
func (m anthropicModel) acceptsThinkingOff() bool {
	return !m.thinkingOffKnown || m.thinkingOff
}

// newestOfFamily picks the most recently created model of a family. Ties go to
// the lexically greater ID, so the answer does not depend on listing order.
func newestOfFamily(models []anthropicModel, family string) (anthropicModel, bool) {
	prefix := familyPrefix(family)
	var best anthropicModel
	found := false
	for _, m := range models {
		if !strings.HasPrefix(m.ID, prefix) {
			continue
		}
		if !found || m.Created.After(best.Created) || (m.Created.Equal(best.Created) && m.ID > best.ID) {
			best, found = m, true
		}
	}
	return best, found
}

// curatedNewestOfFamily is the fallback when the listing cannot be had: the
// first entry of the family in the built-in list, which is kept newest-first
// (a test holds it there).
func curatedNewestOfFamily(family string) (string, bool) {
	prefix := familyPrefix(family)
	for _, list := range [][]ModelInfo{curatedModels[ProviderClaude], familyFallbackOnly} {
		for _, m := range list {
			if strings.HasPrefix(m.ID, prefix) {
				return m.ID, true
			}
		}
	}
	return "", false
}

// Catalogue cache. The listing is what an alias resolves against and what says
// which effort levels a model takes, so a completion may need it — and a
// completion is a hotkey press, which must not pay a listing round trip every
// time. Same lifetimes as the settings screen's model list: a settled answer
// for ten minutes, a failure for thirty seconds.
const (
	catalogueTTL        = 10 * time.Minute
	catalogueFailureTTL = 30 * time.Second
	// catalogueTimeout bounds the listing inside a completion, so a slow
	// models endpoint cannot eat the completion's own budget. On timeout the
	// built-in list stands in.
	catalogueTimeout = 10 * time.Second
)

type cachedCatalogue struct {
	models  []anthropicModel
	ok      bool
	fetched time.Time
}

var catalogueCache = struct {
	sync.Mutex
	entries map[string]cachedCatalogue
}{entries: map[string]cachedCatalogue{}}

// catalogueKey separates accounts and endpoints. The key is hashed so the raw
// credential is not held a second time in a long-lived map.
func catalogueKey(cfg Config) string {
	sum := sha256.Sum256([]byte(cfg.APIKey))
	return resolveBaseURL(cfg.BaseURL, "") + "\x00" + hex.EncodeToString(sum[:])
}

func storeCatalogue(cfg Config, models []anthropicModel, ok bool) {
	catalogueCache.Lock()
	catalogueCache.entries[catalogueKey(cfg)] = cachedCatalogue{models: models, ok: ok, fetched: time.Now()}
	catalogueCache.Unlock()
}

// anthropicCatalogue returns the account's models, from cache when fresh. ok is
// false when the listing could not be had; the caller falls back.
func anthropicCatalogue(ctx context.Context, cfg Config) (models []anthropicModel, ok bool) {
	key := catalogueKey(cfg)
	catalogueCache.Lock()
	cached, found := catalogueCache.entries[key]
	catalogueCache.Unlock()
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
	models, err := fetchAnthropicCatalogue(listCtx, cfg)
	if err != nil {
		// The caller's own cancellation is not a fact about the endpoint, so
		// it must not be remembered as one.
		if ctx.Err() == nil {
			storeCatalogue(cfg, nil, false)
		}
		logger.Info("llm: model listing unavailable, using the built-in list", "feature", cfg.Feature, "err", err)
		return nil, false
	}
	storeCatalogue(cfg, models, true)
	return models, true
}

// fetchAnthropicCatalogue asks the models endpoint once.
func fetchAnthropicCatalogue(ctx context.Context, cfg Config) ([]anthropicModel, error) {
	attempts := &httpAttempts{cfg: cfg, provider: anthropicProvider}
	client := (&anthropicClient{cfg: cfg}).client(attempts)

	// Without a limit the SDK asks for 20 and this code never follows the
	// cursor; a workspace with more would silently lose the oldest entries,
	// which are exactly the pinned IDs people configure.
	page, err := client.Models.List(ctx, anthropic.ModelListParams{Limit: anthropic.Int(1000)})
	if err != nil {
		return nil, mapAnthropicError(attempts, "", err)
	}
	models := make([]anthropicModel, 0, len(page.Data))
	for _, entry := range page.Data {
		label := entry.DisplayName
		if label == "" {
			label = entry.ID
		}
		m := anthropicModel{ID: entry.ID, Label: label, Created: entry.CreatedAt}
		effort := entry.Capabilities.Effort
		if effort.JSON.Supported.Valid() {
			m.effortKnown = true
			m.effort = map[string]bool{
				EffortLow:    effort.Supported && effort.Low.Supported,
				EffortMedium: effort.Supported && effort.Medium.Supported,
				EffortHigh:   effort.Supported && effort.High.Supported,
				EffortXHigh:  effort.Supported && effort.Xhigh.Supported,
				EffortMax:    effort.Supported && effort.Max.Supported,
			}
		}
		var types map[string]struct {
			Supported bool `json:"supported"`
		}
		if raw := entry.Capabilities.Thinking.Types.RawJSON(); raw != "" && json.Unmarshal([]byte(raw), &types) == nil {
			if disabled, ok := types["disabled"]; ok {
				m.thinkingOffKnown = true
				m.thinkingOff = disabled.Supported
			}
		}
		models = append(models, m)
	}
	return models, nil
}

// resolveAnthropicFamily turns an alias into a model ID: the newest of the
// family in the live listing, else the built-in list's newest. It always
// returns something for a known family, because a hotkey fix that fails over a
// listing hiccup would be worse than one that runs on last month's newest.
//
// The resolution is logged at Info on every call, so a support question —
// "which model did my fix use?" — has an answer in the log.
func resolveAnthropicFamily(cfg Config, family string, models []anthropicModel, live bool) string {
	if live {
		if m, ok := newestOfFamily(models, family); ok {
			logger.Info("llm: model alias resolved", "feature", cfg.Feature, "alias", family, "model", m.ID, "source", "live")
			return m.ID
		}
	}
	id, _ := curatedNewestOfFamily(family)
	source := "built-in"
	if live {
		// The account lists models but none of this family — Fable on an
		// account without access is the case. The built-in ID will most likely
		// be refused, and the error then names it.
		source = "built-in (family not in the account's listing)"
	}
	logger.Info("llm: model alias resolved", "feature", cfg.Feature, "alias", family, "model", id, "source", source)
	return id
}

// ResolveModel returns the model ID a request for model will actually use.
//
// Only an alias on the Anthropic API changes: every other provider either
// takes the name as is (the Claude Code CLI resolves aliases itself) or has no
// aliases. Measurement code calls this so a run records the resolved ID and
// never the alias — the eval's configKey names the model, and an alias there
// would hide a generation change as a prompt effect.
func ResolveModel(ctx context.Context, provider, model string, cfg Config) string {
	if provider != ProviderClaude || !IsModelFamily(model) {
		return model
	}
	models, live := anthropicCatalogue(ctx, cfg)
	return resolveAnthropicFamily(cfg, model, models, live)
}

// familyAliasesFor builds the alias entries that head the Anthropic picker:
// one per family the listing carries, each naming the model it resolves to
// today. A family the account does not list is not offered — Fable on an
// account without access would only fail at call time.
func familyAliasesFor(models []anthropicModel) []ModelInfo {
	var aliases []ModelInfo
	for _, family := range modelFamilies {
		if m, ok := newestOfFamily(models, family); ok {
			aliases = append(aliases, ModelInfo{ID: family, Label: familyLabel(family), Resolved: m.ID})
		}
	}
	return aliases
}

// IsFastTier reports whether a model is a provider's fast, cheap tier — the
// one a silent hotkey fix runs on for speed: a Haiku (alias or ID) or an
// OpenAI Luna. Fix uses it to ask such a model to answer without reasoning
// first; see Request.NoThinking.
func IsFastTier(model string) bool {
	return model == FamilyHaiku ||
		strings.HasPrefix(model, familyPrefix(FamilyHaiku)) ||
		strings.HasPrefix(model, "gpt-6-luna") ||
		strings.HasPrefix(model, "gpt-5.6-luna")
}
