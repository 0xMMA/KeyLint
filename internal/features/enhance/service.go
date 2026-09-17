package enhance

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
	"keylint/internal/logger"
)

const systemPrompt = `You are a grammar, spelling, and clarity correction assistant. Your task is to fix grammatical errors, spelling mistakes, and improve clarity in text while preserving the original meaning, tone, and intent.

**Rules:**
1. Correct all grammar and spelling errors
2. Preserve the original meaning and factual content exactly
3. Maintain the author's voice, tone, and perspective
4. Keep the original language - never translate the full text
5. Improve sentence structure only when necessary for clarity
6. Make direct corrections without explanations, comments, or questions
7. Focus on making the text more professional and readable while keeping it authentic
8. Multilingual authors naturally blend languages as they think and write.
   When a word or short phrase appears in a different language than the dominant
   language of the text, apply this logic:
   - Would the target audience immediately understand this word as-is? → Leave it unchanged
   - Would translating it genuinely help the reader understand better? → Replace it with
     the contextually appropriate equivalent
   This is not an error — it reflects how multilingual minds naturally reach for the
   nearest available word across languages.

**Examples:**

Input:  "their going to the meeting later and i think its going to be about the new project we discussed yesterday"
Output: "They're going to the meeting later, and I think it's going to be about the new project we discussed yesterday."

Input:  "Please send me the Rechnung for last month"
Output: "Please send me the invoice for last month."

Input:  "We need to review the whole Ablaufplan before the launch"
Output: "We need to review the whole workflow before the launch."

Input:  "The meeting is mañana at 9am"
Output: "The meeting is tomorrow at 9am."

Input:  "Hallo Hans, das release für morgen steht, einen neuen build brauchen wir nicht, einfach redeploy, hab die Klasse CarService gefixt"
Output: "Hallo Hans, das Release für morgen steht, einen neuen Build brauchen wir nicht, einfach redeploy, hab die Klasse CarService gefixt."`

// Model IDs and limits for the fix/enhance flow. They stay at the call site
// until model selection moves into settings (#33 step 4).
const (
	openAIModel = "gpt-4o-mini"
	claudeModel = "claude-haiku-4-5-20251001"
	ollamaModel = "llama3.2"
	// claudeCodeModel is a CLI alias, not a pinned ID — the CLI resolves it to
	// the current generation, which is what a subscription user expects.
	claudeCodeModel = "haiku"
	maxTokens       = 2048
)

// httpTimeout bounds a provider HTTP call, and enhanceTimeout bounds the whole
// enhancement including a local CLI provider, which has no HTTP client to bound
// it. Without either, a dead provider hangs the silent-fix hotkey forever.
const (
	httpTimeout    = 60 * time.Second
	enhanceTimeout = 90 * time.Second
)

// logFeature tags this feature's provider calls in the log.
const logFeature = "enhance"

// ollamaPromptSeparator reproduces the exact system/user join this flow used
// before internal/llm existed — Ollama's /api/generate takes a single prompt.
const ollamaPromptSeparator = "\n\nText: "

// Service calls AI provider APIs from Go so the Wails WebView does not need
// external network access (avoids WebKit content-security-policy issues on Linux).
type Service struct {
	settings *settings.Service
	client   *http.Client
	// newClient builds the provider client. Tests replace it with a fake.
	newClient func(provider string, cfg llm.Config) (llm.Client, error)
	// getKey resolves a provider API key. Tests replace it so they never touch
	// the OS keyring.
	getKey func(provider string) string
}

// NewService creates an EnhanceService backed by the given settings.
func NewService(s *settings.Service) *Service {
	return &Service{
		settings:  s,
		client:    &http.Client{Timeout: httpTimeout},
		newClient: llm.New,
		getKey:    s.GetKey,
	}
}

// Enhance sends text to the configured AI provider and returns the improved version.
func (s *Service) Enhance(text string) (result string, err error) {
	cfg := s.settings.Get()
	logger.Info("enhance: start", "provider", cfg.ActiveProvider, "input_len", len(text))
	defer func() {
		if err != nil {
			logger.Error("enhance: failed", "provider", cfg.ActiveProvider, "err", err)
		} else {
			logger.Info("enhance: done", "provider", cfg.ActiveProvider, "output_len", len(result))
		}
	}()

	clientCfg, model, err := s.providerConfig(cfg)
	if err != nil {
		return "", err
	}

	newClient := s.newClient
	if newClient == nil {
		newClient = llm.New
	}
	client, err := newClient(cfg.ActiveProvider, clientCfg)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), enhanceTimeout)
	defer cancel()

	resp, err := client.Complete(ctx, llm.Request{
		System:    systemPrompt,
		User:      text,
		Model:     model,
		MaxTokens: maxTokens,
	})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// providerConfig resolves credentials, endpoint and model ID for the active
// provider. It is the only place in this package that knows provider IDs.
func (s *Service) providerConfig(cfg settings.Settings) (llm.Config, string, error) {
	switch cfg.ActiveProvider {
	case llm.ProviderOpenAI:
		key := s.resolveKey(llm.ProviderOpenAI)
		if key == "" {
			return llm.Config{}, "", fmt.Errorf("OpenAI API key is not configured. Go to Settings → AI Providers to add it")
		}
		return llm.Config{APIKey: key, HTTPClient: s.client, Feature: logFeature}, openAIModel, nil
	case llm.ProviderClaude:
		key := s.resolveKey(llm.ProviderClaude)
		if key == "" {
			return llm.Config{}, "", fmt.Errorf("Anthropic API key is not configured. Go to Settings → AI Providers → Anthropic API Key, and make sure 'Anthropic Claude' is selected as the Active Provider")
		}
		return llm.Config{APIKey: key, HTTPClient: s.client, Feature: logFeature}, claudeModel, nil
	case llm.ProviderOllama:
		return llm.Config{
			BaseURL:         cfg.Providers.OllamaURL,
			HTTPClient:      s.client,
			PromptSeparator: ollamaPromptSeparator,
			Feature:         logFeature,
		}, ollamaModel, nil
	case llm.ProviderClaudeCode:
		// The user signed in to the CLI themselves; KeyLint needs no key and
		// never touches their credentials.
		return llm.Config{Feature: logFeature}, claudeCodeModel, nil
	case "bedrock":
		return llm.Config{}, "", fmt.Errorf("AWS Bedrock is not yet supported. Please select a different provider")
	default:
		return llm.Config{}, "", fmt.Errorf("unknown provider: %q", cfg.ActiveProvider)
	}
}

// resolveKey reads a provider API key, falling back to settings when the test
// seam is unset — the same nil-safety Enhance applies to newClient.
func (s *Service) resolveKey(provider string) string {
	if s.getKey != nil {
		return s.getKey(provider)
	}
	return s.settings.GetKey(provider)
}
