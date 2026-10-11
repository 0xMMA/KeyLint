package pyramidize

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

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
	// Temperature is pinned at 0 where the provider honours it: the judge
	// should return the same score for the same three texts, so that a
	// difference between runs is a difference in the pipeline.
	//
	// nil — written as null — where it cannot be pinned. The Claude Code CLI
	// has no temperature flag, and recording a 0 that never reached the model
	// would claim a repeatability the instrument does not have. null also keys
	// differently from 0 in eval-aggregate.sh, so a CLI-judged run can never be
	// compared as equal to an API-judged one.
	Temperature *float64 `json:"temperature"`
	// TemperatureNote says why Temperature is null. Empty when it is pinned.
	TemperatureNote string `json:"temperatureNote,omitempty"`
	// ResolvedModel is the model ID that actually answered, filled in after a
	// run from what the provider reported. For a full ID it should equal
	// Model; if it does not, the instrument was not what the config says.
	ResolvedModel string `json:"resolvedModel,omitempty"`
}

const (
	// defaultJudgeProvider and defaultJudgeModels pin the instrument. Changing
	// either invalidates every baseline recorded with the old one — which is
	// why they are fixed here rather than a fallback to whatever the pipeline
	// happens to use.
	//
	// The provider is the installed Claude Code CLI since 2026-10: every eval
	// call goes through the owner's subscription, not an API key. The price is
	// the temperature pin, which the CLI cannot set — see JudgeConfig. The
	// API-judged baselines before that date are a different instrument.
	defaultJudgeProvider = llm.ProviderClaudeCode
	judgeTemperature     = 0.0
)

// defaultJudgeModels are the judges that score every sample: two since
// 2026-10-11 (docs/roadmap.md, E3), reported separately and never averaged.
// A cheap judge and a strong one; whether both earn their place is an open
// question, and the set may shrink to one.
//
// Full model IDs, never aliases. #53 suspected the March baseline had drifted
// under an alias, and an alias is exactly what a measuring instrument must not
// be. Haiku 5.5 and Sonnet 5.5 have no dated snapshot behind them — the ID is
// the pin — so a run also records the ID that answered for each judge
// (resolvedModel), and the configKey keys on it.
//
// The pipeline under test still runs on the alias, because that is what users
// get. Only the judges are frozen.
var defaultJudgeModels = []string{"claude-haiku-5-5", "claude-sonnet-5-5"}

// judgeTemperatureNote is recorded wherever the judge's temperature is null.
const judgeTemperatureNote = "not pinned: the Claude Code CLI has no temperature flag, so the judge samples at the CLI default (and thinks first, as the CLI does by default)"

// judgeModelTemperatureNote is recorded for an API judge whose model takes no
// temperature (see llm.SupportsTemperature) — the 5.5 judges among them.
const judgeModelTemperatureNote = "not pinned: this model takes no temperature on the API (the 4.7 generation and later reject a non-default one), so the judge samples at the model default"

// JudgesFromEnv resolves the judge set, pinned unless a run deliberately
// overrides it. An override is recorded in summary.json like everything else,
// so a run judged by something else says so.
func JudgesFromEnv() ([]JudgeConfig, error) { return judgesFrom(os.Getenv) }

// judgesFrom reads the set: EVAL_JUDGES (comma-separated model IDs) chooses it
// — a bridge run adds the old claude-sonnet-4-5-20250929 as a third judge —
// and EVAL_JUDGE_MODEL, from before the set, names a single judge. All judges
// share EVAL_JUDGE_PROVIDER. The two model variables together are refused, as
// are an empty entry and a duplicate: each would record a set that is not the
// one that ran.
func judgesFrom(getenv func(string) string) ([]JudgeConfig, error) {
	models := defaultJudgeModels
	list := strings.TrimSpace(getenv("EVAL_JUDGES"))
	single := strings.TrimSpace(getenv("EVAL_JUDGE_MODEL"))
	switch {
	case list != "" && single != "":
		return nil, fmt.Errorf("EVAL_JUDGES and EVAL_JUDGE_MODEL are both set; use EVAL_JUDGES")
	case list != "":
		models = nil
		for _, m := range strings.Split(list, ",") {
			m = strings.TrimSpace(m)
			if m == "" {
				return nil, fmt.Errorf("EVAL_JUDGES %q has an empty entry", list)
			}
			if slices.Contains(models, m) {
				return nil, fmt.Errorf("EVAL_JUDGES names %q twice", m)
			}
			models = append(models, m)
		}
	case single != "":
		models = []string{single}
	}
	provider := judgeProviderFrom(getenv)
	judges := make([]JudgeConfig, len(models))
	for i, m := range models {
		judges[i] = newJudgeConfig(provider, m)
	}
	return judges, nil
}

