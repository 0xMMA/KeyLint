package enhance

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

// fakeClient is a stand-in for a provider client. It records the request it was
// given and returns a canned reply.
type fakeClient struct {
	gotRequest llm.Request
	reply      string
	err        error
	// inspectContext lets a test assert on the context Enhance passes down.
	inspectContext func(ctx context.Context)
}

func (f *fakeClient) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	f.gotRequest = req
	if f.inspectContext != nil {
		f.inspectContext(ctx)
	}
	if f.err != nil {
		return llm.Response{}, f.err
	}
	return llm.Response{Text: f.reply}, nil
}

// recorder captures what Enhance asked llm.New for.
type recorder struct {
	provider string
	cfg      llm.Config
	client   *fakeClient
	err      error
}

func (r *recorder) new(provider string, cfg llm.Config) (llm.Client, error) {
	r.provider = provider
	r.cfg = cfg
	if r.err != nil {
		return nil, r.err
	}
	return r.client, nil
}

// newTestService returns a Service whose settings live in a temp dir, whose keys
// come from keys instead of the OS keyring, and whose provider client is a fake.
func newTestService(t *testing.T, cfg settings.Settings, keys map[string]string) (*Service, *recorder) {
	t.Helper()

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
	if err := settingsSvc.Save(cfg); err != nil {
		t.Fatalf("settings.Save: %v", err)
	}

	rec := &recorder{client: &fakeClient{reply: "improved text"}}
	svc := NewService(settingsSvc)
	svc.newClient = rec.new
	svc.getKey = func(provider string) string { return keys[provider] }
	return svc, rec
}

func TestEnhanceOpenAI(t *testing.T) {
	cfg := settings.Default()
	cfg.ActiveProvider = "openai"
	svc, rec := newTestService(t, cfg, map[string]string{"openai": "sk-test"})

	got, err := svc.Enhance("their going to the meeting")
	if err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if got != "improved text" {
		t.Errorf("Enhance = %q, want %q", got, "improved text")
	}

	if rec.provider != llm.ProviderOpenAI {
		t.Errorf("provider = %q, want %q", rec.provider, llm.ProviderOpenAI)
	}
	if rec.cfg.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want sk-test", rec.cfg.APIKey)
	}
	if rec.cfg.HTTPClient != svc.client {
		t.Error("the service HTTP client must be handed to the provider client")
	}
	if rec.cfg.Feature != logFeature {
		t.Errorf("Feature = %q, want %q so the logs name the calling feature", rec.cfg.Feature, logFeature)
	}
	if rec.client.gotRequest.Model != openAIModel {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, openAIModel)
	}
	if rec.client.gotRequest.User != "their going to the meeting" {
		t.Errorf("User = %q, want the input text", rec.client.gotRequest.User)
	}
	if rec.client.gotRequest.System != systemPrompt {
		t.Error("System must be the enhance system prompt")
	}
	if rec.client.gotRequest.MaxTokens != maxTokens {
		t.Errorf("MaxTokens = %d, want %d", rec.client.gotRequest.MaxTokens, maxTokens)
	}
	if rec.client.gotRequest.JSONMode {
		t.Error("JSONMode must stay off for enhance")
	}
}

func TestEnhanceClaude(t *testing.T) {
	cfg := settings.Default()
	cfg.ActiveProvider = "claude"
	svc, rec := newTestService(t, cfg, map[string]string{"claude": "sk-ant-test"})

	if _, err := svc.Enhance("text"); err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if rec.provider != llm.ProviderClaude {
		t.Errorf("provider = %q, want %q", rec.provider, llm.ProviderClaude)
	}
	if rec.cfg.APIKey != "sk-ant-test" {
		t.Errorf("APIKey = %q, want sk-ant-test", rec.cfg.APIKey)
	}
	if rec.client.gotRequest.Model != claudeModel {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, claudeModel)
	}
}

func TestEnhanceOllamaNeedsNoKey(t *testing.T) {
	cfg := settings.Default()
	cfg.ActiveProvider = "ollama"
	cfg.Providers.OllamaURL = "http://ollama.test:11434"
	svc, rec := newTestService(t, cfg, nil)

	if _, err := svc.Enhance("text"); err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if rec.provider != llm.ProviderOllama {
		t.Errorf("provider = %q, want %q", rec.provider, llm.ProviderOllama)
	}
	if rec.cfg.BaseURL != "http://ollama.test:11434" {
		t.Errorf("BaseURL = %q, want the configured Ollama URL", rec.cfg.BaseURL)
	}
	if rec.cfg.PromptSeparator != ollamaPromptSeparator {
		t.Errorf("PromptSeparator = %q, want %q", rec.cfg.PromptSeparator, ollamaPromptSeparator)
	}
	if rec.client.gotRequest.Model != ollamaModel {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, ollamaModel)
	}
}

