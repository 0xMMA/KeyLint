package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// orCatalogue is a models endpoint answer in the shape OpenRouter serves at
// GET /api/v1/models (checked against the live endpoint on 2026-10-15), cut
// down to one entry per case the client has to tell apart.
const orCatalogue = `{"data":[
	{"id":"anthropic/claude-haiku-5.5","name":"Anthropic: Claude Haiku 5.5",
	 "architecture":{"output_modalities":["text"]},"supported_parameters":["max_tokens","reasoning","response_format"],
	 "reasoning":{"mandatory":false,"supported_efforts":["max","xhigh","high","medium","low"],"default_effort":"medium"}},
	{"id":"anthropic/claude-haiku-5.5:batch","name":"Anthropic: Claude Haiku 5.5 (batch)",
	 "architecture":{"output_modalities":["text"]},
	 "reasoning":{"mandatory":false,"supported_efforts":["max","xhigh","high","medium","low"],"default_effort":"medium"}},
	{"id":"anthropic/claude-sonnet-5.5","name":"Anthropic: Claude Sonnet 5.5",
	 "architecture":{"output_modalities":["text"]},
	 "reasoning":{"mandatory":true,"supported_efforts":["max","xhigh","high","medium","low"],"default_effort":"high"}},
	{"id":"openai/gpt-6-luna","name":"OpenAI: GPT-6 Luna",
	 "architecture":{"output_modalities":["text"]},
	 "reasoning":{"mandatory":false,"default_enabled":true,"supported_efforts":["max","xhigh","high","medium","low","none"],"default_effort":"medium"}},
	{"id":"deepseek/any-effort","name":"DeepSeek: Any Effort",
	 "architecture":{"output_modalities":["text"]},
	 "reasoning":{"mandatory":false,"supported_efforts":null,"default_effort":"high"}},
	{"id":"vendor/picky","name":"Vendor: Picky",
	 "architecture":{"output_modalities":["text"]},
	 "reasoning":{"mandatory":false,"supported_efforts":["high","low"],"default_effort":"high"}},
	{"id":"meta/no-reasoning","name":"Meta: No Reasoning",
	 "architecture":{"output_modalities":["text"]}},
	{"id":"vendor/pictures","name":"Vendor: Pictures Only",
	 "architecture":{"output_modalities":["image"]}},
	{"id":"vendor/free-model:free","name":"Vendor: Free Model (free)",
	 "architecture":{"output_modalities":["text"]}}
]}`

const orOK = `{"id":"gen-1","model":"%MODEL%","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"fixed"}}]}`

// fakeOpenRouter serves the catalogue and the chat endpoint, and records what
// reached each.
type fakeOpenRouter struct {
	mu        sync.Mutex
	catalogue string // "" answers the catalogue with a 500
	listed    int
	listAuth  []string
	listUA    []string
	bodies    []map[string]any
	headers   []http.Header
	paths     []string
	// reject, when it returns a non-empty body, answers the chat call with
	// that body and status.
	reject  func(body map[string]any) (int, string)
	reply   string // the chat reply; orOK when empty
	baseURL string
}

