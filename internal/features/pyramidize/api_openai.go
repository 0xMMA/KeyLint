package pyramidize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"keylint/internal/logger"
)

// callOpenAI sends a system + user message pair to the OpenAI chat completions API
// and returns the raw content string of the first choice. It requests JSON object
// output mode so the model is constrained to return valid JSON.
func callOpenAI(client *http.Client, systemPrompt, userMessage, apiKey, model string) (string, error) {
	if model == "" {
		model = "gpt-5.2"
	}
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload, err := json.Marshal(map[string]any{
		"model": model,
		"messages": []msg{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return "", fmt.Errorf("pyramidize/openai: marshal error: %w", err)
	}

	logger.Sensitive("pyramidize: openai request", "len", len(payload))

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("pyramidize/openai: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("pyramidize/openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	logger.Sensitive("pyramidize: openai response", "status", resp.StatusCode, "len", len(body))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pyramidize/openai: error %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil || len(result.Choices) == 0 {
		return "", fmt.Errorf("pyramidize/openai: unexpected response: %s", body)
	}
	return result.Choices[0].Message.Content, nil
}
