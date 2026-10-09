package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// Gaps the review of the model-family PR named, one test each.

func TestAnsweringModelPicksTheOneThatWroteTheAnswer(t *testing.T) {
	env := claudeCodeEnvelope{ModelUsage: map[string]any{
		"claude-haiku-4-5-20251001": map[string]any{"outputTokens": 12.0},
		"claude-sonnet-5-5":         map[string]any{"outputTokens": 812.0},
	}}
	if got := answeringModel(env); got != "claude-sonnet-5-5" {
		t.Errorf("answeringModel = %q, want the model with the most output", got)
	}
	if got := answeringModel(claudeCodeEnvelope{}); got != "" {
		t.Errorf("answeringModel with no usage = %q, want empty", got)
	}
}

// TestAnUnresolvedAliasIsNeverRecordedBare: an older CLI reports no usage, so
// nothing names the model; the record must not then read "sonnet".
func TestAnUnresolvedAliasIsNeverRecordedBare(t *testing.T) {
	var r ModelRecorder
	if got := r.Recorded("sonnet"); got != "sonnet (unresolved)" {
		t.Errorf("Recorded = %q", got)
	}
	if got := r.Recorded("claude-sonnet-4-6"); got != "claude-sonnet-4-6" {
		t.Errorf("Recorded pinned = %q", got)
	}
}

func TestTypedAliasesAreNormalised(t *testing.T) {
	for in, want := range map[string]string{"Sonnet": "sonnet", " HAIKU ": "haiku", "claude-Sonnet-4-6": "claude-Sonnet-4-6"} {
		if got := NormalizeFamily(in); got != want {
			t.Errorf("NormalizeFamily(%q) = %q, want %q", in, got, want)
		}
	}
	f := newFakeAnthropic(t, liveListing)
	if _, err := f.client("k-typed").Complete(context.Background(), Request{Model: "Sonnet", User: "x", MaxTokens: 64}); err != nil {
		t.Fatal(err)
	}
	if got := f.last(t)["model"]; got != "claude-sonnet-5-5" {
		t.Errorf("typed Sonnet went out as %v", got)
	}
}

func TestRejectedThinkingIsRetriedWithout(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	f.reject = func(body map[string]any) string {
		if body["thinking"] != nil {
			return "thinking.type: disabled is not supported for this model"
		}
		return ""
	}
	// Not in the listing, so nothing says whether it can turn thinking off.
	if _, err := f.client("k-think-retry").Complete(context.Background(), Request{Model: "claude-haiku-9", User: "x", MaxTokens: 64, NoThinking: true}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(f.bodies) != 2 || f.bodies[1]["thinking"] != nil {
		t.Errorf("want one retry without thinking, got %v", f.bodies)
	}
}

// TestAFailedListingIsRememberedBriefly: a listing that failed is not asked
// again on the very next completion, and a cancelled caller is not taken as
// news about the endpoint.
func TestAFailedListingIsRememberedBriefly(t *testing.T) {
	f := newFakeAnthropic(t, "")
	complete := func() {
		if _, err := f.client("k-fail-ttl").Complete(context.Background(), Request{Model: FamilyHaiku, User: "x", MaxTokens: 64}); err != nil {
			t.Fatal(err)
		}
	}
	complete()
	// The SDK retries a 500 once, so the first listing is two requests.
	first := f.listed
	complete()
	if f.listed != first {
		t.Errorf("listed again (%d → %d) right after a failure, want it cached", first, f.listed)
	}

	g := newFakeAnthropic(t, liveListing)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = g.client("k-cancel").Complete(ctx, Request{Model: FamilyHaiku, User: "x", MaxTokens: 64})
	if _, err := g.client("k-cancel").Complete(context.Background(), Request{Model: FamilyHaiku, User: "x", MaxTokens: 64}); err != nil {
		t.Fatal(err)
	}
	if got := g.last(t)["model"]; got != "claude-haiku-5-5" {
		t.Errorf("after a cancelled call the alias resolved to %v — the cancellation was cached as a failed listing", got)
	}
}

func TestClaudeCodeDoesNotRetryAnUnrelatedFailure(t *testing.T) {
	s := newStub(t)
	s.fails("error: rate limited", "1")
	_, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x", Effort: EffortLow})
	if err == nil {
		t.Fatal("want the failure reported")
	}
	if !containsArg(s.argv(), "--effort") {
		t.Errorf("a run that failed for another reason was retried without --effort: %v", s.argv())
	}
}

// TestClaudeCodeIgnoresEffortInAnAnswer: a good run whose answer happens to
// say "effort" is not a rejection and must not cost a second run.
func TestClaudeCodeIgnoresEffortInAnAnswer(t *testing.T) {
	s := newStub(t)
	s.replies(`{"result":"It took real effort.","is_error":false}`)
	resp, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x", Effort: EffortLow})
	if err != nil || resp.Text != "It took real effort." {
		t.Fatalf("Complete = %q, %v", resp.Text, err)
	}
	if !containsArg(s.argv(), "--effort") {
		t.Error("the only run should have been the one with --effort")
	}
}

func TestOpenAIDoesNotRetryAnUnrelated400(t *testing.T) {
	calls := 0
	base := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"messages: too long","type":"invalid_request_error"}}`)
	})
	_, err := newOpenAI(Config{APIKey: "sk-test", BaseURL: base}).Complete(context.Background(), Request{Model: "gpt-6-luna", User: "x", NoThinking: true})
	if err == nil || calls != 1 {
		t.Errorf("err = %v, calls = %d — want one failed call", err, calls)
	}
}

// TestOllamaNeverGetsReasoningEffort: it shares the OpenAI dialect, but a model
// name that happens to match a GPT family must not carry the field there.
func TestOllamaNeverGetsReasoningEffort(t *testing.T) {
	var body map[string]any
	base := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
	})
	if _, err := newOllama(Config{BaseURL: base}).Complete(context.Background(), Request{Model: "gpt-6-luna", User: "x", NoThinking: true, Effort: EffortHigh}); err != nil {
		t.Fatal(err)
	}
	if body["reasoning_effort"] != nil {
		t.Errorf("Ollama was sent reasoning_effort = %v", body["reasoning_effort"])
	}
}
