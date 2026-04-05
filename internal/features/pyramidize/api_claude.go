package pyramidize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"keylint/internal/logger"
)

// callClaude sends a system + user message pair to the Anthropic Messages API
// and returns the raw text of the first content block.
func callClaude(client *http.Client, systemPrompt, userMessage, apiKey, model string) (string, error) {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	payload, err := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 4096,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userMessage},
		},
	})
	if err != nil {
		return "", fmt.Errorf("pyramidize/claude: marshal error: %w", err)
	}

	logger.Debug("pyramidize: claude request", "payload", logger.Redact(string(payload)))

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("pyramidize/claude: build request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("pyramidize/claude: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	logger.Debug("pyramidize: claude response", "status", resp.StatusCode, "body", logger.Redact(string(body)))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pyramidize/claude: error %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &result); err != nil || len(result.Content) == 0 {
		return "", fmt.Errorf("pyramidize/claude: unexpected response: %s", body)
	}
	return result.Content[0].Text, nil
}
