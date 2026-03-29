# CLI + Evaluation Pipeline — Design Spec

**Date:** 2026-03-29
**Branch:** `feat/pyramidize`
**Status:** Approved

## Motivation

The Pyramidize feature's output quality has regressed compared to the archived Rust/LangChain variant. Improving prompts and evaluating changes requires a machine-testable pipeline — currently all testing is manual through the GUI. Two objectives must be resolved before this PR moves forward:

1. **Machine testability** — CLI commands that invoke the same service layer as the GUI, enabling fast prompt iteration and model comparison.
2. **Output quality** — measured improvement over the baseline test-data (generated with a weaker model), with LangChain as a fallback option if direct prompting can't reach the bar.

## Objective 1: CLI Interface

### Commands

| Command | Example | Description |
|---------|---------|-------------|
| `-fix` | `keylint -fix "thier going"` | Silent grammar fix (enhance service) |
| `-pyramidize` | `keylint -pyramidize -type email -f input.md` | Pyramidize with explicit doc type |
| `-pyramidize` | `cat email.md \| keylint -pyramidize` | Auto-detect, stdin input |
| `--json` | `keylint -pyramidize --json -f input.md` | Structured JSON output |
| `--provider`, `--model` | `keylint -pyramidize --provider claude --model claude-sonnet-4-6` | Override provider/model |

### Input Priority

1. If `-f path` is given → read file
2. Else if stdin is a pipe → read stdin
3. Else use trailing positional string argument
4. Error if none provided

### Output

- **Default:** Plain text to stdout (the `fullDocument` field)
- **`--json`:** Full `PyramidizeResult` struct as JSON (includes `qualityScore`, `qualityFlags`, `documentType`, `language`, `appliedRefinement`, etc.)
- Errors go to stderr
- Exit code 0 on success, 1 on error

### Prompt Parity

The CLI and GUI use the exact same service methods and prompts. No formatting instructions leak into the model's working prompts. Output formatting (text vs JSON) is purely a presentation concern of the caller.

### Package Structure

```
internal/cli/
  cli.go          — Run(args []string) error, flag parsing, dispatch
  pyramidize.go   — runPyramidize() — reads input, calls service, formats output
  fix.go          — runFix() — reads input, calls enhance service, prints result
```

### Integration with main.go

CLI dispatch happens before any Wails or `flag` package setup. `os.Args[1]` is checked directly:

```go
func main() {
    // CLI dispatch — before any Wails/flag setup
    if len(os.Args) > 1 {
        switch os.Args[1] {
        case "-fix", "-pyramidize":
            if err := cli.Run(os.Args[1:]); err != nil {
                fmt.Fprintf(os.Stderr, "error: %v\n", err)
                os.Exit(1)
            }
            os.Exit(0)
        }
    }

    // Existing GUI path continues below...
    simulateShortcut := flag.Bool("simulate-shortcut", false, "...")
    flag.Parse()
    // ...
}
```

### Service Wiring

CLI manually creates only the services it needs:

```go
settings, err := settings.NewService()
pyramidizeSvc := pyramidize.NewService(settings, nil) // nil clipboard — not needed for CLI
```

No Wire, no Wails. The `nil` clipboard is safe because `Pyramidize()` never touches clipboard — only `SendBack()` and `CaptureSourceApp()` do, which are GUI-only operations. These methods should return a clear error ("not available in CLI mode") when clipboard is nil, rather than panicking.

## Objective 2: Evaluation Framework

### Layer 1 — Deterministic Checks (fast gate)

- **Language preservation** — detected language of output matches input
- **Information coverage** — key entities/terms from input appear in output (token overlap ratio)
- **Structural checks** — has subject line (first line before `\n\n`), has markdown headers, section count > 1
- **No hallucination signals** — output doesn't introduce proper nouns absent from input

### Layer 2 — LLM-as-Judge

- Separate AI call receiving: raw input, baseline output (from test-data), new output
- Scores on dimensions: pyramid structure adherence, clarity, completeness, tone preservation
- Returns structured score (0–1 per dimension) + overall + brief rationale
- Uses the same provider infrastructure (`callAISync`) — can run against any model

### File Layout

```
internal/features/pyramidize/
  eval_test.go            — //go:build eval — Go integration tests
  eval_judge.go           — LLM-as-judge prompt + scoring logic (not build-tagged, reusable)
  eval_deterministic.go   — deterministic check functions

scripts/
  eval.sh                 — shell wrapper: runs `go test -tags eval`, formats results table
  eval-human.sh           — human review mode: 3-pane diff + manual scoring
```

### Build Tag Convention

Eval tests are gated behind the `eval` build tag to avoid accidental runs during normal development (they make real API calls and consume credits):

```go
//go:build eval

// To run evaluation tests (requires API keys, makes real AI calls):
//   go test -tags eval ./internal/features/pyramidize/ -v -timeout 300s
```

This convention will be documented in `CLAUDE.md` under Build/run/test.

### Test Data

Existing `test-data/pyramidal-emails/*.md` files (13 German-language email samples) serve as the baseline. Each has `# Raw Input` and `# User accepted output` sections. No changes to this format — quality criteria will emerge from the improvement process, not be designed upfront.

### Eval Run Logging

Every eval run (automated or human) writes a self-contained record to `test-data/eval-runs/<timestamp>/`:

```
test-data/eval-runs/
  2026-03-29T14-30-00/
    summary.json            — run metadata: provider, model, timestamp, overall scores
    results.jsonl           — per-sample: filename, automated scores, human score (if any), raw output snippet
    samples/
      email-airbyte-escalation.md    — full generated output text
      email-project-status.md
      ...
```

**Why:** Terminal output is ephemeral. Eval runs take time and API credits. The full generated text is preserved per-sample so any past run can be re-read or diffed without re-running.

Both `eval.sh` and `eval-human.sh` write to this same structure. Human scores merge into `results.jsonl` alongside automated scores.

`test-data/eval-runs/` is added to `.gitignore` by default. Individual runs can be committed with `git add -f` when worth preserving.

### Human Review Mode

`eval-human.sh` provides interactive evaluation:

1. Loops over each `test-data/pyramidal-emails/*.md` file
2. Runs `keylint -pyramidize -type email -f <file> --json`
3. Displays 3-pane terminal view:
   - Raw input (abbreviated, first ~20 lines)
   - Baseline (from `# User accepted output`)
   - New output (from CLI run)
4. Shows automated scores (deterministic + LLM-as-judge) below
5. Prompts: `Score (1-5), or [s]kip, [q]uit:`
6. Writes all results to the eval-run log folder

## Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| CLI architecture | Flag guard in `main.go` + `internal/cli/` package | Single binary, clean separation, no new dependencies |
| Input modes | File arg + stdin + inline string | Inline for quick one-liners, stdin/file for real content |
| Output format | Text default, `--json` flag | Pipe-friendly for humans, machine-parseable for eval |
| Prompt parity | CLI and GUI share identical prompts | Formatting is caller's concern, not the model's |
| Eval isolation | `//go:build eval` build tag | Go-idiomatic, invisible to normal test runs, no skip noise |
| Eval scoring | Deterministic fast gate + LLM-as-judge + optional human | Layered: cheap checks first, AI for subjective quality, human for fine-tuning |
| Eval logging | `test-data/eval-runs/<timestamp>/` with full output preservation | Runs are expensive and ephemeral terminal output is unreliable |
| Test data format | No changes | Quality criteria emerge from improvement work, not upfront design |
