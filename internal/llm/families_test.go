package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// effortAll / effortNone are the capability objects the live models endpoint
// returns (shape checked against GET /v1/models on 2026-10-08).
const (
	effortAll  = `"effort":{"supported":true,"low":{"supported":true},"medium":{"supported":true},"high":{"supported":true},"xhigh":{"supported":true},"max":{"supported":true}}`
	effortNone = `"effort":{"supported":false,"low":{"supported":false},"medium":{"supported":false},"high":{"supported":false},"xhigh":{"supported":false},"max":{"supported":false}}`
	effortNoX  = `"effort":{"supported":true,"low":{"supported":true},"medium":{"supported":true},"high":{"supported":true},"xhigh":{"supported":false},"max":{"supported":true}}`
	thinkOff   = `"thinking":{"supported":true,"types":{"enabled":{"supported":false},"adaptive":{"supported":true},"disabled":{"supported":true}}}`
	thinkOn    = `"thinking":{"supported":true,"types":{"enabled":{"supported":false},"adaptive":{"supported":true},"disabled":{"supported":false}}}`
)

// liveListing is a models endpoint answer resembling this account's, newest
// generation listed out of date order on purpose: resolution must go by
// created_at, not by position.
const liveListing = `{"data":[
	{"type":"model","id":"claude-sonnet-4-6","display_name":"Claude Sonnet 4.6","created_at":"2026-02-17T00:00:00Z","capabilities":{` + effortNoX + `,` + thinkOff + `}},
	{"type":"model","id":"claude-haiku-5-5","display_name":"Claude Haiku 5.5","created_at":"2026-10-07T18:00:00Z","capabilities":{` + effortAll + `,` + thinkOff + `}},
	{"type":"model","id":"claude-sonnet-5-5","display_name":"Claude Sonnet 5.5","created_at":"2026-09-28T00:00:00Z","capabilities":{` + effortAll + `,` + thinkOn + `}},
	{"type":"model","id":"claude-opus-5-5","display_name":"Claude Opus 5.5","created_at":"2026-09-21T16:24:00Z","capabilities":{` + effortAll + `,` + thinkOn + `}},
	{"type":"model","id":"claude-haiku-4-5-20251001","display_name":"Claude Haiku 4.5","created_at":"2025-10-15T00:00:00Z","capabilities":{` + effortNone + `,` + thinkOff + `}}
],"has_more":false}`

const okMessage = `{"model":"%MODEL%","stop_reason":"end_turn","content":[{"type":"text","text":"fixed"}]}`

// fakeAnthropic serves both the models listing and the messages endpoint and
// records every message body it was sent.
type fakeAnthropic struct {
	mu       sync.Mutex
	listing  string // "" answers the listing with a 500
	listed   int
	bodies   []map[string]any
	reject   func(body map[string]any) string // non-empty: answer 400 with this message
	baseURL  string
	modelOut string
}

func newFakeAnthropic(t *testing.T, listing string) *fakeAnthropic {
	f := &fakeAnthropic{listing: listing}
	f.baseURL = newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			f.listed++
			if f.listing == "" {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"type":"error","error":{"type":"api_error","message":"down"}}`)
				return
			}
			_, _ = io.WriteString(w, f.listing)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.bodies = append(f.bodies, body)
		if f.reject != nil {
			if msg := f.reject(body); msg != "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"`+msg+`"}}`)
				return
			}
		}
		model, _ := body["model"].(string)
		_, _ = io.WriteString(w, strings.ReplaceAll(okMessage, "%MODEL%", model))
	})
	return f
}

func (f *fakeAnthropic) client(apiKey string) Client {
	// A key per test keeps the package-level catalogue cache from carrying an
	// answer from one test into the next; the base URL differs anyway.
	return newAnthropic(Config{APIKey: apiKey, BaseURL: f.baseURL})
}

func (f *fakeAnthropic) last(t *testing.T) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.bodies) == 0 {
		t.Fatal("no message request reached the server")
	}
	return f.bodies[len(f.bodies)-1]
}

func outputConfig(body map[string]any) map[string]any {
	oc, _ := body["output_config"].(map[string]any)
	return oc
}

