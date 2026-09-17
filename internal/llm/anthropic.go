package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// defaultAnthropicBaseURL is the public Anthropic API host.
const defaultAnthropicBaseURL = "https://api.anthropic.com"

// anthropicVersion is the API version header every request must carry.
const anthropicVersion = "2023-06-01"

// anthropicName prefixes errors surfaced to the user. It says "Claude" because
// that is the provider name the UI uses.
const anthropicName = "Claude"

type anthropicClient struct {
	cfg Config
}

func newAnthropic(cfg Config) Client { return &anthropicClient{cfg: cfg} }

func (c *anthropicClient) Complete(ctx context.Context, req Request) (Response, error) {
	if req.Model == "" {
		return Response{}, fmt.Errorf("%s: model is required", anthropicName)
	}
	if req.MaxTokens <= 0 {
		return Response{}, fmt.Errorf("%s: max_tokens is required", anthropicName)
	}

	payload := map[string]any{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		"system":     req.System,
		"messages": []map[string]string{
			{"role": "user", "content": req.User},
		},
	}

	url := resolveBaseURL(c.cfg.BaseURL, defaultAnthropicBaseURL) + "/v1/messages"
	body, err := postJSON(ctx, c.cfg, anthropicName, url, map[string]string{
		"x-api-key":         c.cfg.APIKey,
		"anthropic-version": anthropicVersion,
		"Content-Type":      "application/json",
	}, payload)
	if err != nil {
		return Response{}, err
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &result); err != nil || len(result.Content) == 0 {
		return Response{}, fmt.Errorf("%s unexpected response: %s", anthropicName, body)
	}
	return Response{Text: result.Content[0].Text}, nil
}