func newFakeOpenRouter(t *testing.T, catalogue string) *fakeOpenRouter {
	t.Helper()
	f := &fakeOpenRouter{catalogue: catalogue}
	f.baseURL = newRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			f.listed++
			f.listAuth = append(f.listAuth, r.Header.Get("Authorization"))
			f.listUA = append(f.listUA, r.Header.Get("User-Agent"))
			if f.catalogue == "" {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"error":{"code":500,"message":"down"}}`)
				return
			}
			_, _ = io.WriteString(w, f.catalogue)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.bodies = append(f.bodies, body)
		f.headers = append(f.headers, r.Header.Clone())
		f.paths = append(f.paths, r.URL.Path)
		if f.reject != nil {
			if status, msg := f.reject(body); msg != "" {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, msg)
				return
			}
		}
		reply := f.reply
		if reply == "" {
			reply = orOK
		}
		model, _ := body["model"].(string)
		_, _ = io.WriteString(w, strings.ReplaceAll(reply, "%MODEL%", model))
	})
	return f
}

func (f *fakeOpenRouter) client() Client {
	return newOpenRouter(Config{APIKey: "sk-or-test", BaseURL: f.baseURL})
}

func (f *fakeOpenRouter) last(t *testing.T) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.bodies) == 0 {
		t.Fatal("no chat request reached the server")
	}
	return f.bodies[len(f.bodies)-1]
}

// reasoningEffort is what the request asked for in OpenRouter's reasoning
// object, or nil when the object is absent.
func reasoningEffort(t *testing.T, body map[string]any) any {
	t.Helper()
	raw, ok := body["reasoning"]
	if !ok {
		return nil
	}
	object, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("reasoning = %v, want an object", raw)
	}
	return object["effort"]
}

// TestOpenRouterCompleteRequestShape pins the wire: OpenAI's chat dialect at
// /v1/chat/completions under the configured base, the key as a bearer token,
// the output limit as max_tokens, and nothing that says reasoning when no
// option asked for it.
func TestOpenRouterCompleteRequestShape(t *testing.T) {
	f := newFakeOpenRouter(t, orCatalogue)
	resp, err := f.client().Complete(context.Background(), Request{
		System: "system prompt", User: "user message", Model: "anthropic/claude-sonnet-5.5", MaxTokens: 2048,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "fixed" || resp.Model != "anthropic/claude-sonnet-5.5" {
		t.Errorf("response = %+v, want the text and the model that answered", resp)
	}
	body := f.last(t)
	if f.paths[0] != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", f.paths[0])
	}
	if got := f.headers[0].Get("Authorization"); got != "Bearer sk-or-test" {
		t.Errorf("Authorization = %q, want the OpenRouter key as a bearer token", got)
	}
	if body["model"] != "anthropic/claude-sonnet-5.5" {
		t.Errorf("model = %v", body["model"])
	}
	// Fix's 2048 and Pyramidize's 16000 bound the reply here as they do on the
	// Anthropic API. Without it OpenRouter picks its own ceiling, which also
	// counts against the credit it reserves for the call.
	if body["max_tokens"] != float64(2048) {
		t.Errorf("max_tokens = %v, want 2048", body["max_tokens"])
	}
	if _, ok := body["reasoning"]; ok {
		t.Errorf("reasoning = %v sent with no effort and no NoThinking", body["reasoning"])
	}
	if _, ok := body["reasoning_effort"]; ok {
		t.Error("reasoning_effort is OpenAI's top-level field; OpenRouter takes the reasoning object")
	}
	messages, _ := body["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("messages = %v, want system and user", body["messages"])
	}
	// No option set, so the catalogue was not needed and not asked.
	if f.listed != 0 {
		t.Errorf("listed the catalogue %d times for a call that needed nothing from it", f.listed)
	}
}

// TestOpenRouterSendsNoAttributionOrFingerprint: OpenRouter documents two
// optional headers that name the calling app on its leaderboards. KeyLint
// sends neither, for the same reason it strips the SDK's machine headers: a
// provider learns what it needs for the call and nothing about the machine or
// the app making it.
func TestOpenRouterSendsNoAttributionOrFingerprint(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-ENVIRONMENT-SHOULD-NOT-LEAK")
	t.Setenv("OPENAI_CUSTOM_HEADERS", "X-Injected-Marker: leaked")
	f := newFakeOpenRouter(t, orCatalogue)
	if _, err := f.client().Complete(context.Background(), Request{Model: "meta/no-reasoning", User: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	h := f.headers[0]
	for _, name := range []string{"HTTP-Referer", "Referer", "X-Title", "X-Injected-Marker", "OpenAI-Organization", "OpenAI-Project"} {
		if v := h.Get(name); v != "" {
			t.Errorf("%s = %q reached OpenRouter", name, v)
		}
	}
	for _, name := range fingerprintHeaders {
		if name == "User-Agent" {
			continue
		}
		if v := h.Get(name); v != "" {
			t.Errorf("%s = %q reached OpenRouter", name, v)
		}
	}
	if ua := h.Get("User-Agent"); ua != userAgent {
		t.Errorf("User-Agent = %q, want %q", ua, userAgent)
	}
	for name, values := range h {
		for _, v := range values {
			if strings.Contains(v, runtime.GOOS) || strings.Contains(v, runtime.Version()) || strings.Contains(v, "ENVIRONMENT") {
				t.Errorf("%s = %q leaks the machine or the environment", name, v)
			}
		}
	}
}

// urlTransport answers every request itself and records the full URL, so the
// production endpoint can be checked without dialling it.
type urlTransport struct {
	mu   sync.Mutex
	urls []string
	body string
}

func (u *urlTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u.mu.Lock()
	u.urls = append(u.urls, req.URL.Scheme+"://"+req.URL.Host+req.URL.Path)
	u.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(u.body)),
		Request:    req,
	}, nil
}

// TestOpenRouterProductionEndpoint covers what every other test bypasses: no
// base URL configured. The SDK appends /v1/ to the base, so the base has to be
// https://openrouter.ai/api — and OPENAI_BASE_URL, which the SDK would read,
// must not move it.
func TestOpenRouterProductionEndpoint(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "http://redirected.invalid")

	chat := &urlTransport{body: `{"choices":[{"finish_reason":"stop","message":{"content":"ok"}}]}`}
	c := newOpenRouter(Config{APIKey: "sk-or-test", HTTPClient: &http.Client{Transport: chat}})
	if _, err := c.Complete(context.Background(), Request{Model: "anthropic/claude-sonnet-5.5", User: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(chat.urls) != 1 || chat.urls[0] != "https://openrouter.ai/api/v1/chat/completions" {
		t.Errorf("chat went to %v, want https://openrouter.ai/api/v1/chat/completions", chat.urls)
	}

	list := &urlTransport{body: `{"data":[{"id":"anthropic/claude-haiku-5.5","name":"Anthropic: Claude Haiku 5.5"}]}`}
	if _, err := ListModels(context.Background(), ProviderOpenRouter, Config{HTTPClient: &http.Client{Transport: list}}); err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(list.urls) != 1 || list.urls[0] != "https://openrouter.ai/api/v1/models" {
		t.Errorf("catalogue came from %v, want https://openrouter.ai/api/v1/models", list.urls)
	}
}

// TestOpenRouterReasoningFollowsTheCatalogue: OpenRouter takes reasoning as
// {"reasoning": {"effort": …}} and support varies per routed model, so what is
// sent is read off that model's catalogue entry. "none" turns reasoning off and
// is refused by a model whose reasoning is mandatory.
func TestOpenRouterReasoningFollowsTheCatalogue(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		want any
	}{
		{"listed effort is sent", Request{Model: "anthropic/claude-sonnet-5.5", Effort: EffortHigh}, "high"},
		{"unlisted effort is left out", Request{Model: "vendor/picky", Effort: EffortMedium}, nil},
		{"null efforts take any level", Request{Model: "deepseek/any-effort", Effort: EffortXHigh}, "xhigh"},
		{"no reasoning, nothing sent", Request{Model: "meta/no-reasoning", Effort: EffortHigh}, nil},
		{"Fix on Haiku turns reasoning off", Request{Model: "anthropic/claude-haiku-5.5", NoThinking: true}, "none"},
		{"Fix on Luna turns reasoning off", Request{Model: "openai/gpt-6-luna", NoThinking: true}, "none"},
		{"mandatory reasoning is not asked to stop", Request{Model: "anthropic/claude-sonnet-5.5", NoThinking: true}, nil},
		{"no reasoning, no none", Request{Model: "meta/no-reasoning", NoThinking: true}, nil},
		{"unknown level is never sent", Request{Model: "deepseek/any-effort", Effort: "turbo"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFakeOpenRouter(t, orCatalogue)
			c.req.User = "x"
			if _, err := f.client().Complete(context.Background(), c.req); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			body := f.last(t)
			if got := reasoningEffort(t, body); got != c.want {
				t.Errorf("reasoning.effort = %v, want %v", got, c.want)
			}
			if _, ok := body["reasoning_effort"]; ok {
				t.Error("reasoning_effort must never reach OpenRouter")
			}
		})
	}
}

// TestOpenRouterWithoutTheCatalogue: when the catalogue cannot be had, a chosen
// effort is sent anyway — the user asked for it, and a refusal is retried
// without it — but "none" is not: whether a model can stop reasoning is
// exactly what is unknown, and a mandatory one would refuse the call.
func TestOpenRouterWithoutTheCatalogue(t *testing.T) {
	t.Run("effort is sent", func(t *testing.T) {
		f := newFakeOpenRouter(t, "")
		if _, err := f.client().Complete(context.Background(), Request{Model: "anthropic/claude-sonnet-5.5", User: "x", Effort: EffortLow}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if got := reasoningEffort(t, f.last(t)); got != "low" {
			t.Errorf("reasoning.effort = %v, want low", got)
		}
	})
	t.Run("none is not", func(t *testing.T) {
		f := newFakeOpenRouter(t, "")
		if _, err := f.client().Complete(context.Background(), Request{Model: "anthropic/claude-haiku-5.5", User: "x", NoThinking: true}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if got := reasoningEffort(t, f.last(t)); got != nil {
			t.Errorf("reasoning.effort = %v with no catalogue, want nothing", got)
		}
	})
	t.Run("a model the catalogue does not list counts as unknown", func(t *testing.T) {
		f := newFakeOpenRouter(t, orCatalogue)
		if _, err := f.client().Complete(context.Background(), Request{Model: "vendor/brand-new", User: "x", Effort: EffortHigh, NoThinking: true}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if got := reasoningEffort(t, f.last(t)); got != "high" {
			t.Errorf("reasoning.effort = %v, want the chosen high", got)
		}
	})
}

// TestOpenRouterRetriesWithoutARejectedReasoning mirrors the OpenAI and
// Anthropic clients: a model that refuses the reasoning option gets the call
// once more without it, rather than failing the user's text over a setting.
func TestOpenRouterRetriesWithoutARejectedReasoning(t *testing.T) {
	f := newFakeOpenRouter(t, "")
	f.reject = func(body map[string]any) (int, string) {
		if _, ok := body["reasoning"]; ok {
			return http.StatusBadRequest, `{"error":{"code":400,"message":"Reasoning is not supported for this model"}}`
		}
		return 0, ""
	}
	resp, err := f.client().Complete(context.Background(), Request{Model: "vendor/picky", User: "x", Effort: EffortMax})
	if err != nil {
		t.Fatalf("Complete: %v — the rejection should have been retried without reasoning", err)
	}
	if resp.Text != "fixed" || len(f.bodies) != 2 {
		t.Fatalf("want two calls, the second answered; got %d", len(f.bodies))
	}
	if _, ok := f.bodies[1]["reasoning"]; ok {
		t.Errorf("the retry still carried reasoning = %v", f.bodies[1]["reasoning"])
	}
}

func TestOpenRouterDoesNotRetryAnUnrelated400(t *testing.T) {
	f := newFakeOpenRouter(t, "")
	f.reject = func(map[string]any) (int, string) {
		return http.StatusBadRequest, `{"error":{"code":400,"message":"messages: too long"}}`
	}
	_, err := f.client().Complete(context.Background(), Request{Model: "vendor/picky", User: "x", Effort: EffortHigh})
	if err == nil || len(f.bodies) != 1 {
		t.Errorf("err = %v, calls = %d — want one failed call", err, len(f.bodies))
	}
}

// TestOpenRouterCatalogueIsCached: Fix asks Haiku not to reason on every
// hotkey press, and every press must not pay a 0.8 MB listing for it.
func TestOpenRouterCatalogueIsCached(t *testing.T) {
	f := newFakeOpenRouter(t, orCatalogue)
	for i := 0; i < 3; i++ {
		if _, err := f.client().Complete(context.Background(), Request{Model: "anthropic/claude-haiku-5.5", User: "x", NoThinking: true}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
	}
	if f.listed != 1 {
		t.Errorf("listed the catalogue %d times for three calls, want once", f.listed)
	}

	// A failure is remembered too, briefly: a catalogue that is down is not
	// asked again on the very next press.
	g := newFakeOpenRouter(t, "")
	for i := 0; i < 2; i++ {
		if _, err := g.client().Complete(context.Background(), Request{Model: "anthropic/claude-haiku-5.5", User: "x", NoThinking: true}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
	}
	if g.listed != 1 {
		t.Errorf("asked a failing catalogue %d times in a row, want once", g.listed)
	}
}

// TestOpenRouterErrorsNameOpenRouter: the status wording is the shared one,
// and the provider is named as the user knows it — the hotkey toast says
// "The OpenRouter account is out of credit." from Provider and Status.
func TestOpenRouterErrorsNameOpenRouter(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusPaymentRequired, "OpenRouter error 402: the account is out of credit or has a billing problem"},
		{http.StatusUnauthorized, "OpenRouter error 401: the API key was not accepted"},
		{http.StatusNotFound, `OpenRouter error 404: the model "vendor/gone" was not found`},
	}
	for _, c := range cases {
		f := newFakeOpenRouter(t, orCatalogue)
		f.reject = func(map[string]any) (int, string) {
			return c.status, `{"error":{"code":400,"message":"secret detail from the provider"}}`
		}
		_, err := f.client().Complete(context.Background(), Request{Model: "vendor/gone", User: "x"})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("status %d: err = %v, want %q", c.status, err, c.want)
			continue
		}
		if strings.Contains(err.Error(), "secret detail") {
			t.Errorf("the provider's own message reached the error string: %v", err)
		}
		var se *StatusError
		if !errors.As(err, &se) || se.Provider != "OpenRouter" || se.Status != c.status {
			t.Errorf("err = %#v, want a StatusError for OpenRouter %d", err, c.status)
		}
	}
}

// TestOpenRouterCutOffWhileReasoning: a model that reasoned and hit the
// output limit gets the reasoning wording, which names the remedy, instead of
// "try a shorter selection".
func TestOpenRouterCutOffWhileReasoning(t *testing.T) {
	f := newFakeOpenRouter(t, orCatalogue)
	f.reply = `{"model":"%MODEL%","choices":[{"finish_reason":"length","message":{"role":"assistant","content":"","reasoning":"Let me think about"}}],
		"usage":{"completion_tokens":2048,"completion_tokens_details":{"reasoning_tokens":2048}}}`
	_, err := f.client().Complete(context.Background(), Request{Model: "anthropic/claude-sonnet-5.5", User: "x", MaxTokens: 2048})
	if !errors.Is(err, ErrOutputLimitThinking) {
		t.Errorf("err = %v, want ErrOutputLimitThinking", err)
	}

	g := newFakeOpenRouter(t, orCatalogue)
	g.reply = `{"model":"%MODEL%","choices":[{"finish_reason":"length","message":{"role":"assistant","content":"They are going to the"}}],
		"usage":{"completion_tokens":2048,"completion_tokens_details":{"reasoning_tokens":0}}}`
	_, err = g.client().Complete(context.Background(), Request{Model: "meta/no-reasoning", User: "x", MaxTokens: 2048})
	if !errors.Is(err, ErrOutputLimit) || errors.Is(err, ErrOutputLimitThinking) {
		t.Errorf("err = %v, want the plain ErrOutputLimit", err)
	}
}

// TestListModelsOpenRouterReadsThePublicCatalogue: the picker gets the whole
// catalogue minus what a chat call cannot use, labelled with OpenRouter's own
// names, in OpenRouter's order — and the catalogue is public, so the key is
// not sent to it even when one is set.
func TestListModelsOpenRouterReadsThePublicCatalogue(t *testing.T) {
	f := newFakeOpenRouter(t, orCatalogue)
	list, err := ListModels(context.Background(), ProviderOpenRouter, Config{APIKey: "sk-or-secret", BaseURL: f.baseURL})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if list.Source != ModelSourceLive {
		t.Errorf("source = %q, want live", list.Source)
	}
	var ids []string
	for _, m := range list.Models {
		ids = append(ids, m.ID)
		if m.Resolved != "" {
			t.Errorf("%s resolves to %q; OpenRouter IDs are explicit, there is no alias", m.ID, m.Resolved)
		}
	}
	want := "anthropic/claude-haiku-5.5,anthropic/claude-sonnet-5.5,openai/gpt-6-luna,deepseek/any-effort,vendor/picky,meta/no-reasoning,vendor/free-model:free"
	if strings.Join(ids, ",") != want {
		t.Errorf("models = %v\nwant     %s\n(:batch is OpenRouter's asynchronous Batch API, not a chat model; image-only output cannot answer)", ids, want)
	}
	if list.Models[0].Label != "Anthropic: Claude Haiku 5.5" {
		t.Errorf("label = %q, want OpenRouter's name", list.Models[0].Label)
	}
	if f.listAuth[0] != "" {
		t.Errorf("Authorization = %q sent to the public catalogue", f.listAuth[0])
	}
	if f.listUA[0] != userAgent {
		t.Errorf("User-Agent = %q, want %q", f.listUA[0], userAgent)
	}

	// The listing primed the cache a completion reads, so the next hotkey
	// press does not list again.
	if _, err := newOpenRouter(Config{APIKey: "sk-or-secret", BaseURL: f.baseURL}).Complete(context.Background(),
		Request{Model: "anthropic/claude-haiku-5.5", User: "x", NoThinking: true}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if f.listed != 1 {
		t.Errorf("listed %d times, want the settings listing to serve the completion", f.listed)
	}
}

func TestListModelsOpenRouterStates(t *testing.T) {
	t.Run("unreachable falls back to the built-in list", func(t *testing.T) {
		f := newFakeOpenRouter(t, "")
		list, err := ListModels(context.Background(), ProviderOpenRouter, Config{BaseURL: f.baseURL})
		if err == nil {
			t.Error("the failure should be reported alongside the fallback")
		}
		if list.Source != ModelSourceUnreachable || len(list.Models) == 0 {
			t.Errorf("list = %+v, want the built-in list marked unreachable", list)
		}
	})
	t.Run("an empty catalogue is empty", func(t *testing.T) {
		f := newFakeOpenRouter(t, `{"data":[]}`)
		list, _ := ListModels(context.Background(), ProviderOpenRouter, Config{BaseURL: f.baseURL})
		if list.Source != ModelSourceEmpty || len(list.Models) != 0 {
			t.Errorf("list = %+v, want empty", list)
		}
	})
	t.Run("nothing callable is unusable, not empty", func(t *testing.T) {
		f := newFakeOpenRouter(t, `{"data":[{"id":"a/b:batch","name":"B"},{"id":"a/c","name":"C","architecture":{"output_modalities":["audio"]}}]}`)
		list, _ := ListModels(context.Background(), ProviderOpenRouter, Config{BaseURL: f.baseURL})
		if list.Source != ModelSourceUnusable || len(list.Models) == 0 {
			t.Errorf("list = %+v, want the built-in list marked unusable", list)
		}
	})
}

// TestOpenRouterDefaultsAndFallback: the defaults are the owner's choice
// (2026-10-15) and explicit vendor/model IDs — no alias — and the built-in
// list a user sees when the catalogue is down offers them.
func TestOpenRouterDefaultsAndFallback(t *testing.T) {
	if got := DefaultModel(ProviderOpenRouter, FeatureFix); got != "anthropic/claude-haiku-5.5" {
		t.Errorf("Fix default = %q", got)
	}
	if got := DefaultModel(ProviderOpenRouter, FeaturePyramidize); got != "anthropic/claude-sonnet-5.5" {
		t.Errorf("Pyramidize default = %q", got)
	}
	curated := CuratedModels(ProviderOpenRouter, ModelSourceUnreachable).Models
	for _, feature := range []string{FeatureFix, FeaturePyramidize} {
		id := DefaultModel(ProviderOpenRouter, feature)
		if !openRouterChatModel(id, []string{"text"}) {
			t.Errorf("the %s default %q is filtered out of the catalogue", feature, id)
		}
		found := false
		for _, m := range curated {
			found = found || m.ID == id
		}
		if !found {
			t.Errorf("the built-in list does not offer the %s default %q", feature, id)
		}
	}
	for _, m := range curated {
		if !strings.Contains(m.ID, "/") || IsModelFamily(m.ID) {
			t.Errorf("built-in entry %q is not an explicit vendor/model ID", m.ID)
		}
		if !openRouterChatModel(m.ID, []string{"text"}) {
			t.Errorf("built-in entry %q would be filtered out of a live catalogue", m.ID)
		}
	}
	// Fix's default must be one it asks not to reason, or the hotkey burns its
	// 2048 tokens on reasoning as Haiku 5.5 did on the Anthropic API.
	if !IsFastTier(DefaultModel(ProviderOpenRouter, FeatureFix)) {
		t.Error("the Fix default is not recognised as the fast tier")
	}
}
