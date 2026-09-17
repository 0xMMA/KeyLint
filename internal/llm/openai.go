package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// defaultOpenAIBaseURL is the public OpenAI API host. Any OpenAI-compatible
// endpoint can be reached by setting Config.BaseURL instead.
const defaultOpenAIBaseURL = "https://api.openai.com"

// openAIProvider carries the ID used in logs and the name used in errors.
var openAIProvider = provider{id: ProviderOpenAI, name: "OpenAI"}

type openAIClient struct {
	cfg Config
}

func newOpenAI(cfg Config) Client { return &openAIClient{cfg: cfg} }

func (c *openAIClient) Complete(ctx context.Context, req Request) (Response, error) {
	if req.Model == "" {
		return Response{}, fmt.Errorf("%s: model is required", openAIProvider.name)
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := map[string]any{
		"model": req.Model,
		"messages": []message{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
	}
	if req.JSONMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}

	url := resolveBaseURL(c.cfg.BaseURL, defaultOpenAIBaseURL) + "/v1/chat/completions"
	body, err := postJSON(ctx, c.cfg, openAIProvider, url, map[string]string{
		"Authorization": "Bearer " + c.cfg.APIKey,
		"Content-Type":  "application/json",
	}, payload)
	if err != nil {
		return Response{}, err
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil || len(result.Choices) == 0 {
		return Response{}, fmt.Errorf("%s unexpected response: %s", openAIProvider.name, body)
	}
	return Response{Text: result.Choices[0].Message.Content}, nil
}