func TestEnhanceMissingKey(t *testing.T) {
	tests := []struct {
		provider string
		wantErr  string
	}{
		{"openai", "OpenAI API key is not configured"},
		{"claude", "Anthropic API key is not configured"},
	}
	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			cfg := settings.Default()
			cfg.ActiveProvider = tc.provider
			svc, rec := newTestService(t, cfg, nil)

			_, err := svc.Enhance("text")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Enhance error = %v, want one containing %q", err, tc.wantErr)
			}
			if rec.provider != "" {
				t.Error("no provider client must be built when the key is missing")
			}
		})
	}
}

func TestEnhanceUnsupportedProviders(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  string
	}{
		{"bedrock is a stub", "bedrock", "AWS Bedrock is not yet supported"},
		{"unknown provider", "mystery", `unknown provider: "mystery"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := settings.Default()
			cfg.ActiveProvider = tc.provider
			svc, _ := newTestService(t, cfg, nil)

			_, err := svc.Enhance("text")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Enhance error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestEnhancePropagatesProviderError(t *testing.T) {
	cfg := settings.Default()
	cfg.ActiveProvider = "ollama"
	svc, rec := newTestService(t, cfg, nil)
	rec.client.err = errors.New("Ollama error 404: model not found")

	_, err := svc.Enhance("text")
	if err == nil || !strings.Contains(err.Error(), "Ollama error 404") {
		t.Fatalf("Enhance error = %v, want the provider error", err)
	}
}

// TestEnhanceWireConstantsUnchanged pins the literals that #33 step 1 promised
// not to touch. Without this the model assertions elsewhere in this file only
// prove that a constant was passed through, not which one.
func TestEnhanceWireConstantsUnchanged(t *testing.T) {
	if openAIModel != "gpt-4o-mini" {
		t.Errorf("openAIModel = %q, want gpt-4o-mini", openAIModel)
	}
	if claudeModel != "claude-haiku-4-5-20251001" {
		t.Errorf("claudeModel = %q, want claude-haiku-4-5-20251001", claudeModel)
	}
	if ollamaModel != "llama3.2" {
		t.Errorf("ollamaModel = %q, want llama3.2", ollamaModel)
	}
	if maxTokens != 2048 {
		t.Errorf("maxTokens = %d, want 2048", maxTokens)
	}
	// The Ollama join differs from pyramidize's on purpose — it is the exact
	// string the hand-rolled callOllama built before internal/llm.
	if ollamaPromptSeparator != "\n\nText: " {
		t.Errorf("ollamaPromptSeparator = %q, want %q", ollamaPromptSeparator, "\n\nText: ")
	}
}

// TestEnhanceOllamaPromptJoin checks the separator end to end, through a real
// provider client against an httptest server, not just as a Config field.
func TestEnhanceOllamaPromptJoin(t *testing.T) {
	var gotPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		gotPrompt = payload.Prompt
		io.WriteString(w, `{"response":"ok"}`)
	}))
	t.Cleanup(srv.Close)

	cfg := settings.Default()
	cfg.ActiveProvider = "ollama"
	cfg.Providers.OllamaURL = srv.URL
	svc, _ := newTestService(t, cfg, nil)
	svc.newClient = llm.New // the real client, so the join is exercised for real

	if _, err := svc.Enhance("my text"); err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if want := systemPrompt + "\n\nText: " + "my text"; gotPrompt != want {
		t.Errorf("prompt = %q, want the system prompt joined by %q", gotPrompt, "\n\nText: ")
	}
}

func TestEnhanceClaudeCodeNeedsNoKey(t *testing.T) {
	cfg := settings.Default()
	cfg.ActiveProvider = "claude-code"
	svc, rec := newTestService(t, cfg, nil)

	if _, err := svc.Enhance("text"); err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if rec.provider != llm.ProviderClaudeCode {
		t.Errorf("provider = %q, want %q", rec.provider, llm.ProviderClaudeCode)
	}
	// The user signed in to the CLI themselves — KeyLint must not carry a key.
	if rec.cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", rec.cfg.APIKey)
	}
	if rec.client.gotRequest.Model != claudeCodeModel {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, claudeCodeModel)
	}
	if claudeCodeModel != "haiku" {
		t.Errorf("claudeCodeModel = %q, want the alias haiku so the CLI picks the current generation", claudeCodeModel)
	}
}

func TestEnhanceBoundsTheCall(t *testing.T) {
	// A dead provider must not hang the silent-fix hotkey: the HTTP client has
	// its own timeout and the whole call carries a deadline, which is the only
	// bound a local CLI provider gets.
	cfg := settings.Default()
	cfg.ActiveProvider = "claude-code"
	svc, rec := newTestService(t, cfg, nil)

	var deadlineSet bool
	rec.client.inspectContext = func(ctx context.Context) {
		_, deadlineSet = ctx.Deadline()
	}
	if _, err := svc.Enhance("text"); err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if !deadlineSet {
		t.Error("Complete was called with a context that has no deadline")
	}
	if svc.client.Timeout == 0 {
		t.Error("the HTTP client has no timeout")
	}
}
