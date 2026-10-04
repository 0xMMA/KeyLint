package llm

import (
	"context"
	"slices"
	"strings"
	"sync"
)

// ResolvedModels records which model IDs actually answered a set of calls.
//
// It exists for measurement. An eval asks for "sonnet" or "haiku" and the
// provider decides which generation that means today; a baseline that records
// only the alias cannot tell a prompt change from a silent model change. Wrap
// the client factory a run uses, and String reports what really answered.
//
// Safe for concurrent use. The zero value is ready.
type ResolvedModels struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// WrapFactory returns a client factory whose clients report every answering
// model to r. Its signature matches llm.New and the newClient seams in the
// feature services, so a run can swap it in without touching product code.
func (r *ResolvedModels) WrapFactory(next func(string, Config) (Client, error)) func(string, Config) (Client, error) {
	return func(provider string, cfg Config) (Client, error) {
		client, err := next(provider, cfg)
		if err != nil {
			return nil, err
		}
		return &recordingClient{next: client, rec: r}, nil
	}
}

// Record notes one response's model. Responses that do not name one are
// skipped: a provider that does not say is not evidence of anything.
func (r *ResolvedModels) Record(model string) {
	// A Claude Code response that used several models arrives comma-joined;
	// split it so the set holds IDs, not combinations.
	for _, id := range strings.Split(model, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		r.mu.Lock()
		if r.seen == nil {
			r.seen = map[string]struct{}{}
		}
		r.seen[id] = struct{}{}
		r.mu.Unlock()
	}
}

// String is every model ID seen, sorted and comma-joined, or "" when none was.
//
// One ID is the normal case. Two mean the run straddled a generation change or
// the provider mixed models — either way not the measurement a single-ID run
// is, and the joined string keeps it from comparing equal to one.
func (r *ResolvedModels) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.seen))
	for id := range r.seen {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return strings.Join(ids, ",")
}

type recordingClient struct {
	next Client
	rec  *ResolvedModels
}

func (c *recordingClient) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := c.next.Complete(ctx, req)
	if err == nil {
		c.rec.Record(resp.Model)
	}
	return resp, err
}

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
