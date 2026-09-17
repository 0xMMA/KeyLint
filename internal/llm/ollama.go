package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// defaultOllamaBaseURL is where a local Ollama daemon listens.
const defaultOllamaBaseURL = "http://localhost:11434"

// defaultPromptSeparator joins system and user content when the caller does not
// specify one.
const defaultPromptSeparator = "\n\n"

// ollamaProvider carries the ID used in logs and the name used in errors.
var ollamaProvider = provider{id: ProviderOllama, name: "Ollama"}

type ollamaClient struct {
	cfg Config
}

func newOllama(cfg Config) Client { return &ollamaClient{cfg: cfg} }

func (c *ollamaClient) Complete(ctx context.Context, req Request) (Response, error) {
	if req.Model == "" {
		return Response{}, fmt.Errorf("%s: model is required", ollamaProvider.name)
	}

	// /api/generate takes a single "prompt" field, so system and user content
	// are combined with the caller's separator.
	separator := c.cfg.PromptSeparator
	if separator == "" {
		separator = defaultPromptSeparator
	}
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.System + separator + req.User,
		"stream": false,
	}

	url := resolveBaseURL(c.cfg.BaseURL, defaultOllamaBaseURL) + "/api/generate"
	body, err := postJSON(ctx, c.cfg, ollamaProvider, url, map[string]string{
		"Content-Type": "application/json",
	}, payload)
	if err != nil {
		return Response{}, err
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return Response{}, fmt.Errorf("%s unexpected response: %s", ollamaProvider.name, body)
	}
	return Response{Text: result.Response}, nil
}
