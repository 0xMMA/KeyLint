package pyramidize

import (
	"fmt"
	"net/http"
	"time"

	"keylint/internal/features/settings"
)

// JudgeScore holds the LLM-as-judge evaluation of one sample.
type JudgeScore struct {
	PyramidStructure float64 `json:"pyramidStructure"` // 0–1
	Clarity          float64 `json:"clarity"`           // 0–1
	Completeness     float64 `json:"completeness"`      // 0–1
	TonePreservation float64 `json:"tonePreservation"`  // 0–1
	Overall          float64 `json:"overall"`           // 0–1
	Rationale        string  `json:"rationale"`
}

const judgeSystemPrompt = `You are an expert evaluator of business document restructuring quality.
You will receive three texts:
1. RAW INPUT — the original unstructured text
2. BASELINE — a previous restructuring of the same input (for reference)
3. CANDIDATE — a new restructuring to evaluate

Score the CANDIDATE on these dimensions (0.0 to 1.0):

- pyramidStructure: Does it follow the Pyramid Principle? Main message first, then supporting details grouped logically. For emails: subject line contains the key message, action items, and stakeholders.
- clarity: Is the text clear, well-organized, and easy to scan? Are headers meaningful?
- completeness: Does it preserve ALL information from the raw input? No facts dropped.
- tonePreservation: Does it match the tone and formality of the original? Does it preserve the original language (no unwanted translation)?
- overall: Your holistic assessment of quality (not just an average of above).

Respond with ONLY a JSON object:
{"pyramidStructure":0.0,"clarity":0.0,"completeness":0.0,"tonePreservation":0.0,"overall":0.0,"rationale":"Brief explanation"}`

// RunJudge calls the LLM to evaluate a candidate output against the baseline.
func RunJudge(settingsSvc *settings.Service, opts aiOpts, rawInput, baseline, candidate string) (JudgeScore, error) {
	cfg := settingsSvc.Get()
	userMessage := fmt.Sprintf("<raw_input>\n%s\n</raw_input>\n\n<baseline>\n%s\n</baseline>\n\n<candidate>\n%s\n</candidate>",
		rawInput, baseline, candidate)

	// Resolve API key upfront so callAISync has no keyring dependency.
	provider := opts.provider
	if provider == "" {
		provider = cfg.ActiveProvider
	}
	apiKey := ""
	switch provider {
	case "openai":
		apiKey = settingsSvc.GetKey("openai")
	case "claude":
		apiKey = settingsSvc.GetKey("claude")
	}

	client := &http.Client{Timeout: 90 * time.Second}
	svc := &Service{client: client}

	raw, err := svc.callAISync(cfg, opts, apiKey, judgeSystemPrompt, userMessage)
	if err != nil {
		return JudgeScore{}, fmt.Errorf("judge AI call failed: %w", err)
	}

	var score JudgeScore
	if err := unmarshalRobust(raw, &score); err != nil {
		return JudgeScore{}, fmt.Errorf("judge parse error: %w (raw: %s)", err, raw)
	}
	return score, nil
}
