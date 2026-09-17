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

			raw, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "key", "system", "user")
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

	if _, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "sk-ant-test", "system", "user"); err != nil {
		t.Fatalf("callAISync: %v", err)
	}

	req := rec.client.gotRequest
	if req.System != "system" || req.User != "user" {
		t.Errorf("System/User = %q/%q, want system/user", req.System, req.User)
	}
	if !req.JSONMode {
		t.Error("JSONMode must be on — the pipeline parses JSON replies")
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
	if rec.cfg.Source != logSource {
		t.Errorf("Source = %q, want %q so debug logs name the feature", rec.cfg.Source, logSource)
	}
}

func TestCallAISyncOverrides(t *testing.T) {
	svc, rec := newTestService()
	cfg := settings.Default()
	cfg.ActiveProvider = "openai"

	opts := aiOpts{provider: "claude", model: "claude-opus-4-1"}
	if _, err := svc.callAISync(context.Background(), cfg, opts, "key", "system", "user"); err != nil {
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

	if _, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "", "system", "user"); err != nil {
		t.Fatalf("callAISync: %v", err)
	}
	if rec.cfg.BaseURL != "http://ollama.test:11434" {
		t.Errorf("BaseURL = %q, want the configured Ollama URL", rec.cfg.BaseURL)
	}
	if rec.cfg.PromptSeparator != ollamaPromptSeparator {
		t.Errorf("PromptSeparator = %q, want %q", rec.cfg.PromptSeparator, ollamaPromptSeparator)
	}
}

func TestCallAISyncUnsupportedProvider(t *testing.T) {
	svc, _ := newTestService()
	cfg := settings.Default()
	cfg.ActiveProvider = "bedrock"

	_, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "", "system", "user")
	if err == nil || !strings.Contains(err.Error(), `unsupported provider: "bedrock"`) {
		t.Fatalf("error = %v, want an unsupported-provider error", err)
	}
}

func TestCallAISyncPropagatesProviderError(t *testing.T) {
	svc, rec := newTestService()
	rec.client.err = errors.New("Claude error 429: rate limited")
	cfg := settings.Default()
	cfg.ActiveProvider = "claude"

	_, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "key", "system", "user")
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
	// This join differs from enhance's on purpose — it is the exact string the
	// hand-rolled callOllama built before internal/llm.
	if ollamaPromptSeparator != "\n\n---\n\n" {
		t.Errorf("ollamaPromptSeparator = %q, want %q", ollamaPromptSeparator, "\n\n---\n\n")
	}
}

// TestCallAISyncOllamaPromptJoin exercises the separator through the real
// provider client against an httptest server, not just as a Config field.
func TestCallAISyncOllamaPromptJoin(t *testing.T) {
	var gotPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		gotPrompt = payload.Prompt
		io.WriteString(w, `{"response":"{}"}`)
	}))
	t.Cleanup(srv.Close)

	svc := &Service{client: &http.Client{}, newClient: llm.New}
	cfg := settings.Default()
	cfg.ActiveProvider = "ollama"
	cfg.Providers.OllamaURL = srv.URL

	if _, err := svc.callAISync(context.Background(), cfg, aiOpts{}, "", "system", "user"); err != nil {
		t.Fatalf("callAISync: %v", err)
	}
	if want := "system\n\n---\n\nuser"; gotPrompt != want {
		t.Errorf("prompt = %q, want %q", gotPrompt, want)
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
