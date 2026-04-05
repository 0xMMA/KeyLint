package enhance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"keylint/internal/features/settings"
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

// Service calls AI provider APIs from Go so the Wails WebView does not need
// external network access (avoids WebKit content-security-policy issues on Linux).
type Service struct {
	settings *settings.Service
	client   *http.Client
}

// NewService creates an EnhanceService backed by the given settings.
func NewService(s *settings.Service) *Service {
	return &Service{settings: s, client: &http.Client{}}
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
	switch cfg.ActiveProvider {
	case "openai":
		key := s.settings.GetKey("openai")
		if key == "" {
			return "", fmt.Errorf("OpenAI API key is not configured. Go to Settings → AI Providers to add it")
		}
		return callOpenAI(s.client, text, key)
	case "claude":
		key := s.settings.GetKey("claude")
		if key == "" {
			return "", fmt.Errorf("Anthropic API key is not configured. Go to Settings → AI Providers → Anthropic API Key, and make sure 'Anthropic Claude' is selected as the Active Provider")
		}
		return callClaude(s.client, text, key)
	case "ollama":
		return callOllama(s.client, text, cfg.Providers.OllamaURL)
	case "bedrock":
		return "", fmt.Errorf("AWS Bedrock is not yet supported. Please select a different provider")
	default:
		return "", fmt.Errorf("unknown provider: %q", cfg.ActiveProvider)
	}
}

func callOpenAI(client *http.Client, text, apiKey string) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload, _ := json.Marshal(map[string]any{
		"model": "gpt-4o-mini",
		"messages": []msg{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
	})
	logger.Debug("enhance: request", "provider", "openai", "payload", logger.Redact(string(payload)))
	req, _ := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	logger.Debug("enhance: response", "provider", "openai", "status", resp.StatusCode, "body", logger.Redact(string(body)))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI error %d: %s", resp.StatusCode, body)
	}
	var result struct {
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil || len(result.Choices) == 0 {
		return "", fmt.Errorf("OpenAI unexpected response: %s", body)
	}
	return result.Choices[0].Message.Content, nil
}

func callClaude(client *http.Client, text, apiKey string) (string, error) {
	payload, _ := json.Marshal(map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 2048,
		"system":     systemPrompt,
		"messages":   []map[string]string{{"role": "user", "content": text}},
	})
	logger.Debug("enhance: request", "provider", "claude", "payload", logger.Redact(string(payload)))
	req, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Anthropic request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	logger.Debug("enhance: response", "provider", "claude", "status", resp.StatusCode, "body", logger.Redact(string(body)))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Claude error %d: %s", resp.StatusCode, body)
	}
	var result struct {
		Content []struct{ Text string } `json:"content"`
	}
	if err := json.Unmarshal(body, &result); err != nil || len(result.Content) == 0 {
		return "", fmt.Errorf("Claude unexpected response: %s", body)
	}
	return result.Content[0].Text, nil
}

func callOllama(client *http.Client, text, baseURL string) (string, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	payload, _ := json.Marshal(map[string]any{
		"model":  "llama3.2",
		"prompt": systemPrompt + "\n\nText: " + text,
		"stream": false,
	})
	logger.Debug("enhance: request", "provider", "ollama", "payload", logger.Redact(string(payload)))
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/generate", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	logger.Debug("enhance: response", "provider", "ollama", "status", resp.StatusCode, "body", logger.Redact(string(body)))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama error %d: %s", resp.StatusCode, body)
	}
	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("Ollama unexpected response: %s", body)
	}
	return result.Response, nil
}
