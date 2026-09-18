package pyramidize

import (
	"context"
	"fmt"
	"os"
	"strings"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// JudgeConfig is what the judge runs with.
//
// Deliberately separate from the pipeline's provider and model. The pipeline is
// what an eval changes; the judge is the instrument measuring it, and an
// instrument that moves with the thing it measures reports nothing. A Sonnet
// pipeline and an Opus pipeline are scored by the same judge or their numbers
// cannot be put in the same table.
type JudgeConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// Temperature is pinned at 0: the judge should return the same score for
	// the same three texts, so that a difference between runs is a difference
	// in the pipeline.
	Temperature float64 `json:"temperature"`
}

const (
	// defaultJudgeProvider and defaultJudgeModel pin the instrument. Changing
	// either invalidates every baseline recorded with the old one — which is
	// why they are constants here rather than a fallback to whatever the
	// pipeline happens to use.
	//
	// The model is a DATED snapshot on purpose. #53 suspected the March
	// baseline had drifted under an alias, and an alias is exactly what a
	// measuring instrument must not be: claude-sonnet-4-6 is what the account
	// lists, with no dated form behind it, so anything judged by that alias can
	// change without a commit. claude-sonnet-4-5-20250929 cannot.
	//
	// The pipeline under test still runs on the alias, because that is what
	// users get. Only the judge is frozen.
	defaultJudgeProvider = "claude"
	defaultJudgeModel    = "claude-sonnet-4-5-20250929"
	judgeTemperature     = 0.0
)

// JudgeConfigFromEnv resolves the judge's configuration, pinned unless a run
// deliberately overrides it. An override is recorded in summary.json like
// everything else, so a run judged by something else says so.
func JudgeConfigFromEnv() JudgeConfig {
	cfg := JudgeConfig{
		Provider:    defaultJudgeProvider,
		Model:       defaultJudgeModel,
		Temperature: judgeTemperature,
	}
	if v := strings.TrimSpace(os.Getenv("EVAL_JUDGE_PROVIDER")); v != "" {
		cfg.Provider = v
	}
	if v := strings.TrimSpace(os.Getenv("EVAL_JUDGE_MODEL")); v != "" {
		cfg.Model = v
	}
	return cfg
}

// JudgeScore holds the LLM-as-judge evaluation of one sample.
type JudgeScore struct {
	PyramidStructure float64 `json:"pyramidStructure"` // 0–1
	Clarity          float64 `json:"clarity"`          // 0–1
	Completeness     float64 `json:"completeness"`     // 0–1
	TonePreservation float64 `json:"tonePreservation"` // 0–1
	Overall          float64 `json:"overall"`          // 0–1
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

// runJudge calls the LLM to evaluate a candidate output against the baseline.
//
// The judge always sends its schema, whatever KEYLINT_PYRAMIDIZE_SCHEMA says.
// That flag is a property of the pipeline under test; letting it reach the judge
// would mean a --schema run and a plain run were scored by differently
// constrained instruments, and comparing those two configurations is the only
// reason the flag exists. The judge's schema constrains five numbers and a
// sentence — it cannot flatter the candidate, only make the reply parse.
// Unexported: Service is registered with Wails, and every exported method on it
// becomes callable from the webview. The judge spends the user's API key on a
// scoring call the product has no use for, so it must not be part of that
// surface — see bindings_test.go, which fails if this set drifts again.
func (svc *Service) runJudge(settingsSvc *settings.Service, judge JudgeConfig, rawInput, baseline, candidate string) (JudgeScore, error) {
	cfg := settingsSvc.Get()
	// Fixed order, every time: the judge reads three texts, and reordering them
	// between runs would change the scores for reasons that have nothing to do
	// with the pipeline.
	userMessage := fmt.Sprintf("<raw_input>\n%s\n</raw_input>\n\n<baseline>\n%s\n</baseline>\n\n<candidate>\n%s\n</candidate>",
		rawInput, baseline, candidate)

	// Resolve API key upfront so callAISync has no keyring dependency.
	apiKey := ""
	switch judge.Provider {
	case "openai", "claude":
		apiKey = settingsSvc.GetKey(judge.Provider)
	}

	opts := aiOpts{
		provider:    judge.Provider,
		model:       judge.Model,
		temperature: llm.Temp(judge.Temperature),
	}
	raw, err := svc.callAISync(context.Background(), cfg, opts, apiKey, judgeSystemPrompt, userMessage, judgeSchema)
	if err != nil {
		return JudgeScore{}, fmt.Errorf("judge AI call failed: %w", err)
	}

	var score JudgeScore
	if err := unmarshalRobust(raw, &score); err != nil {
		return JudgeScore{}, fmt.Errorf("judge parse error: %w (raw: %s)", err, raw)
	}
	return score, nil
}