// TestAliasResolvesToTheNewestOfItsFamily: the alias goes out as the newest
// model of the family by created_at, and the response names it — which is what
// an eval records.
func TestAliasResolvesToTheNewestOfItsFamily(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	for alias, want := range map[string]string{
		FamilyHaiku:  "claude-haiku-5-5",
		FamilySonnet: "claude-sonnet-5-5",
		FamilyOpus:   "claude-opus-5-5",
	} {
		resp, err := f.client("k-newest").Complete(context.Background(), Request{Model: alias, User: "x", MaxTokens: 64})
		if err != nil {
			t.Fatalf("%s: %v", alias, err)
		}
		if got := f.last(t)["model"]; got != want {
			t.Errorf("%s went out as %v, want %s", alias, got, want)
		}
		if resp.Model != want {
			t.Errorf("%s: Response.Model = %q, want %q", alias, resp.Model, want)
		}
	}
	// One listing for three completions: the catalogue is cached.
	if f.listed != 1 {
		t.Errorf("listed %d times, want 1 — a hotkey press must not pay a listing every time", f.listed)
	}
}

// TestAliasFallsBackToTheBuiltInListWhenTheListingFails: a listing hiccup must
// not fail the fix; the built-in list's newest of the family stands in.
func TestAliasFallsBackToTheBuiltInListWhenTheListingFails(t *testing.T) {
	f := newFakeAnthropic(t, "")
	for alias, want := range map[string]string{
		FamilyHaiku:  "claude-haiku-5-5",
		FamilySonnet: "claude-sonnet-5-5",
		FamilyOpus:   "claude-opus-5-5",
		FamilyFable:  "claude-fable-5-1",
	} {
		if _, err := f.client("k-fallback").Complete(context.Background(), Request{Model: alias, User: "x", MaxTokens: 64}); err != nil {
			t.Fatalf("%s: %v", alias, err)
		}
		if got := f.last(t)["model"]; got != want {
			t.Errorf("%s went out as %v, want the built-in %s", alias, got, want)
		}
	}
}

// TestCuratedAnthropicListIsNewestFirstPerFamily holds the invariant the
// fallback relies on.
func TestCuratedAnthropicListIsNewestFirstPerFamily(t *testing.T) {
	for family, want := range map[string]string{
		FamilyHaiku: "claude-haiku-5-5", FamilySonnet: "claude-sonnet-5-5", FamilyOpus: "claude-opus-5-5",
	} {
		if got, _ := curatedNewestOfFamily(family); got != want {
			t.Errorf("curated newest %s = %q, want %q", family, got, want)
		}
	}
}

// TestPinnedModelWithoutOptionsNeverLists: an explicit ID with no effort costs
// exactly what it cost before this change — no listing round trip.
func TestPinnedModelWithoutOptionsNeverLists(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	if _, err := f.client("k-pinned").Complete(context.Background(), Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 64}); err != nil {
		t.Fatal(err)
	}
	if f.listed != 0 {
		t.Errorf("listed %d times for a pinned model with no options", f.listed)
	}
	body := f.last(t)
	if _, ok := body["output_config"]; ok {
		t.Errorf("output_config = %v, want none", body["output_config"])
	}
	if _, ok := body["thinking"]; ok {
		t.Errorf("thinking = %v, want none", body["thinking"])
	}
}

func TestEffortIsSentWhereTheModelTakesIt(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	if _, err := f.client("k-effort").Complete(context.Background(), Request{Model: FamilySonnet, User: "x", MaxTokens: 64, Effort: EffortLow}); err != nil {
		t.Fatal(err)
	}
	if got := outputConfig(f.last(t))["effort"]; got != "low" {
		t.Errorf("effort = %v, want low", got)
	}
}

// TestEffortIsLeftOutWhereTheModelRefusesIt: Haiku 4.5 answers any effort with
// a 400, Sonnet 4.6 an xhigh (both verified live). The listing says so, so the
// level is not sent and the user never sees the 400.
func TestEffortIsLeftOutWhereTheModelRefusesIt(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	f.reject = func(body map[string]any) string {
		if outputConfig(body)["effort"] != nil {
			return "This model does not support the effort parameter."
		}
		return ""
	}
	cases := []struct{ model, effort string }{
		{"claude-haiku-4-5-20251001", EffortLow},
		{"claude-sonnet-4-6", EffortXHigh},
	}
	for _, c := range cases {
		if _, err := f.client("k-refuse").Complete(context.Background(), Request{Model: c.model, User: "x", MaxTokens: 64, Effort: c.effort}); err != nil {
			t.Fatalf("%s/%s: %v", c.model, c.effort, err)
		}
		if got := outputConfig(f.last(t))["effort"]; got != nil {
			t.Errorf("%s: effort %v was sent to a model that refuses it", c.model, got)
		}
	}
	if len(f.bodies) != 2 {
		t.Errorf("%d requests, want 2 — the level should be left out up front, not retried", len(f.bodies))
	}
}

