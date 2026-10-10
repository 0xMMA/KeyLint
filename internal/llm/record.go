package llm

import (
	"context"
	"slices"
	"strings"
	"sync"
)

// ModelRecorder notes which models actually answered, for measurement code
// that must record a resolved model ID and never the alias it asked for.
//
// An eval resolves an Anthropic API alias up front (ResolveModel), but the
// Claude Code CLI resolves its aliases itself and only says afterwards, in its
// usage report — so the record has to come from the responses.
type ModelRecorder struct {
	mu   sync.Mutex
	seen []string
}

// Wrap returns a client constructor whose clients report to r.
func (r *ModelRecorder) Wrap(newClient func(string, Config) (Client, error)) func(string, Config) (Client, error) {
	if newClient == nil {
		newClient = New
	}
	return func(provider string, cfg Config) (Client, error) {
		client, err := newClient(provider, cfg)
		if err != nil {
			return nil, err
		}
		return recordingClient{inner: client, r: r}, nil
	}
}

// Recorded is what a run should record as its model: the one model that
// answered; several joined with "+" when the generation moved mid-run, which a
// comparison must then refuse; or requested when no response named a model.
//
// A requested alias is never recorded bare: "sonnet" in a configKey would
// compare cleanly across a generation change, which is the one thing the
// record exists to prevent. It is marked unresolved instead, so --compare
// reads "not comparable" rather than a prompt effect.
func (r *ModelRecorder) Recorded(requested string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.seen) == 0 {
		if IsModelFamily(requested) {
			return requested + " (unresolved)"
		}
		return requested
	}
	return strings.Join(r.seen, "+")
}

func (r *ModelRecorder) note(model string) {
	if model == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !slices.Contains(r.seen, model) {
		r.seen = append(r.seen, model)
		slices.Sort(r.seen)
	}
}

type recordingClient struct {
	inner Client
	r     *ModelRecorder
}

func (c recordingClient) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := c.inner.Complete(ctx, req)
	if err == nil {
		c.r.note(resp.Model)
	}
	return resp, err
}
