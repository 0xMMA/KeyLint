package pyramidize

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// fakeClient stands in for a provider client and records the request it got.
type fakeClient struct {
	gotRequest llm.Request
	reply      string
	err        error
	// onComplete runs before the reply is returned, so a test can cancel the
	// operation from inside an in-flight call.
	onComplete func()
}

func (f *fakeClient) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	f.gotRequest = req
	if f.onComplete != nil {
		f.onComplete()
		<-ctx.Done()
		return llm.Response{}, ctx.Err()
	}
	if f.err != nil {
		return llm.Response{}, f.err
	}
	return llm.Response{Text: f.reply}, nil
}

// recorder captures what callAISync asked llm.New for.
type recorder struct {
	provider string
	cfg      llm.Config
	client   *fakeClient
}

func (r *recorder) new(provider string, cfg llm.Config) (llm.Client, error) {
	r.provider = provider
	r.cfg = cfg
	return r.client, nil
}

// newTestService builds a Service wired to a fake provider client. It needs no
// settings service because callAISync takes the settings value as an argument.
func newTestService() (*Service, *recorder) {
	rec := &recorder{client: &fakeClient{reply: `{"ok":true}`}}
	return &Service{client: &http.Client{}, newClient: rec.new}, rec
}

func TestCallAISyncModelDefaults(t *testing.T) {
	tests := []struct {
		provider  string
		wantModel string
	}{
		{"openai", llm.DefaultModel(llm.ProviderOpenAI, llm.FeaturePyramidize)},
		{"claude", llm.DefaultModel(llm.ProviderClaude, llm.FeaturePyramidize)},
		{"ollama", llm.DefaultModel(llm.ProviderOllama, llm.FeaturePyramidize)},
	}
	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			svc, rec := newTestService()
			cfg := settings.Default()
			cfg.ActiveProvider = tc.provider

			raw, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "key", "system", "user", documentSchema)
			if err != nil {
				t.Fatalf("callAISync: %v", err)
			}
			if raw != `{"ok":true}` {
				t.Errorf("raw = %q, want the provider reply", raw)
			}
			if rec.provider != tc.provider {
				t.Errorf("provider = %q, want %q", rec.provider, tc.provider)
			}
			if rec.client.gotRequest.Model != tc.wantModel {
				t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, tc.wantModel)
			}
		})
	}
}

func TestCallAISyncRequestShape(t *testing.T) {
	svc, rec := newTestService()
	cfg := settings.Default()
	cfg.ActiveProvider = "claude"

	if _, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "sk-ant-test", "system", "user", documentSchema); err != nil {
		t.Fatalf("callAISync: %v", err)
	}

	req := rec.client.gotRequest
	if req.System != "system" || req.User != "user" {
		t.Errorf("System/User = %q/%q, want system/user", req.System, req.User)
	}
	if !req.JSONMode {
		t.Error("the pipeline parses JSON replies, so the call must ask for an object")
	}
	if req.MaxTokens != maxTokens {
		t.Errorf("MaxTokens = %d, want %d", req.MaxTokens, maxTokens)
	}
	if rec.cfg.APIKey != "sk-ant-test" {
		t.Errorf("APIKey = %q, want the resolved key", rec.cfg.APIKey)
	}
	if rec.cfg.HTTPClient != svc.client {
		t.Error("the service HTTP client (90s timeout) must be handed to the provider client")
	}
	if rec.cfg.Feature != logFeature {
		t.Errorf("Feature = %q, want %q so the logs name the calling feature", rec.cfg.Feature, logFeature)
	}
}

func TestCallAISyncOverrides(t *testing.T) {
	svc, rec := newTestService()
	cfg := settings.Default()
	cfg.ActiveProvider = "openai"

	opts := aiOpts{provider: "claude", model: "claude-opus-4-1"}
	if _, err := svc.callAISync(context.Background(), cfg, opts, "key", "system", "user", documentSchema); err != nil {
		t.Fatalf("callAISync: %v", err)
	}
	if rec.provider != "claude" {
		t.Errorf("provider = %q, want the override %q", rec.provider, "claude")
	}
	if rec.client.gotRequest.Model != "claude-opus-4-1" {
		t.Errorf("Model = %q, want the override", rec.client.gotRequest.Model)
	}
}

func TestCallAISyncOllamaConfig(t *testing.T) {
	svc, rec := newTestService()
	cfg := settings.Default()
	cfg.ActiveProvider = "ollama"
	cfg.Providers.OllamaURL = "http://ollama.test:11434"

	if _, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "", "system", "user", documentSchema); err != nil {
		t.Fatalf("callAISync: %v", err)
	}
	if rec.cfg.BaseURL != "http://ollama.test:11434" {
		t.Errorf("BaseURL = %q, want the configured Ollama URL", rec.cfg.BaseURL)
	}
}

