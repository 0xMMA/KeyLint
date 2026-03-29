# CLI + Evaluation Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CLI commands (`-fix`, `-pyramidize`) to the KeyLint binary and an evaluation framework for measuring pyramidize output quality against baseline test-data.

**Architecture:** Flag guard in `main.go` dispatches to `internal/cli/` before Wails boots. The CLI package manually wires only the services it needs (no Wire, no Wails). Evaluation lives in build-tagged Go tests (`//go:build eval`) alongside shell scripts for automated and human review modes.

**Tech Stack:** Go 1.26 stdlib (`flag`, `os`, `encoding/json`), existing `internal/features/` services, bash scripts for eval harness.

**Spec:** `docs/superpowers/specs/2026-03-29-cli-eval-design.md`

---

## File Structure

```
internal/cli/
  cli.go              — Run(args) dispatch, input reading helper
  cli_test.go         — unit tests for arg parsing and input reading
  pyramidize.go       — runPyramidize command handler
  pyramidize_test.go  — unit tests for pyramidize CLI output formatting
  fix.go              — runFix command handler
  fix_test.go         — unit tests for fix CLI output formatting

internal/features/pyramidize/
  eval_deterministic.go      — deterministic scoring functions
  eval_deterministic_test.go — unit tests for deterministic checks
  eval_judge.go              — LLM-as-judge prompt + scoring
  eval_test.go               — //go:build eval — integration tests against test-data

scripts/
  eval.sh             — automated eval runner (wraps go test -tags eval)
  eval-human.sh       — interactive human review mode
```

**Modified files:**
- `main.go` — CLI dispatch guard before Wails boot
- `internal/features/pyramidize/service.go` — nil-clipboard guard on SendBack
- `.gitignore` — add `test-data/eval-runs/`
- `CLAUDE.md` — document eval build tag and CLI commands

---

### Task 1: CLI Dispatch in main.go

**Files:**
- Modify: `main.go:34-37`
- Create: `internal/cli/cli.go`
- Create: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the failing test for CLI dispatch**

Create `internal/cli/cli_test.go`:

```go
package cli

import (
	"bytes"
	"testing"
)

func TestRunUnknownCommand(t *testing.T) {
	var stderr bytes.Buffer
	err := Run([]string{"-unknown"}, nil, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if err.Error() != `unknown command: "-unknown"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunNoArgs(t *testing.T) {
	err := Run(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty args")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -run TestRun`
Expected: Compilation failure — package `cli` does not exist yet.

- [ ] **Step 3: Write minimal cli.go implementation**

Create `internal/cli/cli.go`:

```go
package cli

import (
	"fmt"
	"io"
)

// Run dispatches a CLI command. args[0] is the command name ("-fix" or "-pyramidize").
// stdout receives the command output; stderr receives error messages.
func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("no command provided")
	}

	switch args[0] {
	case "-fix":
		return runFix(args[1:], stdout, stderr)
	case "-pyramidize":
		return runPyramidize(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command: %q", args[0])
	}
}
```

Create stub `internal/cli/fix.go`:

```go
package cli

import (
	"fmt"
	"io"
)

func runFix(args []string, stdout io.Writer, stderr io.Writer) error {
	return fmt.Errorf("not implemented")
}
```

Create stub `internal/cli/pyramidize.go`:

```go
package cli

import (
	"fmt"
	"io"
)

func runPyramidize(args []string, stdout io.Writer, stderr io.Writer) error {
	return fmt.Errorf("not implemented")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -run TestRun`
Expected: PASS

- [ ] **Step 5: Wire CLI dispatch into main.go**

Modify `main.go` — add the CLI guard at the very top of `main()`, before the existing `simulateShortcut` flag parsing:

```go
func main() {
	// CLI dispatch — runs headlessly, no Wails/GUI.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-fix", "-pyramidize":
			if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	simulateShortcut := flag.Bool("simulate-shortcut", false, "Fire a synthetic shortcut event on startup (Linux dev mode)")
	flag.Parse()
	// ... rest unchanged
```

Add `"keylint/internal/cli"` to the imports, and add `"fmt"` and `"os"` if not already present (`os` is not currently imported in main.go).

- [ ] **Step 6: Verify the build compiles**

Run: `cd /home/dev/projects/keylint && go build -o bin/KeyLint .`
Expected: Compiles cleanly.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go internal/cli/fix.go internal/cli/pyramidize.go main.go
git commit -m "feat(cli): add CLI dispatch skeleton with fix and pyramidize stubs"
```

---

### Task 2: Input Reading Helper

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

The input helper is shared between `-fix` and `-pyramidize`. It reads from file (`-f`), stdin (if piped), or a trailing positional argument.

- [ ] **Step 1: Write the failing tests for input reading**

Add to `internal/cli/cli_test.go`:

```go
func TestReadInputFromString(t *testing.T) {
	got, err := readInput("", "hello world", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("got %q, want %q", got, "hello world")
	}
}

func TestReadInputFromFile(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/input.txt"
	if err := os.WriteFile(path, []byte("file content"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readInput(path, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "file content" {
		t.Fatalf("got %q, want %q", got, "file content")
	}
}

func TestReadInputFromStdin(t *testing.T) {
	stdin := strings.NewReader("stdin content")
	got, err := readInput("", "", stdin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "stdin content" {
		t.Fatalf("got %q, want %q", got, "stdin content")
	}
}

func TestReadInputNone(t *testing.T) {
	_, err := readInput("", "", nil)
	if err == nil {
		t.Fatal("expected error when no input provided")
	}
}

func TestReadInputFilePriority(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/input.txt"
	if err := os.WriteFile(path, []byte("from file"), 0644); err != nil {
		t.Fatal(err)
	}
	// File takes priority over inline string.
	got, err := readInput(path, "from string", strings.NewReader("from stdin"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from file" {
		t.Fatalf("got %q, want %q", got, "from file")
	}
}
```

Add `"os"` and `"strings"` to the test imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -run TestReadInput`
Expected: Compilation failure — `readInput` not defined.

- [ ] **Step 3: Implement readInput in cli.go**

Add to `internal/cli/cli.go`:

```go
import (
	"fmt"
	"io"
	"os"
	"strings"
)

// readInput returns text from the first available source:
// 1. File path (if filePath is non-empty)
// 2. Stdin (if stdinReader is non-nil)
// 3. Inline string (if inlineText is non-empty)
// Returns an error if no input is provided.
func readInput(filePath, inlineText string, stdinReader io.Reader) (string, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("reading input file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	if stdinReader != nil {
		data, err := io.ReadAll(stdinReader)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		text := strings.TrimSpace(string(data))
		if text != "" {
			return text, nil
		}
	}
	if inlineText != "" {
		return inlineText, nil
	}
	return "", fmt.Errorf("no input provided — use -f <file>, pipe to stdin, or pass text as argument")
}

// stdinIfPiped returns os.Stdin if it is connected to a pipe, nil otherwise.
func stdinIfPiped() io.Reader {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil
	}
	if stat.Mode()&os.ModeCharDevice == 0 {
		return os.Stdin
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -run TestReadInput`
Expected: PASS (all 5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat(cli): add input reading helper (file, stdin, inline)"
```

---

### Task 3: -fix Command

**Files:**
- Modify: `internal/cli/fix.go`
- Create: `internal/cli/fix_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/fix_test.go`:

```go
package cli

import (
	"bytes"
	"testing"
)

type mockEnhancer struct {
	result string
	err    error
}

func (m *mockEnhancer) Enhance(text string) (string, error) {
	return m.result, m.err
}

func TestRunFixInlineString(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	enhancer := &mockEnhancer{result: "They're going to the meeting."}
	err := runFixWith([]string{"their going to the meeting"}, &stdout, &stderr, enhancer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stdout.String(); got != "They're going to the meeting.\n" {
		t.Fatalf("got %q, want %q", got, "They're going to the meeting.\n")
	}
}

func TestRunFixNoInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	enhancer := &mockEnhancer{result: "fixed"}
	err := runFixWith(nil, &stdout, &stderr, enhancer)
	if err == nil {
		t.Fatal("expected error when no input")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -run TestRunFix`
Expected: Compilation failure — `runFixWith` not defined.

- [ ] **Step 3: Implement the fix command**

Replace `internal/cli/fix.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// enhancer abstracts the enhance service for testability.
type enhancer interface {
	Enhance(text string) (string, error)
}

func runFix(args []string, stdout io.Writer, stderr io.Writer) error {
	settings, err := initSettings()
	if err != nil {
		return fmt.Errorf("loading settings: %w", err)
	}

	// Import here to avoid import cycle in tests — the real wiring
	// uses the concrete enhance.Service.
	enhanceSvc := newEnhanceService(settings)
	return runFixWith(args, stdout, stderr, enhanceSvc)
}

func runFixWith(args []string, stdout io.Writer, stderr io.Writer, svc enhancer) error {
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	filePath := fs.String("f", "", "Input file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	inlineText := strings.Join(fs.Args(), " ")
	text, err := readInput(*filePath, inlineText, stdinIfPiped())
	if err != nil {
		return err
	}

	result, err := svc.Enhance(text)
	if err != nil {
		return fmt.Errorf("enhance failed: %w", err)
	}

	fmt.Fprintln(stdout, result)
	return nil
}
```

Add to `internal/cli/cli.go` — service factory helpers:

```go
import (
	"keylint/internal/features/enhance"
	"keylint/internal/features/settings"
)

// initSettings creates a settings service for CLI use.
func initSettings() (*settings.Service, error) {
	return settings.NewService()
}

// newEnhanceService creates an enhance service for CLI use.
func newEnhanceService(s *settings.Service) *enhance.Service {
	return enhance.NewService(s)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -run TestRunFix`
Expected: PASS

- [ ] **Step 5: Build and verify `-fix` runs**

Run: `cd /home/dev/projects/keylint && go build -o bin/KeyLint . && echo "thier going" | ./bin/KeyLint -fix`
Expected: Outputs corrected text (requires a configured API key via env var or keyring).

If no API key is configured, verify the error message goes to stderr:
Run: `./bin/KeyLint -fix "hello" 2>&1`
Expected: Error message about missing API key on stderr, exit code 1.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/fix.go internal/cli/fix_test.go internal/cli/cli.go
git commit -m "feat(cli): implement -fix command with inline, file, and stdin input"
```

---

### Task 4: -pyramidize Command

**Files:**
- Modify: `internal/cli/pyramidize.go`
- Create: `internal/cli/pyramidize_test.go`
- Modify: `internal/features/pyramidize/service.go` (nil-clipboard guard)

- [ ] **Step 1: Add nil-clipboard guard to SendBack**

In `internal/features/pyramidize/service.go`, modify the `SendBack` method (line 72):

```go
func (svc *Service) SendBack(text string) error {
	if svc.clipboard == nil {
		return fmt.Errorf("SendBack is not available in CLI mode")
	}
	if err := svc.clipboard.Write(text); err != nil {
		return fmt.Errorf("clipboard write failed: %w", err)
	}
	svc.mu.Lock()
	windowID := svc.sourceWindowID
	svc.mu.Unlock()
	return sendBackToWindow(windowID)
}
```

- [ ] **Step 2: Write failing tests for pyramidize CLI**

Create `internal/cli/pyramidize_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"keylint/internal/features/pyramidize"
)

type mockPyramidizer struct {
	result pyramidize.PyramidizeResult
	err    error
}

func (m *mockPyramidizer) Pyramidize(req pyramidize.PyramidizeRequest) (pyramidize.PyramidizeResult, error) {
	return m.result, m.err
}

func TestRunPyramidizeTextOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "Subject Line\n\nStructured body.",
			DocumentType: "EMAIL",
			QualityScore: 0.85,
			QualityFlags: []string{},
		},
	}
	err := runPyramidizeWith([]string{"-type", "email", "raw input text"}, &stdout, &stderr, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stdout.String(); got != "Subject Line\n\nStructured body.\n" {
		t.Fatalf("got %q, want plain text output", got)
	}
}

func TestRunPyramidizeJSONOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "Subject Line\n\nBody.",
			DocumentType: "EMAIL",
			QualityScore: 0.85,
			QualityFlags: []string{},
		},
	}
	err := runPyramidizeWith([]string{"--json", "-type", "email", "raw input"}, &stdout, &stderr, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result pyramidize.PyramidizeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, stdout.String())
	}
	if result.QualityScore != 0.85 {
		t.Fatalf("qualityScore = %v, want 0.85", result.QualityScore)
	}
}

func TestRunPyramidizeDefaultType(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var capturedReq pyramidize.PyramidizeRequest
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "output",
			QualityFlags: []string{},
		},
	}
	// Wrap to capture the request.
	capturing := &capturingPyramidizer{mock: mock, captured: &capturedReq}
	err := runPyramidizeWith([]string{"some text"}, &stdout, &stderr, capturing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.DocumentType != "auto" {
		t.Fatalf("documentType = %q, want %q", capturedReq.DocumentType, "auto")
	}
}

type capturingPyramidizer struct {
	mock     *mockPyramidizer
	captured *pyramidize.PyramidizeRequest
}

func (c *capturingPyramidizer) Pyramidize(req pyramidize.PyramidizeRequest) (pyramidize.PyramidizeResult, error) {
	*c.captured = req
	return c.mock.Pyramidize(req)
}

func TestRunPyramidizeProviderOverride(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var capturedReq pyramidize.PyramidizeRequest
	mock := &mockPyramidizer{
		result: pyramidize.PyramidizeResult{
			FullDocument: "output",
			QualityFlags: []string{},
		},
	}
	capturing := &capturingPyramidizer{mock: mock, captured: &capturedReq}
	err := runPyramidizeWith([]string{"--provider", "openai", "--model", "gpt-4o", "text"}, &stdout, &stderr, capturing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.Provider != "openai" {
		t.Fatalf("provider = %q, want %q", capturedReq.Provider, "openai")
	}
	if capturedReq.Model != "gpt-4o" {
		t.Fatalf("model = %q, want %q", capturedReq.Model, "gpt-4o")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -run TestRunPyramidize`
Expected: Compilation failure — `runPyramidizeWith` and `pyramidizer` interface not defined.

- [ ] **Step 4: Implement the pyramidize command**

Replace `internal/cli/pyramidize.go`:

```go
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"keylint/internal/features/pyramidize"
)

// pyramidizer abstracts the pyramidize service for testability.
type pyramidizer interface {
	Pyramidize(req pyramidize.PyramidizeRequest) (pyramidize.PyramidizeResult, error)
}

func runPyramidize(args []string, stdout io.Writer, stderr io.Writer) error {
	settings, err := initSettings()
	if err != nil {
		return fmt.Errorf("loading settings: %w", err)
	}

	svc := pyramidize.NewService(settings, nil)
	return runPyramidizeWith(args, stdout, stderr, svc)
}

func runPyramidizeWith(args []string, stdout io.Writer, stderr io.Writer, svc pyramidizer) error {
	fs := flag.NewFlagSet("pyramidize", flag.ContinueOnError)
	fs.SetOutput(stderr)

	filePath := fs.String("f", "", "Input file path")
	docType := fs.String("type", "auto", "Document type: auto|email|wiki|memo|powerpoint")
	jsonOut := fs.Bool("json", false, "Output full result as JSON")
	provider := fs.String("provider", "", "AI provider override: claude|openai|ollama")
	model := fs.String("model", "", "Model override (e.g. claude-sonnet-4-6)")
	style := fs.String("style", "professional", "Communication style")
	relationship := fs.String("relationship", "professional", "Relationship level")

	if err := fs.Parse(args); err != nil {
		return err
	}

	inlineText := strings.Join(fs.Args(), " ")
	text, err := readInput(*filePath, inlineText, stdinIfPiped())
	if err != nil {
		return err
	}

	req := pyramidize.PyramidizeRequest{
		Text:               text,
		DocumentType:       *docType,
		CommunicationStyle: *style,
		RelationshipLevel:  *relationship,
		Provider:           *provider,
		Model:              *model,
	}

	result, err := svc.Pyramidize(req)
	if err != nil {
		return fmt.Errorf("pyramidize failed: %w", err)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintln(stdout, result.FullDocument)
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/dev/projects/keylint && go test ./internal/cli/ -v -run TestRunPyramidize`
Expected: PASS (all 4 tests).

- [ ] **Step 6: Run the full test suite**

Run: `cd /home/dev/projects/keylint && go test ./internal/... -v`
Expected: All existing tests pass, plus new CLI tests.

- [ ] **Step 7: Verify the build and basic invocation**

Run: `cd /home/dev/projects/keylint && go build -o bin/KeyLint .`
Then: `./bin/KeyLint -pyramidize --help 2>&1` (should show flag usage).

- [ ] **Step 8: Commit**

```bash
git add internal/cli/pyramidize.go internal/cli/pyramidize_test.go internal/features/pyramidize/service.go
git commit -m "feat(cli): implement -pyramidize command with JSON output and provider overrides"
```

---

### Task 5: Deterministic Evaluation Checks

**Files:**
- Create: `internal/features/pyramidize/eval_deterministic.go`
- Create: `internal/features/pyramidize/eval_deterministic_test.go`

- [ ] **Step 1: Write failing tests for deterministic checks**

Create `internal/features/pyramidize/eval_deterministic_test.go`:

```go
package pyramidize

import (
	"testing"
)

func TestCheckStructure(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantOK  bool
	}{
		{
			name:   "valid email with subject and headers",
			output: "Subject Line | Details\n\n## Kernergebnis\nText\n\n## Hintergrund\nMore text",
			wantOK: true,
		},
		{
			name:   "empty output",
			output: "",
			wantOK: false,
		},
		{
			name:   "no headers",
			output: "Just a plain paragraph without any structure.",
			wantOK: false,
		},
		{
			name:   "single section only",
			output: "Subject\n\n## Only One Section\nText",
			wantOK: true, // subject + 1 header is minimum
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkStructure(tt.output)
			if result.Pass != tt.wantOK {
				t.Errorf("checkStructure pass = %v, want %v (details: %s)", result.Pass, tt.wantOK, result.Detail)
			}
		})
	}
}

func TestCheckInfoCoverage(t *testing.T) {
	input := "Die Contoso Bank hat beschlossen von Adverity zu Airbyte zu wechseln. Jordan Fischer ist der externe Berater."
	tests := []struct {
		name   string
		output string
		wantOK bool
	}{
		{
			name:   "all key terms present",
			output: "Contoso Bank wechselt von Adverity zu Airbyte. Externer Berater: Jordan Fischer.",
			wantOK: true,
		},
		{
			name:   "missing key entities",
			output: "Eine Bank hat beschlossen den Anbieter zu wechseln.",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkInfoCoverage(input, tt.output)
			if result.Pass != tt.wantOK {
				t.Errorf("checkInfoCoverage pass = %v, want %v (score: %.2f, detail: %s)",
					result.Pass, tt.wantOK, result.Score, result.Detail)
			}
		})
	}
}

func TestCheckNoHallucination(t *testing.T) {
	input := "Meeting mit Hans und Maria über das Projekt Alpha."
	tests := []struct {
		name   string
		output string
		wantOK bool
	}{
		{
			name:   "no new proper nouns",
			output: "Hans und Maria besprechen Projekt Alpha im Meeting.",
			wantOK: true,
		},
		{
			name:   "introduces new proper noun",
			output: "Hans, Maria und Stefan besprechen Projekt Alpha und Beta im Meeting.",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkNoHallucination(input, tt.output)
			if result.Pass != tt.wantOK {
				t.Errorf("pass = %v, want %v (detail: %s)", result.Pass, tt.wantOK, result.Detail)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/dev/projects/keylint && go test ./internal/features/pyramidize/ -v -run "TestCheck(Structure|InfoCoverage|NoHallucination)"`
Expected: Compilation failure — functions not defined.

- [ ] **Step 3: Implement deterministic checks**

Create `internal/features/pyramidize/eval_deterministic.go`:

```go
package pyramidize

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// CheckResult holds the outcome of a single deterministic check.
type CheckResult struct {
	Name   string  `json:"name"`
	Pass   bool    `json:"pass"`
	Score  float64 `json:"score"`  // 0.0–1.0
	Detail string  `json:"detail"` // human-readable explanation
}

// EvalScorecard collects all check results for one sample.
type EvalScorecard struct {
	Checks       []CheckResult `json:"checks"`
	OverallScore float64       `json:"overallScore"`
	AllPassed    bool          `json:"allPassed"`
}

// RunDeterministicChecks runs all fast checks against a single output.
func RunDeterministicChecks(input, output string) EvalScorecard {
	checks := []CheckResult{
		checkStructure(output),
		checkInfoCoverage(input, output),
		checkNoHallucination(input, output),
	}

	totalScore := 0.0
	allPassed := true
	for _, c := range checks {
		totalScore += c.Score
		if !c.Pass {
			allPassed = false
		}
	}

	return EvalScorecard{
		Checks:       checks,
		OverallScore: totalScore / float64(len(checks)),
		AllPassed:    allPassed,
	}
}

var headerRegex = regexp.MustCompile(`(?m)^#{1,3}\s+.+`)

// checkStructure verifies the output has a subject line and markdown headers.
func checkStructure(output string) CheckResult {
	output = strings.TrimSpace(output)
	if output == "" {
		return CheckResult{Name: "structure", Pass: false, Score: 0, Detail: "empty output"}
	}

	// Subject line = first line (before first blank line).
	parts := strings.SplitN(output, "\n\n", 2)
	hasSubject := strings.TrimSpace(parts[0]) != ""

	headers := headerRegex.FindAllString(output, -1)
	hasHeaders := len(headers) >= 1

	score := 0.0
	if hasSubject {
		score += 0.5
	}
	if hasHeaders {
		score += 0.5
	}

	pass := hasSubject && hasHeaders
	detail := fmt.Sprintf("subject=%v, headers=%d", hasSubject, len(headers))
	return CheckResult{Name: "structure", Pass: pass, Score: score, Detail: detail}
}

// checkInfoCoverage measures how many key terms from the input appear in the output.
// Key terms are capitalized words (likely proper nouns, product names, abbreviations).
func checkInfoCoverage(input, output string) CheckResult {
	inputTerms := extractKeyTerms(input)
	if len(inputTerms) == 0 {
		return CheckResult{Name: "info_coverage", Pass: true, Score: 1.0, Detail: "no key terms in input"}
	}

	outputLower := strings.ToLower(output)
	found := 0
	missing := []string{}
	for _, term := range inputTerms {
		if strings.Contains(outputLower, strings.ToLower(term)) {
			found++
		} else {
			missing = append(missing, term)
		}
	}

	score := float64(found) / float64(len(inputTerms))
	pass := score >= 0.7 // allow some paraphrasing
	detail := fmt.Sprintf("%.0f%% coverage (%d/%d)", score*100, found, len(inputTerms))
	if len(missing) > 0 && len(missing) <= 5 {
		detail += fmt.Sprintf(", missing: %s", strings.Join(missing, ", "))
	}
	return CheckResult{Name: "info_coverage", Pass: pass, Score: score, Detail: detail}
}

// extractKeyTerms returns capitalized words that are likely proper nouns or key entities.
func extractKeyTerms(text string) []string {
	words := strings.Fields(text)
	seen := map[string]bool{}
	terms := []string{}

	// Common German/English words to skip even when capitalized (sentence starts, etc.)
	skip := map[string]bool{
		"die": true, "der": true, "das": true, "den": true, "dem": true,
		"ein": true, "eine": true, "einer": true, "eines": true,
		"und": true, "oder": true, "aber": true, "mit": true, "von": true,
		"für": true, "ist": true, "hat": true, "sind": true, "wird": true,
		"ich": true, "wir": true, "sie": true, "the": true, "and": true,
		"for": true, "has": true, "was": true, "our": true, "not": true,
	}

	for _, w := range words {
		// Strip punctuation from edges.
		cleaned := strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if len(cleaned) < 2 {
			continue
		}
		if skip[strings.ToLower(cleaned)] {
			continue
		}
		// Check if it starts with uppercase (likely proper noun or key term).
		first := rune(cleaned[0])
		if unicode.IsUpper(first) && !seen[strings.ToLower(cleaned)] {
			seen[strings.ToLower(cleaned)] = true
			terms = append(terms, cleaned)
		}
	}
	return terms
}

// checkNoHallucination detects proper nouns in the output that don't appear in the input.
func checkNoHallucination(input, output string) CheckResult {
	inputTerms := map[string]bool{}
	for _, t := range extractKeyTerms(input) {
		inputTerms[strings.ToLower(t)] = true
	}

	outputTerms := extractKeyTerms(output)
	hallucinated := []string{}
	for _, t := range outputTerms {
		if !inputTerms[strings.ToLower(t)] {
			hallucinated = append(hallucinated, t)
		}
	}

	// Allow a small number of new terms (structural words like "Kernergebnis", "Hintergrund").
	score := 1.0
	if len(outputTerms) > 0 {
		score = 1.0 - float64(len(hallucinated))/float64(len(outputTerms))
		if score < 0 {
			score = 0
		}
	}

	pass := len(hallucinated) <= 3 // allow some structural/formatting terms
	detail := fmt.Sprintf("%d new terms", len(hallucinated))
	if len(hallucinated) > 0 && len(hallucinated) <= 5 {
		detail += fmt.Sprintf(": %s", strings.Join(hallucinated, ", "))
	}
	return CheckResult{Name: "no_hallucination", Pass: pass, Score: score, Detail: detail}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/dev/projects/keylint && go test ./internal/features/pyramidize/ -v -run "TestCheck(Structure|InfoCoverage|NoHallucination)"`
Expected: PASS

- [ ] **Step 5: Run the full pyramidize test suite to check for regressions**

Run: `cd /home/dev/projects/keylint && go test ./internal/features/pyramidize/ -v`
Expected: All 47 existing tests + new tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/features/pyramidize/eval_deterministic.go internal/features/pyramidize/eval_deterministic_test.go
git commit -m "feat(eval): add deterministic checks — structure, info coverage, hallucination"
```

---

### Task 6: LLM-as-Judge Scoring

**Files:**
- Create: `internal/features/pyramidize/eval_judge.go`

This file is not build-tagged — it's reusable library code. The actual test invocations are in `eval_test.go` (Task 7).

- [ ] **Step 1: Implement the LLM judge**

Create `internal/features/pyramidize/eval_judge.go`:

```go
package pyramidize

import (
	"encoding/json"
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
func RunJudge(cfg settings.Settings, opts aiOpts, rawInput, baseline, candidate string) (JudgeScore, error) {
	userMessage := fmt.Sprintf("<raw_input>\n%s\n</raw_input>\n\n<baseline>\n%s\n</baseline>\n\n<candidate>\n%s\n</candidate>",
		rawInput, baseline, candidate)

	client := &http.Client{Timeout: 90 * time.Second}
	svc := &Service{
		settings: nil,
		client:   client,
	}

	raw, err := svc.callAISync(cfg, opts, judgeSystemPrompt, userMessage)
	if err != nil {
		return JudgeScore{}, fmt.Errorf("judge AI call failed: %w", err)
	}

	var score JudgeScore
	if err := unmarshalRobust(raw, &score); err != nil {
		return JudgeScore{}, fmt.Errorf("judge parse error: %w (raw: %s)", err, raw)
	}
	return score, nil
}
```

Note: `RunJudge` creates a temporary `Service` struct to reuse `callAISync`. The `settings` field is nil because we pass the `cfg` directly. Review whether `callAISync` accesses `svc.settings` — if so, we need to pass it. Let me check:

Looking at `callAISync` (service.go:391-416): it uses `svc.settings.GetKey(...)` to retrieve API keys. So we DO need the settings service. Update the function signature:

```go
// RunJudge calls the LLM to evaluate a candidate output against the baseline.
func RunJudge(settingsSvc *settings.Service, opts aiOpts, rawInput, baseline, candidate string) (JudgeScore, error) {
	cfg := settingsSvc.Get()
	userMessage := fmt.Sprintf("<raw_input>\n%s\n</raw_input>\n\n<baseline>\n%s\n</baseline>\n\n<candidate>\n%s\n</candidate>",
		rawInput, baseline, candidate)

	client := &http.Client{Timeout: 90 * time.Second}
	svc := &Service{
		settings:  settingsSvc,
		clipboard: nil,
		client:    client,
	}

	raw, err := svc.callAISync(cfg, opts, judgeSystemPrompt, userMessage)
	if err != nil {
		return JudgeScore{}, fmt.Errorf("judge AI call failed: %w", err)
	}

	var score JudgeScore
	if err := unmarshalRobust(raw, &score); err != nil {
		return JudgeScore{}, fmt.Errorf("judge parse error: %w (raw: %s)", err, raw)
	}
	return score, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/dev/projects/keylint && go build ./internal/features/pyramidize/`
Expected: Compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add internal/features/pyramidize/eval_judge.go
git commit -m "feat(eval): add LLM-as-judge scoring with pyramid structure criteria"
```

---

### Task 7: Eval Integration Tests (build-tagged)

**Files:**
- Create: `internal/features/pyramidize/eval_test.go`

- [ ] **Step 1: Implement the build-tagged eval tests**

Create `internal/features/pyramidize/eval_test.go`:

```go
//go:build eval

package pyramidize

// Evaluation tests — make real AI calls against test-data samples.
// Run with: go test -tags eval ./internal/features/pyramidize/ -v -timeout 300s
//
// Requires:
//   - A configured AI provider (env vars: ANTHROPIC_API_KEY or OPENAI_API_KEY)
//   - Network access to the AI provider's API
//
// Override provider/model:
//   EVAL_PROVIDER=claude EVAL_MODEL=claude-sonnet-4-6 go test -tags eval ...

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"keylint/internal/features/settings"
)

// testSample holds one parsed test-data file.
type testSample struct {
	Name     string
	RawInput string
	Baseline string
}

func loadTestSamples(t *testing.T) []testSample {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "test-data", "pyramidal-emails")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read test-data dir: %v", err)
	}

	var samples []testSample
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		raw, baseline := parseTestData(string(data))
		if raw == "" {
			t.Logf("skipping %s: no raw input found", e.Name())
			continue
		}
		samples = append(samples, testSample{
			Name:     strings.TrimSuffix(e.Name(), ".md"),
			RawInput: raw,
			Baseline: baseline,
		})
	}
	if len(samples) == 0 {
		t.Fatal("no test samples found")
	}
	return samples
}

var fenceBlockRegex = regexp.MustCompile("(?s)```\\w*\\n?(.*?)```")

func parseTestData(content string) (rawInput, baseline string) {
	sections := strings.Split(content, "# User accepted output")
	if len(sections) < 2 {
		return "", ""
	}

	rawSection := strings.TrimPrefix(sections[0], "# Raw Input")
	if m := fenceBlockRegex.FindStringSubmatch(rawSection); len(m) > 1 {
		rawInput = strings.TrimSpace(m[1])
	}
	if m := fenceBlockRegex.FindStringSubmatch(sections[1]); len(m) > 1 {
		baseline = strings.TrimSpace(m[1])
	}
	return
}

func TestEvalPyramidize(t *testing.T) {
	settingsSvc, err := settings.NewService()
	if err != nil {
		t.Fatalf("settings init: %v", err)
	}

	provider := os.Getenv("EVAL_PROVIDER")
	model := os.Getenv("EVAL_MODEL")

	svc := NewService(settingsSvc, nil)
	samples := loadTestSamples(t)

	// Create eval run directory.
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	runDir := filepath.Join("..", "..", "..", "test-data", "eval-runs", timestamp)
	samplesDir := filepath.Join(runDir, "samples")
	if err := os.MkdirAll(samplesDir, 0755); err != nil {
		t.Fatalf("creating eval-run dir: %v", err)
	}

	type sampleResult struct {
		Name         string        `json:"name"`
		Deterministic EvalScorecard `json:"deterministic"`
		Judge        *JudgeScore   `json:"judge,omitempty"`
		Error        string        `json:"error,omitempty"`
	}

	resultsFile, err := os.Create(filepath.Join(runDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("creating results file: %v", err)
	}
	defer resultsFile.Close()

	totalDet := 0.0
	totalJudge := 0.0
	judgeCount := 0

	for _, sample := range samples {
		t.Run(sample.Name, func(t *testing.T) {
			result, err := svc.Pyramidize(PyramidizeRequest{
				Text:               sample.RawInput,
				DocumentType:       "email",
				CommunicationStyle: "professional",
				RelationshipLevel:  "professional",
				Provider:           provider,
				Model:              model,
			})

			sr := sampleResult{Name: sample.Name}

			if err != nil {
				sr.Error = err.Error()
				t.Errorf("pyramidize failed: %v", err)
			} else {
				// Save generated output.
				outPath := filepath.Join(samplesDir, sample.Name+".md")
				os.WriteFile(outPath, []byte(result.FullDocument), 0644)

				// Deterministic checks.
				sr.Deterministic = RunDeterministicChecks(sample.RawInput, result.FullDocument)
				totalDet += sr.Deterministic.OverallScore

				t.Logf("deterministic: %.2f (pass=%v)", sr.Deterministic.OverallScore, sr.Deterministic.AllPassed)
				for _, c := range sr.Deterministic.Checks {
					t.Logf("  %s: %.2f pass=%v — %s", c.Name, c.Score, c.Pass, c.Detail)
				}

				// LLM-as-judge (if baseline available).
				if sample.Baseline != "" {
					judge, err := RunJudge(settingsSvc, aiOpts{provider: provider, model: model},
						sample.RawInput, sample.Baseline, result.FullDocument)
					if err != nil {
						t.Logf("judge failed: %v", err)
					} else {
						sr.Judge = &judge
						totalJudge += judge.Overall
						judgeCount++
						t.Logf("judge: overall=%.2f pyramid=%.2f clarity=%.2f completeness=%.2f tone=%.2f",
							judge.Overall, judge.PyramidStructure, judge.Clarity, judge.Completeness, judge.TonePreservation)
						t.Logf("judge rationale: %s", judge.Rationale)
					}
				}
			}

			// Write result line.
			line, _ := json.Marshal(sr)
			fmt.Fprintf(resultsFile, "%s\n", line)
		})
	}

	// Write summary.
	summary := map[string]any{
		"timestamp":       timestamp,
		"provider":        provider,
		"model":           model,
		"sampleCount":     len(samples),
		"avgDeterministic": totalDet / float64(len(samples)),
	}
	if judgeCount > 0 {
		summary["avgJudge"] = totalJudge / float64(judgeCount)
		summary["judgeCount"] = judgeCount
	}
	summaryData, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(filepath.Join(runDir, "summary.json"), summaryData, 0644)

	t.Logf("\n=== EVAL SUMMARY ===")
	t.Logf("Samples: %d", len(samples))
	t.Logf("Avg deterministic: %.2f", totalDet/float64(len(samples)))
	if judgeCount > 0 {
		t.Logf("Avg judge overall: %.2f (%d samples)", totalJudge/float64(judgeCount), judgeCount)
	}
	t.Logf("Results: %s", runDir)
}
```

- [ ] **Step 2: Verify it does NOT run with normal tests**

Run: `cd /home/dev/projects/keylint && go test ./internal/features/pyramidize/ -v`
Expected: All existing tests pass. `TestEvalPyramidize` does NOT appear in output.

- [ ] **Step 3: Verify it compiles with the eval tag**

Run: `cd /home/dev/projects/keylint && go test -tags eval ./internal/features/pyramidize/ -list TestEval`
Expected: Lists `TestEvalPyramidize` without running it.

- [ ] **Step 4: Commit**

```bash
git add internal/features/pyramidize/eval_test.go
git commit -m "feat(eval): add build-tagged integration tests with eval run logging"
```

---

### Task 8: Eval Shell Scripts

**Files:**
- Create: `scripts/eval.sh`
- Create: `scripts/eval-human.sh`

- [ ] **Step 1: Create the automated eval script**

Create `scripts/eval.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Automated evaluation runner for pyramidize quality.
# Wraps `go test -tags eval` and prints a summary.
#
# Usage:
#   ./scripts/eval.sh                          # use configured provider
#   EVAL_PROVIDER=claude EVAL_MODEL=claude-sonnet-4-6 ./scripts/eval.sh
#   ./scripts/eval.sh --provider openai --model gpt-4o

cd "$(git rev-parse --show-toplevel)"

# Parse optional --provider / --model flags.
while [[ $# -gt 0 ]]; do
    case "$1" in
        --provider) export EVAL_PROVIDER="$2"; shift 2 ;;
        --model)    export EVAL_MODEL="$2"; shift 2 ;;
        *)          echo "Unknown flag: $1" >&2; exit 1 ;;
    esac
done

echo "=== KeyLint Pyramidize Eval ==="
echo "Provider: ${EVAL_PROVIDER:-<from settings>}"
echo "Model:    ${EVAL_MODEL:-<provider default>}"
echo ""

go test -tags eval ./internal/features/pyramidize/ -v -timeout 600s 2>&1 | tee /dev/stderr | tail -1

# Find the newest eval-run directory and print its summary.
LATEST=$(ls -td test-data/eval-runs/*/ 2>/dev/null | head -1)
if [[ -n "$LATEST" ]]; then
    echo ""
    echo "=== Results saved to: $LATEST ==="
    if [[ -f "${LATEST}summary.json" ]]; then
        cat "${LATEST}summary.json"
    fi
fi
```

- [ ] **Step 2: Create the human review script**

Create `scripts/eval-human.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Interactive human evaluation for pyramidize quality.
# Runs each test-data sample through the CLI, shows a 3-pane comparison,
# and collects human scores.
#
# Usage:
#   ./scripts/eval-human.sh
#   ./scripts/eval-human.sh --provider claude --model claude-sonnet-4-6

cd "$(git rev-parse --show-toplevel)"

PROVIDER=""
MODEL=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --provider) PROVIDER="$2"; shift 2 ;;
        --model)    MODEL="$2"; shift 2 ;;
        *)          echo "Unknown flag: $1" >&2; exit 1 ;;
    esac
done

# Build the binary first.
echo "Building KeyLint..."
go build -o bin/KeyLint .

TIMESTAMP=$(date +"%Y-%m-%dT%H-%M-%S")
RUN_DIR="test-data/eval-runs/${TIMESTAMP}"
SAMPLES_DIR="${RUN_DIR}/samples"
mkdir -p "$SAMPLES_DIR"

RESULTS_FILE="${RUN_DIR}/results.jsonl"
TESTDATA_DIR="test-data/pyramidal-emails"

CLI_FLAGS=""
[[ -n "$PROVIDER" ]] && CLI_FLAGS="$CLI_FLAGS --provider $PROVIDER"
[[ -n "$MODEL" ]]    && CLI_FLAGS="$CLI_FLAGS --model $MODEL"

TOTAL=0
SCORED=0
SCORE_SUM=0

for file in "$TESTDATA_DIR"/*.md; do
    TOTAL=$((TOTAL + 1))
    NAME=$(basename "$file" .md)

    # Parse raw input and baseline from the test-data file.
    RAW=$(sed -n '/^# Raw Input$/,/^# User accepted output$/{ /^# /d; p; }' "$file" | sed '/^```/d')
    BASELINE=$(sed -n '/^# User accepted output$/,$ { /^# /d; p; }' "$file" | sed '/^```/d')

    # Run pyramidize CLI.
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Sample ${TOTAL}: ${NAME}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # Write just the raw input to a temp file (strip markdown markers).
    TMPFILE=$(mktemp)
    echo "$RAW" > "$TMPFILE"
    OUTPUT=$(./bin/KeyLint -pyramidize -type email $CLI_FLAGS -f "$TMPFILE" 2>/dev/null || echo "[ERROR: pyramidize failed]")
    rm -f "$TMPFILE"

    # Save the generated output.
    echo "$OUTPUT" > "${SAMPLES_DIR}/${NAME}.md"

    # Display 3-pane comparison.
    echo ""
    echo "┌─── RAW INPUT (first 20 lines) ───"
    echo "$RAW" | head -20
    echo "└───"
    echo ""
    echo "┌─── BASELINE ───"
    echo "$BASELINE" | head -30
    echo "└───"
    echo ""
    echo "┌─── NEW OUTPUT ───"
    echo "$OUTPUT" | head -30
    echo "└───"
    echo ""

    # Prompt for human score.
    while true; do
        read -rp "Score (1-5), [s]kip, [q]uit: " REPLY
        case "$REPLY" in
            [1-5])
                SCORED=$((SCORED + 1))
                SCORE_SUM=$((SCORE_SUM + REPLY))
                echo "{\"name\":\"${NAME}\",\"humanScore\":${REPLY}}" >> "$RESULTS_FILE"
                break
                ;;
            s|S)
                echo "{\"name\":\"${NAME}\",\"humanScore\":null,\"skipped\":true}" >> "$RESULTS_FILE"
                break
                ;;
            q|Q)
                echo ""
                echo "Quitting. Results so far saved to: ${RUN_DIR}"
                exit 0
                ;;
            *)
                echo "Enter 1-5, s, or q."
                ;;
        esac
    done
done

# Write summary.
AVG="0"
if [[ $SCORED -gt 0 ]]; then
    AVG=$(echo "scale=2; $SCORE_SUM / $SCORED" | bc)
fi

cat > "${RUN_DIR}/summary.json" <<EOF
{
  "timestamp": "${TIMESTAMP}",
  "provider": "${PROVIDER:-<from settings>}",
  "model": "${MODEL:-<provider default>}",
  "mode": "human",
  "sampleCount": ${TOTAL},
  "scoredCount": ${SCORED},
  "avgHumanScore": ${AVG}
}
EOF

echo ""
echo "=== HUMAN EVAL COMPLETE ==="
echo "Samples: ${TOTAL}, Scored: ${SCORED}, Avg: ${AVG}/5"
echo "Results: ${RUN_DIR}"
```

- [ ] **Step 3: Make both scripts executable**

Run:
```bash
chmod +x scripts/eval.sh scripts/eval-human.sh
```

- [ ] **Step 4: Verify eval.sh syntax**

Run: `bash -n scripts/eval.sh && bash -n scripts/eval-human.sh && echo "OK"`
Expected: `OK`

- [ ] **Step 5: Commit**

```bash
git add scripts/eval.sh scripts/eval-human.sh
git commit -m "feat(eval): add automated and human review eval scripts"
```

---

### Task 9: Update .gitignore and CLAUDE.md

**Files:**
- Modify: `.gitignore`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add eval-runs to .gitignore**

Append to `.gitignore`:

```
test-data/eval-runs/
.superpowers/
```

- [ ] **Step 2: Update CLAUDE.md with CLI and eval documentation**

In `CLAUDE.md`, add to the **Build / run / test** code block:

```
# CLI commands (headless, no GUI):
./bin/KeyLint -fix "text to fix"                       # silent grammar fix
./bin/KeyLint -fix -f input.txt                        # fix from file
cat input.txt | ./bin/KeyLint -fix                     # fix from stdin
./bin/KeyLint -pyramidize -type email -f input.md      # pyramidize from file
./bin/KeyLint -pyramidize --json -f input.md           # JSON output with quality score
./bin/KeyLint -pyramidize --provider claude --model claude-sonnet-4-6 -f input.md  # override provider/model

# Evaluation tests (real API calls — NOT run by default):
go test -tags eval ./internal/features/pyramidize/ -v -timeout 300s
EVAL_PROVIDER=claude EVAL_MODEL=claude-sonnet-4-6 go test -tags eval ./internal/features/pyramidize/ -v -timeout 300s
./scripts/eval.sh                                      # automated eval with summary
./scripts/eval-human.sh                                # interactive human review mode
```

Also add to **Architecture decisions:**

```
- CLI mode (`-fix`, `-pyramidize`) dispatches before Wails boots in `main.go`, uses the same service layer with manual wiring (no Wire/Wails). Prompts are identical between CLI and GUI — output formatting is the caller's concern.
- Evaluation tests use `//go:build eval` build tag to isolate from normal `go test` runs. They make real API calls and write results to `test-data/eval-runs/<timestamp>/`.
```

- [ ] **Step 3: Commit**

```bash
git add .gitignore CLAUDE.md
git commit -m "docs: add CLI commands and eval build tag to CLAUDE.md, update .gitignore"
```

---

### Task 10: End-to-End Smoke Test

**Files:** None new — this task verifies everything works together.

- [ ] **Step 1: Build the binary**

Run: `cd /home/dev/projects/keylint && go build -o bin/KeyLint .`
Expected: Compiles cleanly.

- [ ] **Step 2: Run all unit tests**

Run: `cd /home/dev/projects/keylint && go test ./internal/... -v`
Expected: All tests pass (existing + new CLI + deterministic eval tests).

- [ ] **Step 3: Run frontend tests**

Run: `cd /home/dev/projects/keylint/frontend && npm test`
Expected: All frontend tests pass (no regressions — we didn't touch frontend code).

- [ ] **Step 4: Test CLI -fix with inline string**

Run: `./bin/KeyLint -fix "thier going to the meeting"`
Expected: Corrected text on stdout (if API key is configured), or clear error about missing key on stderr.

- [ ] **Step 5: Test CLI -pyramidize with file input and JSON output**

Create a plain text test file first (test-data files contain markdown markers):
Run: `sed -n '/^# Raw Input$/,/^# User accepted output$/{ /^# /d; /^\x60\x60\x60/d; p; }' test-data/pyramidal-emails/email-meeting-request.md > /tmp/test-input.md && ./bin/KeyLint -pyramidize -type email --json -f /tmp/test-input.md`
Expected: JSON output with `fullDocument`, `qualityScore`, `documentType` fields.

- [ ] **Step 6: Test CLI -pyramidize with stdin**

Run: `echo "Hallo, bitte um ein Meeting morgen um 10 Uhr wegen dem Projekt." | ./bin/KeyLint -pyramidize -type email`
Expected: Pyramidized text on stdout.

- [ ] **Step 7: Test --help shows flags**

Run: `./bin/KeyLint -pyramidize --help 2>&1`
Expected: Prints flag usage for `-f`, `-type`, `--json`, `--provider`, `--model`, `--style`, `--relationship`.

- [ ] **Step 8: Verify eval tests are isolated**

Run: `cd /home/dev/projects/keylint && go test ./internal/features/pyramidize/ -v 2>&1 | grep -c "TestEval"`
Expected: `0` — eval tests should not appear without `-tags eval`.

- [ ] **Step 9: Run eval integration test (optional — costs API credits)**

Run: `cd /home/dev/projects/keylint && go test -tags eval ./internal/features/pyramidize/ -v -timeout 600s -run TestEvalPyramidize`
Expected: Runs all 13 samples, prints deterministic + judge scores, writes results to `test-data/eval-runs/`.

- [ ] **Step 10: Final commit if any fixups were needed**

If any fixes were made during smoke testing, commit them:
```bash
git add -A
git commit -m "fix(cli): smoke test fixups"
```
