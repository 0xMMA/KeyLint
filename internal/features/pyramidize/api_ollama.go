package pyramidize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"keylint/internal/logger"
)

// callOllama sends a combined system+user prompt to the Ollama /api/generate endpoint
// and returns the raw response string. JSON output mode is not enforced at the API
// level for Ollama; unmarshalRobust handles any fence stripping needed.
func callOllama(client *http.Client, systemPrompt, userMessage, baseURL, model string) (string, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.2"
	}

	// Ollama /api/generate takes a single "prompt" field.
	// We combine system and user content with a clear separator.
	combinedPrompt := systemPrompt + "\n\n---\n\n" + userMessage

	payload, err := json.Marshal(map[string]any{
		"model":  model,
		"prompt": combinedPrompt,
		"stream": false,
	})
	if err != nil {
		return "", fmt.Errorf("pyramidize/ollama: marshal error: %w", err)
	}

	logger.Sensitive("pyramidize: ollama request", "len", len(payload))

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("pyramidize/ollama: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("pyramidize/ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	logger.Sensitive("pyramidize: ollama response", "status", resp.StatusCode, "len", len(body))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pyramidize/ollama: error %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("pyramidize/ollama: unexpected response: %s", body)
	}
	return result.Response, nil
}
