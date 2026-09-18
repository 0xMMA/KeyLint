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

	// Built from an explicit config: no settings file to read, no keyring to
	// reach, so this test measures the code and not the machine.
	settingsSvc := settings.NewServiceFrom(cfg, settings.EnvOnlyKeys)

	// A plausible correction of the input the tests send, not an unrelated
	// string: Enhance now runs its output guard, and a reply that shares
	// nothing with the input is refused on purpose.
	rec := &recorder{client: &fakeClient{reply: fakeCorrection}}
	svc := NewService(settingsSvc)
	svc.newClient = rec.new
	svc.getKey = func(provider string) string { return keys[provider] }
	return svc, rec
}

// fakeInput and fakeCorrection are what the plumbing tests send and expect back.
const (
	fakeInput      = "their going to the meeting later and i think its about the new project"
	fakeCorrection = "They're going to the meeting later, and I think it's about the new project."
)

func TestEnhanceOpenAI(t *testing.T) {
	cfg := settings.Default()
	cfg.ActiveProvider = "openai"
	svc, rec := newTestService(t, cfg, map[string]string{"openai": "sk-test"})

	got, err := svc.Enhance(fakeInput)
	if err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if got != fakeCorrection {
		t.Errorf("Enhance = %q, want %q", got, fakeCorrection)
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
	if rec.client.gotRequest.Model != llm.DefaultModel(llm.ProviderOpenAI, llm.FeatureFix) {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, llm.DefaultModel(llm.ProviderOpenAI, llm.FeatureFix))
	}
	if rec.client.gotRequest.User != buildUserMessage(fakeInput) {
		t.Errorf("User = %q, want the input text between the markers", rec.client.gotRequest.User)
	}
	if rec.client.gotRequest.System != systemPrompt {
		t.Error("System must be the enhance system prompt")
	}
	if rec.client.gotRequest.MaxTokens != maxTokens {
		t.Errorf("MaxTokens = %d, want %d", rec.client.gotRequest.MaxTokens, maxTokens)
	}
	if len(rec.client.gotRequest.JSONSchema) != 0 {
		t.Error("enhance asks for prose, so it must not constrain the reply to a schema")
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
	if rec.client.gotRequest.Model != llm.DefaultModel(llm.ProviderClaude, llm.FeatureFix) {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, llm.DefaultModel(llm.ProviderClaude, llm.FeatureFix))
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
	if rec.client.gotRequest.Model != llm.DefaultModel(llm.ProviderOllama, llm.FeatureFix) {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, llm.DefaultModel(llm.ProviderOllama, llm.FeatureFix))
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

// TestEnhanceDefaultsUnchanged pins the models the fix flow shipped with.
// Moving them into settings must not move them: changing a default is a quality
// decision that belongs with E3 (#34) and needs an eval run.
func TestEnhanceDefaultsUnchanged(t *testing.T) {
	want := map[string]string{
		llm.ProviderOpenAI:     "gpt-4o-mini",
		llm.ProviderClaude:     "claude-haiku-4-5-20251001",
		llm.ProviderOllama:     "llama3.2",
		llm.ProviderClaudeCode: "haiku",
	}
	for provider, model := range want {
		if got := llm.DefaultModel(provider, llm.FeatureFix); got != model {
			t.Errorf("default fix model for %s = %q, want %q", provider, got, model)
		}
	}
	if maxTokens != 2048 {
		t.Errorf("maxTokens = %d, want 2048", maxTokens)
	}
}

// TestEnhanceUsesTheConfiguredModel covers the point of #33 step 4: a model
// chosen in settings reaches the provider.
func TestEnhanceUsesTheConfiguredModel(t *testing.T) {
	cfg := settings.Default()
	cfg.ActiveProvider = "openai"
	cfg.Models = map[string]settings.FeatureModels{
		"openai": {Fix: "gpt-4.1-mini", Pyramidize: "gpt-5.2-pro"},
	}
	svc, rec := newTestService(t, cfg, map[string]string{"openai": "sk-test"})

	if _, err := svc.Enhance("text"); err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	// Also covers the two features not leaking into each other: the same
	// provider has a different model configured for Pyramidize.
	if got := rec.client.gotRequest.Model; got != "gpt-4.1-mini" {
		t.Errorf("Model = %q, want the configured fix model (the Pyramidize one is gpt-5.2-pro)", got)
	}
}

// TestEnhanceOllamaSendsASystemMessage exercises the real provider client
// against an httptest server. Ollama used to take one glued-together prompt
// string; through its OpenAI-compatible endpoint the system prompt is a system
// message the model can weigh as such.
func TestEnhanceOllamaSendsASystemMessage(t *testing.T) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	var gotMessages []message
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var payload struct {
			Messages []message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		gotMessages = payload.Messages
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	t.Cleanup(srv.Close)

	cfg := settings.Default()
	cfg.ActiveProvider = "ollama"
	cfg.Providers.OllamaURL = srv.URL
	svc, _ := newTestService(t, cfg, nil)
	svc.newClient = llm.New // the real client, so the wire format is exercised

	if _, err := svc.Enhance("my text"); err != nil {
		t.Fatalf("Enhance: %v", err)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q, want the OpenAI-compatible endpoint", gotPath)
	}
	if len(gotMessages) != 2 {
		t.Fatalf("messages = %v, want a system and a user message", gotMessages)
	}
	if gotMessages[0].Role != "system" || gotMessages[0].Content != systemPrompt {
		t.Errorf("first message = %+v, want the enhance system prompt in a system role", gotMessages[0])
	}
	if gotMessages[1].Role != "user" || gotMessages[1].Content != buildUserMessage("my text") {
		t.Errorf("second message = %+v, want the user text between the markers", gotMessages[1])
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
	if rec.client.gotRequest.Model != llm.DefaultModel(llm.ProviderClaudeCode, llm.FeatureFix) {
		t.Errorf("Model = %q, want %q", rec.client.gotRequest.Model, llm.DefaultModel(llm.ProviderClaudeCode, llm.FeatureFix))
	}
	if llm.DefaultModel(llm.ProviderClaudeCode, llm.FeatureFix) != "haiku" {
		t.Errorf("llm.DefaultModel(llm.ProviderClaudeCode, llm.FeatureFix) = %q, want the alias haiku so the CLI picks the current generation", llm.DefaultModel(llm.ProviderClaudeCode, llm.FeatureFix))
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
