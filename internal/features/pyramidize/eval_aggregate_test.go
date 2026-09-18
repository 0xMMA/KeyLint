package pyramidize

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The regression guard is shell, but what it decides is not cosmetic: it is the
// rule for when a number counts as evidence. These run it against recorded runs,
// so the maths is covered without an API call.
//
// No eval build tag on purpose — this must run in the normal suite.

// fakeRun writes one eval-run directory with the given averages.
func fakeRun(t *testing.T, dir, model string, det, judge float64) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	summary := map[string]any{
		"timestamp":         filepath.Base(dir),
		"gitSHA":            "abc1234",
		"provider":          "claude",
		"model":             model,
		"promptVariant":     2,
		"judge":             map[string]any{"provider": "claude", "model": "claude-sonnet-4-5-20250929", "temperature": 0},
		"schemaEnforcement": false,
		"qualityThreshold":  0.65,
		"sampleCount":       2,
		"avgDeterministic":  det,
		"avgJudge":          judge,
		"judgeCount":        2,
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	lines := fmt.Sprintf(
		`{"name":"sample-a","deterministic":{"overallScore":%.2f},"judge":{"overall":%.2f}}`+"\n"+
			`{"name":"sample-b","deterministic":{"overallScore":%.2f},"judge":{"overall":%.2f}}`+"\n",
		det, judge, det, judge)
	if err := os.WriteFile(filepath.Join(dir, "results.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func aggregate(t *testing.T, args ...string) (map[string]any, int) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "scripts", "eval-aggregate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// A .sh file exists on Windows and jq.exe is a common install, so the
		// guards below both pass there — and exec then fails with something
		// that is not an ExitError, which used to be a hard failure.
		t.Skip("the aggregate script is bash; Windows runs it in CI on Linux")
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("aggregate script not found: %v", err)
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is not installed")
	}

	cmd := exec.Command(script, args...)
	out, err := cmd.Output()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running %s: %v", script, err)
	}
	var parsed map[string]any
	if len(out) > 0 {
		if err := json.Unmarshal(out, &parsed); err != nil {
			t.Fatalf("output is not JSON: %v\n%s", err, out)
		}
	}
	return parsed, code
}

// TestTheSpreadIsTheNoiseFloor: three runs of the same commit differ, and the
// distance between the best and the worst is what a later delta has to beat.
func TestTheSpreadIsTheNoiseFloor(t *testing.T) {
	root := t.TempDir()
	runs := []string{
		fakeRun(t, filepath.Join(root, "r1"), "claude-sonnet-4-6", 0.80, 0.88),
		fakeRun(t, filepath.Join(root, "r2"), "claude-sonnet-4-6", 0.76, 0.90),
		fakeRun(t, filepath.Join(root, "r3"), "claude-sonnet-4-6", 0.78, 0.86),
	}

	got, code := aggregate(t, runs...)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if got["runCount"].(float64) != 3 {
		t.Errorf("runCount = %v, want 3", got["runCount"])
	}
	if consistent, _ := got["configConsistent"].(bool); !consistent {
		t.Error("three identical configurations were reported as inconsistent")
	}
	det := got["deterministic"].(map[string]any)
	if det["mean"].(float64) != 0.78 {
		t.Errorf("mean = %v, want 0.78", det["mean"])
	}
	if det["spread"].(float64) != 0.04 {
		t.Errorf("spread = %v, want 0.04 (0.80 - 0.76)", det["spread"])
	}
}

// TestARegressionIsAnAbsenceOfOverlap: three runs of the new code all scoring
// below every run of the baseline is a difference the old code never produced.
//
// Comparing the new mean against the baseline's spread would miss exactly this:
// a tight cluster 0.023 below the baseline mean sits "inside the noise floor"
// by that test, while none of its runs overlaps the baseline's range at all.
func TestARegressionIsAnAbsenceOfOverlap(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeBaseline(t, root, base)

	now := []string{
		fakeRun(t, filepath.Join(root, "n1"), "claude-sonnet-4-6", 0.7520, 0.88),
		fakeRun(t, filepath.Join(root, "n2"), "claude-sonnet-4-6", 0.7525, 0.88),
		fakeRun(t, filepath.Join(root, "n3"), "claude-sonnet-4-6", 0.7530, 0.88),
	}
	got, code := aggregate(t, append([]string{"--compare", base}, now...)...)

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if v, _ := got["verdict"].(string); !strings.HasPrefix(v, "regression") {
		t.Errorf("verdict = %v", got["verdict"])
	}
}

// TestAWideSpreadIsNotARegression: a new set that scatters across the
// baseline's range says nothing, however far its mean has moved. Testing a mean
// against someone else's spread would call this a regression.
func TestAWideSpreadIsNotARegression(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeBaseline(t, root, base)

	now := []string{
		fakeRun(t, filepath.Join(root, "n1"), "claude-sonnet-4-6", 0.70, 0.88),
		fakeRun(t, filepath.Join(root, "n2"), "claude-sonnet-4-6", 0.78, 0.88),
		fakeRun(t, filepath.Join(root, "n3"), "claude-sonnet-4-6", 0.86, 0.88),
	}
	got, code := aggregate(t, append([]string{"--compare", base}, now...)...)

	if code != 0 {
		t.Errorf("exit = %d, want 0 — these runs overlap the baseline", code)
	}
	if got["verdict"] != "inconclusive: the ranges overlap" {
		t.Errorf("verdict = %v", got["verdict"])
	}
}

