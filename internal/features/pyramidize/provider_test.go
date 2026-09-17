package pyramidize

import (
	"context"
	"errors"
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
}

func (f *fakeClient) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	f.gotRequest = req
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
	return &Service{newClient: rec.new}, rec
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
