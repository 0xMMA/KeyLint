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
		{"openai", openAIModel},
		{"claude", claudeModel},
		{"ollama", ollamaModel},
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

// TestPyramidizeWireConstantsUnchanged pins the literals that #33 step 1
// promised not to touch. The model assertions above only prove a constant was
// passed through; this one proves which.
func TestPyramidizeWireConstantsUnchanged(t *testing.T) {
	if openAIModel != "gpt-5.2" {
		t.Errorf("openAIModel = %q, want gpt-5.2", openAIModel)
	}
	if claudeModel != "claude-sonnet-4-6" {
		t.Errorf("claudeModel = %q, want claude-sonnet-4-6", claudeModel)
	}
	if ollamaModel != "llama3.2" {
		t.Errorf("ollamaModel = %q, want llama3.2", ollamaModel)
	}
	if maxTokens != 4096 {
		t.Errorf("maxTokens = %d, want 4096", maxTokens)
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

	settingsSvc, err := settings.NewService()
	if err != nil {
		t.Fatalf("settings.NewService: %v", err)
	}
	cfg := settings.Default()
	cfg.ActiveProvider = "ollama" // needs no API key, so the keyring stays out of it
	if err := settingsSvc.Save(cfg); err != nil {
		t.Fatalf("settings.Save: %v", err)
	}

	svc := NewService(settingsSvc, nil)
	rec := &recorder{client: &fakeClient{}}
	svc.newClient = rec.new
	// Cancel from inside the call, the way the UI's Cancel button does.
	rec.client.onComplete = svc.CancelOperation

	_, err = svc.RefineGlobal(RefineGlobalRequest{
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
	if rec.client.gotRequest.Model != claudeCodeModel {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, claudeCodeModel)
	}
	if claudeCodeModel != "sonnet" {
		t.Errorf("claudeCodeModel = %q, want the alias sonnet so the CLI picks the current generation", claudeCodeModel)
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