func TestCallAISyncUnsupportedProvider(t *testing.T) {
	svc, _ := newTestService()
	cfg := settings.Default()
	cfg.ActiveProvider = "bedrock"

	_, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "", "system", "user", documentSchema)
	if err == nil || !strings.Contains(err.Error(), `unsupported provider: "bedrock"`) {
		t.Fatalf("error = %v, want an unsupported-provider error", err)
	}
}

func TestCallAISyncPropagatesProviderError(t *testing.T) {
	svc, rec := newTestService()
	rec.client.err = errors.New("Claude error 429: rate limited")
	cfg := settings.Default()
	cfg.ActiveProvider = "claude"

	_, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "key", "system", "user", documentSchema)
	if err == nil || !strings.Contains(err.Error(), "Claude error 429") {
		t.Fatalf("error = %v, want the provider error", err)
	}
}

func TestResolveAPIKeyNoKeyProvider(t *testing.T) {
	svc, _ := newTestService()
	key, err := svc.resolveAPIKey("ollama")
	if err != nil {
		t.Fatalf("resolveAPIKey(ollama): %v", err)
	}
	if key != "" {
		t.Errorf("key = %q, want empty for a provider that needs none", key)
	}
}

// TestPyramidizeDefaultsUnchanged pins the models this pipeline shipped with;
// moving them into settings must not move them.
func TestPyramidizeDefaultsUnchanged(t *testing.T) {
	want := map[string]string{
		llm.ProviderOpenAI:     "gpt-5.2",
		llm.ProviderClaude:     "claude-sonnet-4-6",
		llm.ProviderOllama:     "llama3.2",
		llm.ProviderClaudeCode: "sonnet",
	}
	for provider, model := range want {
		if got := llm.DefaultModel(provider, llm.FeaturePyramidize); got != model {
			t.Errorf("default pyramidize model for %s = %q, want %q", provider, got, model)
		}
	}
	if maxTokens != 4096 {
		t.Errorf("maxTokens = %d, want 4096", maxTokens)
	}
}

// TestModelResolutionOrder pins the whole point of #33 step 4: a per-request
// override beats settings, settings beat the built-in default.
func TestModelResolutionOrder(t *testing.T) {
	configured := settings.Default()
	configured.ActiveProvider = "claude"
	configured.Models = map[string]settings.FeatureModels{
		"claude": {Pyramidize: "claude-opus-4-6"},
	}

	tests := []struct {
		name string
		cfg  settings.Settings
		opts aiOpts
		want string
	}{
		{
			name: "request override wins",
			cfg:  configured,
			opts: aiOpts{model: "claude-sonnet-5"},
			want: "claude-sonnet-5",
		},
		{
			name: "settings win over the default",
			cfg:  configured,
			opts: aiOpts{},
			want: "claude-opus-4-6",
		},
		{
			name: "the default applies when nothing is configured",
			cfg:  func() settings.Settings { c := settings.Default(); c.ActiveProvider = "claude"; return c }(),
			opts: aiOpts{},
			want: llm.DefaultModel(llm.ProviderClaude, llm.FeaturePyramidize),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, rec := newTestService()
			if _, err := svc.callAISync(context.Background(), tc.cfg, tc.opts, "key", "system", "user", nil); err != nil {
				t.Fatalf("callAISync: %v", err)
			}
			if got := rec.client.gotRequest.Model; got != tc.want {
				t.Errorf("Model = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCallAISyncOllamaSendsASystemMessage exercises the real provider client
// against an httptest server. Ollama used to take one glued-together prompt
// string; through its OpenAI-compatible endpoint the system prompt is a system
// message, and the result schema — which the native endpoint ignored — is sent.
func TestCallAISyncOllamaSendsASystemMessage(t *testing.T) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	var gotMessages []message
	var gotPath string
	var gotJSONMode bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var payload struct {
			Messages       []message      `json:"messages"`
			ResponseFormat map[string]any `json:"response_format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		gotMessages = payload.Messages
		gotJSONMode = payload.ResponseFormat["type"] == "json_schema"
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"{}"}}]}`)
	}))
	t.Cleanup(srv.Close)

	svc := &Service{client: &http.Client{}, newClient: llm.New}
	cfg := settings.Default()
	cfg.ActiveProvider = "ollama"
	cfg.Providers.OllamaURL = srv.URL

	if _, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "", "system", "user", documentSchema); err != nil {
		t.Fatalf("callAISync: %v", err)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q, want the OpenAI-compatible endpoint", gotPath)
	}
	if len(gotMessages) != 2 {
		t.Fatalf("messages = %v, want a system and a user message", gotMessages)
	}
	if gotMessages[0].Role != "system" || gotMessages[0].Content != "system" {
		t.Errorf("first message = %+v, want the system prompt in a system role", gotMessages[0])
	}
	if gotMessages[1].Role != "user" || gotMessages[1].Content != "user" {
		t.Errorf("second message = %+v, want the user message", gotMessages[1])
	}
	if !gotJSONMode {
		t.Error("response_format was not sent as json_schema; the pipeline parses JSON replies")
	}
}