// TestRejectedEffortIsRetriedWithout: the listing does not describe the model
// (an ID it does not carry), the model answers 400 on the effort, and the
// request goes once more without it instead of failing the fix.
func TestRejectedEffortIsRetriedWithout(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	f.reject = func(body map[string]any) string {
		if outputConfig(body)["effort"] != nil {
			return "This model does not support effort level 'max'. Supported levels: high, low, medium."
		}
		return ""
	}
	resp, err := f.client("k-retry").Complete(context.Background(), Request{Model: "claude-unlisted-1", User: "x", MaxTokens: 64, Effort: EffortMax})
	if err != nil {
		t.Fatalf("Complete: %v — the effort rejection should have been retried without it", err)
	}
	if resp.Text != "fixed" {
		t.Errorf("Text = %q", resp.Text)
	}
	if len(f.bodies) != 2 || outputConfig(f.bodies[1])["effort"] != nil {
		t.Errorf("want a second request without effort, got %v", f.bodies)
	}
}

// TestUnrelated400IsNotRetried: only an effort or thinking rejection earns the
// retry; any other invalid request fails as before, once.
func TestUnrelated400IsNotRetried(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	f.reject = func(map[string]any) string { return "max_tokens: too large" }
	_, err := f.client("k-unrelated").Complete(context.Background(), Request{Model: "claude-unlisted-1", User: "x", MaxTokens: 64, Effort: EffortLow})
	if err == nil {
		t.Fatal("want the 400 reported")
	}
	if len(f.bodies) != 1 {
		t.Errorf("%d requests, want 1", len(f.bodies))
	}
}

// TestNoThinkingOnlyWhereTheModelCanTurnItOff: Haiku 5.5 accepts thinking
// disabled; Sonnet 5.5 answers it with a 400, so it is not sent there.
func TestNoThinkingOnlyWhereTheModelCanTurnItOff(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	if _, err := f.client("k-think").Complete(context.Background(), Request{Model: FamilyHaiku, User: "x", MaxTokens: 64, NoThinking: true}); err != nil {
		t.Fatal(err)
	}
	thinking, _ := f.last(t)["thinking"].(map[string]any)
	if thinking["type"] != "disabled" {
		t.Errorf("thinking = %v, want disabled on Haiku 5.5", f.last(t)["thinking"])
	}

	if _, err := f.client("k-think").Complete(context.Background(), Request{Model: FamilySonnet, User: "x", MaxTokens: 64, NoThinking: true}); err != nil {
		t.Fatal(err)
	}
	if got, ok := f.last(t)["thinking"]; ok {
		t.Errorf("thinking = %v sent to Sonnet 5.5, which cannot turn it off", got)
	}
}

func TestUnknownEffortLevelIsNotSent(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	if _, err := f.client("k-unknown").Complete(context.Background(), Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 64, Effort: "extreme"}); err != nil {
		t.Fatal(err)
	}
	if got := outputConfig(f.last(t))["effort"]; got != nil {
		t.Errorf("effort = %v, want an unknown level dropped", got)
	}
}

// TestPickerOffersAliasesForListedFamiliesOnly: each family the account lists
// gets an alias entry naming its resolution; Fable, which this listing lacks,
// is not offered.
func TestPickerOffersAliasesForListedFamiliesOnly(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	list, err := ListModels(context.Background(), ProviderClaude, Config{APIKey: "k-picker", BaseURL: f.baseURL})
	if err != nil {
		t.Fatal(err)
	}
	var aliases []string
	for _, m := range list.Models {
		if IsModelFamily(m.ID) {
			aliases = append(aliases, m.ID+"="+m.Resolved)
		}
	}
	want := "opus=claude-opus-5-5,sonnet=claude-sonnet-5-5,haiku=claude-haiku-5-5"
	if strings.Join(aliases, ",") != want {
		t.Errorf("aliases = %v, want %s", aliases, want)
	}

	withFable := strings.Replace(liveListing, `{"data":[`, `{"data":[
	{"type":"model","id":"claude-fable-5-1","display_name":"Claude Fable 5.1","created_at":"2026-08-28T00:00:00Z"},`, 1)
	g := newFakeAnthropic(t, withFable)
	list, _ = ListModels(context.Background(), ProviderClaude, Config{APIKey: "k-picker", BaseURL: g.baseURL})
	if !strings.Contains(listIDs(list), "fable") {
		t.Errorf("an account that lists Fable is not offered the alias: %s", listIDs(list))
	}
}