// TestOneRunEarnsNoVerdict: a single run has no range, so there is nothing to
// compare. Handing back a confident answer here is the mistake the whole
// baseline machinery exists to prevent.
func TestOneRunEarnsNoVerdict(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeBaseline(t, root, base)

	now := fakeRun(t, filepath.Join(root, "n1"), "claude-sonnet-4-6", 0.50, 0.50)
	got, code := aggregate(t, "--compare", base, now)

	if code != 0 {
		t.Errorf("exit = %d, want 0 — an indicative answer is not a failure", code)
	}
	if v, _ := got["verdict"].(string); !strings.HasPrefix(v, "indicative only") {
		t.Errorf("verdict = %v, want an indicative one even though the drop is huge", got["verdict"])
	}
}

// TestAnImprovementIsNamedRatherThanDismissed: reporting a real gain as noise
// teaches people to ignore the tool.
func TestAnImprovementIsNamedRatherThanDismissed(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeBaseline(t, root, base)

	now := []string{
		fakeRun(t, filepath.Join(root, "n1"), "claude-sonnet-4-6", 0.95, 0.95),
		fakeRun(t, filepath.Join(root, "n2"), "claude-sonnet-4-6", 0.96, 0.96),
		fakeRun(t, filepath.Join(root, "n3"), "claude-sonnet-4-6", 0.97, 0.97),
	}
	got, code := aggregate(t, append([]string{"--compare", base}, now...)...)

	if code != 0 {
		t.Errorf("exit = %d, want 0 — going up is not a failure", code)
	}
	if v, _ := got["verdict"].(string); !strings.HasPrefix(v, "improvement") {
		t.Errorf("verdict = %v", got["verdict"])
	}
}

// TestMixedConfigurationsAreRefusedRatherThanAveraged: averaging a Sonnet run
// with a Haiku run produces a number describing neither — and it used to slip
// past the model guard, because the guard only read run one's configuration.
func TestMixedConfigurationsAreRefusedRatherThanAveraged(t *testing.T) {
	root := t.TempDir()
	mixed := []string{
		fakeRun(t, filepath.Join(root, "a"), "claude-sonnet-4-6", 0.80, 0.88),
		fakeRun(t, filepath.Join(root, "b"), "claude-haiku-4-5-20251001", 0.40, 0.50),
	}

	_, code := aggregate(t, mixed...)
	if code != 2 {
		t.Errorf("exit = %d, want 2 — these runs measured different things", code)
	}
}

// TestADifferentModelIsNotAComparison: two models are two measurements, and
// subtracting one from the other is meaningless however tempting the number is.
func TestADifferentModelIsNotAComparison(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeBaseline(t, root, base)

	now := []string{
		fakeRun(t, filepath.Join(root, "n1"), "claude-opus-4-6", 0.60, 0.70),
		fakeRun(t, filepath.Join(root, "n2"), "claude-opus-4-6", 0.61, 0.71),
	}
	got, code := aggregate(t, append([]string{"--compare", base}, now...)...)

	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if got["verdict"] != "not comparable: different configuration" {
		t.Errorf("verdict = %v", got["verdict"])
	}
}

// TestTheSameRunTwiceIsRefused: it would report three runs with a spread of
// zero, which is the most misleading document this script can produce.
func TestTheSameRunTwiceIsRefused(t *testing.T) {
	root := t.TempDir()
	one := fakeRun(t, filepath.Join(root, "one"), "claude-sonnet-4-6", 0.80, 0.88)

	if _, code := aggregate(t, one, one); code != 3 {
		t.Errorf("exit = %d, want 3", code)
	}
}

// TestAnUnusableBaselineIsRefused: an empty or truncated file reaches jq as
// null and comes back as a confident-looking regression.
func TestAnUnusableBaselineIsRefused(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "empty.json")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	now := fakeRun(t, filepath.Join(root, "n1"), "claude-sonnet-4-6", 0.80, 0.88)

	if _, code := aggregate(t, "--compare", empty, now); code != 3 {
		t.Errorf("exit = %d, want 3", code)
	}
}

// TestThePassCountIsRecorded: quality-status.md warns that the pass count moves
// by two on identical code, and nothing used to compute it, so the warning
// could not be checked.
func TestThePassCountIsRecorded(t *testing.T) {
	root := t.TempDir()
	runs := []string{
		fakeRun(t, filepath.Join(root, "r1"), "claude-sonnet-4-6", 0.80, 0.88),
		fakeRun(t, filepath.Join(root, "r2"), "claude-sonnet-4-6", 0.76, 0.90),
	}
	got, code := aggregate(t, runs...)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if got["samplesPassing"] == nil {
		t.Error("no samplesPassing in the aggregate")
	}
}

// writeBaseline records three runs spreading 0.04 on the deterministic score.
func writeBaseline(t *testing.T, root, out string) {
	t.Helper()
	runs := []string{
		fakeRun(t, filepath.Join(root, "b1"), "claude-sonnet-4-6", 0.80, 0.88),
		fakeRun(t, filepath.Join(root, "b2"), "claude-sonnet-4-6", 0.76, 0.90),
		fakeRun(t, filepath.Join(root, "b3"), "claude-sonnet-4-6", 0.78, 0.86),
	}
	doc, code := aggregate(t, runs...)
	if code != 0 {
		t.Fatalf("building the baseline exited %d", code)
	}
	data, _ := json.Marshal(doc)
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
