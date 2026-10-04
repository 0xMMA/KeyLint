package llm

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
)

// fakeModelClient answers with a fixed model ID, or fails.
type fakeModelClient struct {
	model string
	err   error
}

func (f fakeModelClient) Complete(context.Context, Request) (Response, error) {
	if f.err != nil {
		return Response{}, f.err
	}
	return Response{Text: "ok", Model: f.model}, nil
}

func factoryAnswering(model string, err error) func(string, Config) (Client, error) {
	return func(string, Config) (Client, error) { return fakeModelClient{model: model, err: err}, nil }
}

// TestResolvedModelsRecordsWhatAnswered: the alias that was asked for is not
// the measurement; the ID that answered is.
func TestResolvedModelsRecordsWhatAnswered(t *testing.T) {
	var rec ResolvedModels
	newClient := rec.WrapFactory(factoryAnswering("claude-sonnet-4-6", nil))

	client, err := newClient(ProviderClaudeCode, Config{})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := client.Complete(context.Background(), Request{Model: "sonnet"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := rec.String(); got != "claude-sonnet-4-6" {
		t.Errorf("String() = %q, want the one ID that answered, once", got)
	}
}

// TestResolvedModelsKeepsAMixedRunApart: a run that straddled a generation
// change must not read as either generation alone.
func TestResolvedModelsKeepsAMixedRunApart(t *testing.T) {
	var rec ResolvedModels
	rec.Record("claude-sonnet-4-6")
	rec.Record("claude-sonnet-5-0")
	rec.Record("claude-sonnet-4-6")
	// A Claude Code envelope that names two models arrives joined.
	rec.Record("claude-haiku-4-5, claude-sonnet-4-6")

	if got, want := rec.String(), "claude-haiku-4-5,claude-sonnet-4-6,claude-sonnet-5-0"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// TestResolvedModelsIgnoresFailuresAndSilence: a failed call answered nothing,
// and a provider that does not name its model is not evidence of one.
func TestResolvedModelsIgnoresFailuresAndSilence(t *testing.T) {
	var rec ResolvedModels
	failing, _ := rec.WrapFactory(factoryAnswering("claude-opus-9", errors.New("boom")))(ProviderClaude, Config{})
	silent, _ := rec.WrapFactory(factoryAnswering("", nil))(ProviderOllama, Config{})

	if _, err := failing.Complete(context.Background(), Request{}); err == nil {
		t.Fatal("the failing client did not fail")
	}
	if _, err := silent.Complete(context.Background(), Request{}); err != nil {
		t.Fatal(err)
	}
	if got := rec.String(); got != "" {
		t.Errorf("String() = %q, want empty", got)
	}
}

func TestResolvedModelsFactoryErrorPassesThrough(t *testing.T) {
	var rec ResolvedModels
	want := errors.New("unsupported")
	_, err := rec.WrapFactory(func(string, Config) (Client, error) { return nil, want })("nope", Config{})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want the factory's own", err)
	}
}

func TestResolvedModelsIsSafeForConcurrentUse(t *testing.T) {
	var rec ResolvedModels
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				rec.Record("a")
			} else {
				rec.Record("b")
			}
			_ = rec.String()
		}()
	}
	wg.Wait()
	if got := rec.String(); got != "a,b" {
		t.Errorf("String() = %q, want a,b", got)
	}
}

func TestUsesAPIKey(t *testing.T) {
	for provider, want := range map[string]bool{
		ProviderClaude:     true,
		ProviderOpenAI:     true,
		ProviderClaudeCode: false,
		ProviderOllama:     false,
		"":                 false,
	} {
		if got := UsesAPIKey(provider); got != want {
			t.Errorf("UsesAPIKey(%q) = %v, want %v", provider, got, want)
		}
	}
}

// The HTTP providers report the model in their response body; Response.Model
// must carry it through rather than echoing the request.
func TestAnthropicReportsTheModelThatAnswered(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK,
		`{"model":"claude-sonnet-4-6-20260101","content":[{"type":"text","text":"ok"}]}`)
	resp, err := newAnthropic(Config{APIKey: "k", BaseURL: srv.URL}).Complete(context.Background(),
		Request{User: "x", Model: "claude-sonnet-4-6", MaxTokens: 10})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Model != "claude-sonnet-4-6-20260101" {
		t.Errorf("Model = %q, want the ID from the response body", resp.Model)
	}
}

func TestOpenAIReportsTheModelThatAnswered(t *testing.T) {
	var got capture
	srv := newServer(t, &got, http.StatusOK,
		`{"model":"gpt-4o-mini-2024-07-18","choices":[{"message":{"content":"ok"}}]}`)
	resp, err := newOpenAI(Config{APIKey: "k", BaseURL: srv.URL}).Complete(context.Background(),
		Request{User: "x", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Model != "gpt-4o-mini-2024-07-18" {
		t.Errorf("Model = %q, want the ID from the response body", resp.Model)
	}
}

func TestSupportsTemperature(t *testing.T) {
	if SupportsTemperature(ProviderClaudeCode) {
		t.Error("the Claude Code CLI has no temperature flag; claiming otherwise records a pin that never happened")
	}
	for _, p := range []string{ProviderClaude, ProviderOpenAI, ProviderOllama} {
		if !SupportsTemperature(p) {
			t.Errorf("SupportsTemperature(%q) = false, want true", p)
		}
	}
}