// judgeProviderFrom is the provider every judge runs on.
func judgeProviderFrom(getenv func(string) string) string {
	if v := strings.TrimSpace(getenv("EVAL_JUDGE_PROVIDER")); v != "" {
		return v
	}
	return defaultJudgeProvider
}

func newJudgeConfig(provider, model string) JudgeConfig {
	cfg := JudgeConfig{Provider: provider, Model: model}
	switch {
	case llm.SupportsTemperature(provider, model):
		cfg.Temperature = llm.Temp(judgeTemperature)
	case provider == llm.ProviderClaudeCode:
		cfg.TemperatureNote = judgeTemperatureNote
	default:
		cfg.TemperatureNote = judgeModelTemperatureNote
	}
	return cfg
}

// TemperatureLabel is the judge's temperature for a log line: the number when
// pinned, "unpinned" when the provider cannot set one.
func (c JudgeConfig) TemperatureLabel() string {
	if c.Temperature == nil {
		return "unpinned (model default)"
	}
	return fmt.Sprintf("%.1f", *c.Temperature)
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

Mail history in the RAW INPUT (quoted replies, forwarded mails, earlier messages in the thread) is context for the author's own message. The CANDIDATE must not reproduce it; it may refer to facts from it. Leaving the history out is not a completeness flaw.

Score the CANDIDATE on these dimensions (0.0 to 1.0):

- pyramidStructure: Does it follow the Pyramid Principle? Main message first, then supporting details grouped logically. For emails: subject line contains the key message, action items, and stakeholders.
- clarity: Is the text clear, well-organized, and easy to scan? Are headers meaningful?
- completeness: Does it preserve ALL information from the author's own message? No facts dropped.
- tonePreservation: Does it match the tone and formality of the original? Does it preserve the original language (no unwanted translation)?
- overall: Your holistic assessment of quality (not just an average of above).

Respond with ONLY a JSON object:
{"pyramidStructure":0.0,"clarity":0.0,"completeness":0.0,"tonePreservation":0.0,"overall":0.0,"rationale":"Brief explanation"}`

// judgePromptHash names the judge's system prompt. summary.json records it and
// scripts/eval-aggregate.sh keys on it: the prompt is part of the instrument,
// like the judge's model, so runs scored under different wording read "not
// comparable". Hashed, not versioned, so no edit can slip through unbumped.
func judgePromptHash() string {
	sum := sha256.Sum256([]byte(judgeSystemPrompt))
	return "j1-" + hex.EncodeToString(sum[:8])
}

// judgeMaxTokens is the judge's own output limit. It equals what the judge
// sent while it borrowed the pipeline's limit (4096), and it is its own
// constant so that raising the pipeline's limit cannot move the instrument
// that measures the pipeline.
const judgeMaxTokens = 4096

// judgeTimeout bounds one judge call. Through the CLI a judge call takes about
// 30 s; this is a safety net against a hung process, not a budget.
const judgeTimeout = 180 * time.Second

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
	if llm.UsesAPIKey(judge.Provider) {
		apiKey = settingsSvc.GetKey(judge.Provider)
	}

	opts := aiOpts{
		noEffort: true,
		provider: judge.Provider,
		model:    judge.Model,
		// nil for the CLI, which could not honour it anyway.
		temperature: judge.Temperature,
		maxTokens:   judgeMaxTokens,
	}
	// callAISync has no deadline of its own (callAIWithContext adds the
	// pipeline's), and a CLI process that hangs would otherwise run into the
	// go test timeout, which panics before summary.json is written and throws
	// away every sample already paid for.
	ctx, cancel := context.WithTimeout(context.Background(), judgeTimeout)
	defer cancel()
	raw, err := svc.callAISync(ctx, cfg, opts, apiKey, judgeSystemPrompt, userMessage, judgeSchema)
	if err != nil {
		return JudgeScore{}, fmt.Errorf("judge AI call failed: %w", err)
	}

	var score JudgeScore
	if err := unmarshalRobust(raw, &score); err != nil {
		return JudgeScore{}, fmt.Errorf("judge parse error: %w (raw: %s)", err, raw)
	}
	return score, nil
}