// TestResolveModelLeavesEverythingButAnAPIAliasAlone: the eval calls this to
// record what ran; it must not touch IDs or other providers' names.
func TestResolveModelLeavesEverythingButAnAPIAliasAlone(t *testing.T) {
	f := newFakeAnthropic(t, liveListing)
	cfg := Config{APIKey: "k-resolve", BaseURL: f.baseURL}
	if got := ResolveModel(context.Background(), ProviderClaude, FamilySonnet, cfg); got != "claude-sonnet-5-5" {
		t.Errorf("claude sonnet = %q", got)
	}
	if got := ResolveModel(context.Background(), ProviderClaude, "claude-sonnet-4-6", cfg); got != "claude-sonnet-4-6" {
		t.Errorf("pinned = %q", got)
	}
	if got := ResolveModel(context.Background(), ProviderClaudeCode, FamilySonnet, cfg); got != FamilySonnet {
		t.Errorf("claude-code alias = %q, want it left for the CLI", got)
	}
}

func listIDs(list ModelList) string {
	var ids []string
	for _, m := range list.Models {
		ids = append(ids, m.ID)
	}
	return strings.Join(ids, ",")
}

// ── Claude Code CLI ──

func TestClaudeCodePassesEffort(t *testing.T) {
	s := newStub(t)
	s.replies(successEnvelope)
	if _, err := s.client().Complete(context.Background(), Request{Model: "sonnet", User: "x", Effort: EffortHigh}); err != nil {
		t.Fatal(err)
	}
	if v, ok := argValue(s.argv(), "--effort"); !ok || v != "high" {
		t.Errorf("--effort = %q (present %v), want high", v, ok)
	}
}

func TestClaudeCodeSendsNoEffortByDefault(t *testing.T) {
	s := newStub(t)
	s.replies(successEnvelope)
	if _, err := s.client().Complete(context.Background(), Request{Model: "sonnet", User: "x"}); err != nil {
		t.Fatal(err)
	}
	if containsArg(s.argv(), "--effort") {
		t.Errorf("--effort sent with no effort chosen: %v", s.argv())
	}
}

// TestClaudeCodeRetriesWithoutARejectedEffort: the CLI has no listing to check
// a level against, so a run that fails over the effort is run again without.
func TestClaudeCodeRetriesWithoutARejectedEffort(t *testing.T) {
	s := newStub(t)
	s.replies(successEnvelope)
	t.Setenv("CLAUDESTUB_REJECT_EFFORT", "1")
	resp, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x", Effort: EffortXHigh})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text == "" {
		t.Error("empty text after the retry")
	}
	if containsArg(s.argv(), "--effort") {
		t.Errorf("the retry still carried --effort: %v", s.argv())
	}
}

func TestClaudeCodeReportsTheAnsweringModel(t *testing.T) {
	s := newStub(t)
	s.replies(`{"result":"ok","is_error":false,"modelUsage":{"claude-haiku-5-5":{"outputTokens":3}}}`)
	resp, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Model != "claude-haiku-5-5" {
		t.Errorf("Model = %q, want the ID behind the alias", resp.Model)
	}
}

// ── OpenAI ──

func TestOpenAIReasoningEffortOnlyForDocumentedModels(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		want any
	}{
		{"luna fix asks for none", Request{Model: "gpt-6-luna", NoThinking: true}, "none"},
		{"astra cannot do none", Request{Model: "gpt-6-astra", NoThinking: true}, nil},
		{"6.1 sol cannot do none", Request{Model: "gpt-6.1-sol", NoThinking: true}, nil},
		{"explicit effort on astra", Request{Model: "gpt-6-astra", Effort: EffortHigh}, "high"},
		{"undocumented model gets nothing", Request{Model: "gpt-4o-mini", NoThinking: true, Effort: EffortLow}, nil},
		{"no option, nothing sent", Request{Model: "gpt-6-luna"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got capture
			srv := newServer(t, &got, http.StatusOK, `{"model":"`+c.req.Model+`","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
			c.req.User = "x"
			if _, err := newOpenAI(Config{APIKey: "sk-test", BaseURL: srv.URL}).Complete(context.Background(), c.req); err != nil {
				t.Fatal(err)
			}
			if got.body["reasoning_effort"] != c.want {
				t.Errorf("reasoning_effort = %v, want %v", got.body["reasoning_effort"], c.want)
			}
		})
	}
}

func TestOpenAIRetriesWithoutARejectedReasoningEffort(t *testing.T) {
	var calls []map[string]any
	base := newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		calls = append(calls, body)
		w.Header().Set("Content-Type", "application/json")
		if body["reasoning_effort"] != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"Unsupported value: 'reasoning_effort' does not support 'none' with this model.","type":"invalid_request_error"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"model":"gpt-6-luna","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
	})
	resp, err := newOpenAI(Config{APIKey: "sk-test", BaseURL: base}).Complete(context.Background(), Request{Model: "gpt-6-luna", User: "x", NoThinking: true})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Model != "gpt-6-luna" || len(calls) != 2 || calls[1]["reasoning_effort"] != nil {
		t.Errorf("want one retry without reasoning_effort; calls = %v", calls)
	}
}
