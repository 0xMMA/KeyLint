//go:build eval

package enhance

import (
	"context"
	"fmt"
	"strings"

	"keylint/internal/features/settings"
	"keylint/internal/llm"
)

// The judge is asked only about what the deterministic checks cannot decide.
// Whether a comma is in the right place is decidable; whether the author still
// sounds like themselves is not.
const judgeSystemPrompt = `You are evaluating a grammar and spelling correction, not writing one.

You receive three texts:
1. ORIGINAL — what the author wrote, errors and all
2. REFERENCE — one acceptable correction, written by a human
3. CANDIDATE — the correction under evaluation

The reference is ONE acceptable answer, not the only one. Do not punish the
candidate for differing from it where both are correct.

Score each dimension from 0.0 to 1.0:

- correctness: are the actual errors in the ORIGINAL fixed? Spelling, grammar,
  punctuation, capitalisation. Count errors left behind, and count NEW errors
  introduced — a candidate that fixes four things and breaks one is not perfect.
- meaningPreserved: does it say the same thing? No fact added, dropped or
  altered. A changed number, name, date or negation is a serious failure here.
- tonePreserved: does the author still sound like themselves? Register,
  formality, greetings, hedges, emoji, clipped sentences. Formalising a casual
  message is a failure even when every word is correct.
- noOverEditing: did it restrict itself to correcting? Rewriting a correct
  sentence for style, reordering content, adding or removing information, or
  translating text that should have stayed is over-editing. A candidate
  identical to a correct ORIGINAL scores 1.0 here.

Respond with ONLY a JSON object:
{"correctness":0.0,"meaningPreserved":0.0,"tonePreserved":0.0,"noOverEditing":0.0,"overall":0.0,"rationale":"one or two sentences"}

overall is your holistic judgement, not the mean.`

// JudgeConfig pins the instrument, exactly as the Pyramidize eval does — a
// judge that moves with what it measures reports nothing. Same dated snapshot,
// so the two suites' judge columns are at least produced the same way.
type JudgeConfig struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
}

const (
	defaultJudgeProvider = "claude"
	defaultJudgeModel    = "claude-sonnet-4-5-20250929"
	judgeTemperature     = 0.0
)

// JudgeConfigFromEnv resolves the judge, pinned unless deliberately overridden.
func JudgeConfigFromEnv(getenv func(string) string) JudgeConfig {
	cfg := JudgeConfig{Provider: defaultJudgeProvider, Model: defaultJudgeModel, Temperature: judgeTemperature}
	if v := strings.TrimSpace(getenv("EVAL_JUDGE_PROVIDER")); v != "" {
		cfg.Provider = v
	}
	if v := strings.TrimSpace(getenv("EVAL_JUDGE_MODEL")); v != "" {
		cfg.Model = v
	}
	return cfg
}

// RunJudge scores one candidate. The three texts are always in the same order,
// because reordering them would move the scores for reasons that have nothing
// to do with the prompt under test.
func RunJudge(settingsSvc *settings.Service, judge JudgeConfig, original, reference, candidate string) (JudgeScore, error) {
	apiKey := settingsSvc.GetKey(judge.Provider)
	client, err := llm.New(judge.Provider, llm.Config{APIKey: apiKey, Feature: "eval-judge"})
	if err != nil {
		return JudgeScore{}, err
	}

	user := fmt.Sprintf("<original>\n%s\n</original>\n\n<reference>\n%s\n</reference>\n\n<candidate>\n%s\n</candidate>",
		original, reference, candidate)

	resp, err := client.Complete(context.Background(), llm.Request{
		System:      judgeSystemPrompt,
		User:        user,
		Model:       judge.Model,
		MaxTokens:   1024,
		JSONSchema:  fixJudgeSchema,
		Temperature: llm.Temp(judge.Temperature),
	})
	if err != nil {
		return JudgeScore{}, fmt.Errorf("judge call failed: %w", err)
	}

	score, err := parseJudgeScore(resp.Text)
	if err != nil {
		return JudgeScore{}, fmt.Errorf("%w (raw: %s)", err, resp.Text)
	}
	return score, nil
}