// TestRefineGlobalCancelled covers the cancellation path that changed when the
// goroutine/select wrapper was replaced by a context-aware HTTP request: a
// cancel during an in-flight call must still surface as "cancelled".
func TestRefineGlobalCancelled(t *testing.T) {
	// os.UserConfigDir() reads XDG_CONFIG_HOME on Linux/macOS and APPDATA on Windows.
	envKey := "XDG_CONFIG_HOME"
	if runtime.GOOS == "windows" {
		envKey = "APPDATA"
	}
	original := os.Getenv(envKey)
	t.Cleanup(func() { os.Setenv(envKey, original) })
	os.Setenv(envKey, t.TempDir())

	cfg := settings.Default()
	cfg.ActiveProvider = "ollama" // needs no API key, so the keyring stays out of it
	// Built from an explicit config: no settings file to read, no keyring to
	// reach, so this test measures the code and not the machine.
	settingsSvc := settings.NewServiceFrom(cfg, settings.EnvOnlyKeys)

	svc := NewService(settingsSvc, nil)
	rec := &recorder{client: &fakeClient{}}
	svc.newClient = rec.new
	// Cancel from inside the call, the way the UI's Cancel button does.
	rec.client.onComplete = svc.CancelOperation

	_, err := svc.RefineGlobal(RefineGlobalRequest{
		FullCanvas: "canvas", OriginalText: "original", Instruction: "shorten it",
	})
	if err == nil || err.Error() != "cancelled" {
		t.Fatalf("RefineGlobal error = %v, want \"cancelled\"", err)
	}
}

func TestCallAISyncClaudeCodeNeedsNoKey(t *testing.T) {
	svc, rec := newTestService()
	cfg := settings.Default()
	cfg.ActiveProvider = "claude-code"

	if _, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "", "system", "user", documentSchema); err != nil {
		t.Fatalf("callAISync: %v", err)
	}
	if rec.provider != llm.ProviderClaudeCode {
		t.Errorf("provider = %q, want %q", rec.provider, llm.ProviderClaudeCode)
	}
	if rec.cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty — the user signed in to the CLI themselves", rec.cfg.APIKey)
	}
	if rec.client.gotRequest.Model != llm.DefaultModel(llm.ProviderClaudeCode, llm.FeaturePyramidize) {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, llm.DefaultModel(llm.ProviderClaudeCode, llm.FeaturePyramidize))
	}
	if llm.DefaultModel(llm.ProviderClaudeCode, llm.FeaturePyramidize) != "sonnet" {
		t.Errorf("llm.DefaultModel(llm.ProviderClaudeCode, llm.FeaturePyramidize) = %q, want the alias sonnet so the CLI picks the current generation", llm.DefaultModel(llm.ProviderClaudeCode, llm.FeaturePyramidize))
	}
}

// TestCallAISyncAsksForJSONEvenWithoutASchema covers the regression that came
// with the schema switch: every step here parses JSON, so OpenAI and Ollama must
// still be told to return an object when enforcement is off.
func TestCallAISyncAsksForJSONEvenWithoutASchema(t *testing.T) {
	original := schemaEnforcement
	t.Cleanup(func() { schemaEnforcement = original })
	schemaEnforcement = false

	svc, rec := newTestService()
	cfg := settings.Default()
	cfg.ActiveProvider = "openai"

	if _, err := svc.callAIWithContextForTest(t, cfg, "system", "user"); err != nil {
		t.Fatalf("call: %v", err)
	}
	req := rec.client.gotRequest
	if !req.JSONMode {
		t.Error("JSONMode must stay on: the pipeline parses every reply as JSON")
	}
	if len(req.JSONSchema) != 0 {
		t.Error("no schema may travel while enforcement is off")
	}

	schemaEnforcement = true
	svc2, rec2 := newTestService()
	if _, err := svc2.callAIWithContextForTest(t, cfg, "system", "user"); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(rec2.client.gotRequest.JSONSchema) == 0 {
		t.Error("the schema must travel while enforcement is on")
	}
}

// callAIWithContextForTest drives the same gate the pipeline uses.
func (svc *Service) callAIWithContextForTest(t *testing.T, cfg settings.Settings, system, user string) (string, error) {
	t.Helper()
	return svc.callAISync(context.Background(), cfg, aiOpts{}, "key", system, user, enforcedSchema(documentSchema))
}
