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
	return fakeRunWith(t, dir, model, det, judge, "")
}

// fakeRunWith is fakeRun with a recorded prompt hash. An empty hash writes the
// field as null, which is what a pyramidize run still does.
func fakeRunWith(t *testing.T, dir, model string, det, judge float64, promptHash string) string {
	t.Helper()
	return fakeRunFull(t, dir, model, det, judge, promptHash, 0)
}

// fakeRunWithChecks is a run scored by a named version of the checks.
func fakeRunWithChecks(t *testing.T, dir, model string, det, judge float64, checksVersion int) string {
	t.Helper()
	return fakeRunFull(t, dir, model, det, judge, "", checksVersion)
}

// fakeRunWithSplit is a run that measured one half of a split suite.
func fakeRunWithSplit(t *testing.T, dir, model string, det, judge float64, split string) string {
	t.Helper()
	return fakeRunFull(t, dir, model, det, judge, "", 0, split)
}

// split is variadic so the three wrappers above stay as they are; an omitted
// split is a run that did not record one, which reads as "all".
func fakeRunFull(t *testing.T, dir, model string, det, judge float64, promptHash string, checksVersion int, split ...string) string {
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
	if promptHash != "" {
		summary["promptHash"] = promptHash
	}

	if checksVersion > 0 {
		summary["checksVersion"] = checksVersion
	}
	if len(split) > 0 && split[0] != "" {
		summary["split"] = split[0]
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	// allPassed drives samplesPassing; sample-a passes, sample-b does not, so a
	// count of exactly 1 is the only right answer.
	lines := fmt.Sprintf(
		`{"name":"sample-a","deterministic":{"overallScore":%.2f,"allPassed":true},"judge":{"overall":%.2f}}`+"\n"+
			`{"name":"sample-b","deterministic":{"overallScore":%.2f,"allPassed":false},"judge":{"overall":%.2f}}`+"\n",
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
	passing, ok := got["samplesPassing"].(map[string]any)
	if !ok {
		t.Fatal("no samplesPassing in the aggregate")
	}
	// One of the two samples passes in every fake run.
	for _, field := range []string{"mean", "min", "max"} {
		if passing[field] != float64(1) {
			t.Errorf("samplesPassing.%s = %v, want 1", field, passing[field])
		}
	}
	if passing["spread"] != float64(0) {
		t.Errorf("samplesPassing.spread = %v, want 0", passing["spread"])
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

// TestARunRecordedBeforeTheSuiteFieldStillKeys: summary.json gained `suite` when
// the Fix suite arrived, then `checksVersion`, then `split`. Runs recorded
// before those have none of them, and the key has to name them anyway — as
// `pyramidize`, which is what they were, checks version 1, which is what scored
// them, and `all`, because nothing was held back from them.
func TestARunRecordedBeforeTheSuiteFieldStillKeys(t *testing.T) {
	root := t.TempDir()
	runs := []string{
		fakeRun(t, filepath.Join(root, "r1"), "claude-sonnet-4-6", 0.80, 0.88),
		fakeRun(t, filepath.Join(root, "r2"), "claude-sonnet-4-6", 0.76, 0.90),
	}
	got, code := aggregate(t, runs...)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	const want = "pyramidize|claude|claude-sonnet-4-6|claude|claude-sonnet-4-5-20250929|2|false|0.65|2|1|all"
	if got["configKey"] != want {
		t.Errorf("configKey = %v\nwant       %v", got["configKey"], want)
	}
}

// TestAnOldBaselineIsStillComparable: the three pyramidize baselines in
// test-data were written when the key had eight fields. Adding `suite` and
// `promptHash` to it made every one of them "not comparable" against anything
// measured afterwards — which would have thrown away the only recorded history
// this project has, for a key format change that says nothing about the runs.
func TestAnOldBaselineIsStillComparable(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeBaseline(t, root, base)
	downgradeKey(t, base)

	now := []string{
		fakeRun(t, filepath.Join(root, "n1"), "claude-sonnet-4-6", 0.79, 0.88),
		fakeRun(t, filepath.Join(root, "n2"), "claude-sonnet-4-6", 0.77, 0.89),
	}
	got, code := aggregate(t, append([]string{"--compare", base}, now...)...)

	if code == 2 {
		t.Fatalf("exit = 2: %v", got["verdict"])
	}
	if v, _ := got["verdict"].(string); strings.HasPrefix(v, "not comparable") {
		t.Errorf("verdict = %v", v)
	}
	// And the comparison must report the upgraded key, not the stored one: a
	// reader comparing the two `config` lines should see the same string on
	// both sides when the configuration really is the same.
	baseline := got["baseline"].(map[string]any)
	if baseline["config"] != got["now"].(map[string]any)["config"] {
		t.Errorf("baseline config %v != now config %v", baseline["config"], got["now"].(map[string]any)["config"])
	}
}

// downgradeKey rewrites a baseline the way the script wrote them before `suite`
// and `promptHash` joined the key: eight fields, and a config block with
// neither of the two new names in it.
func downgradeKey(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(doc["configKey"].(string), "|")
	if len(parts) != 11 {
		t.Fatalf("configKey has %d fields, expected the current 11: %v", len(parts), doc["configKey"])
	}
	doc["configKey"] = strings.Join(parts[1:len(parts)-2], "|")
	cfg := doc["config"].(map[string]any)
	delete(cfg, "suite")
	delete(cfg, "promptHash")
	delete(cfg, "checksVersion")
	delete(cfg, "split")

	out, _ := json.Marshal(doc)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAPromptChangeIsReportedNotRefused: the hash was in configKey for one
// revision, which made every prompt change read "not comparable" — the suite
// refusing the only comparison it exists to make. It belongs next to the
// verdict, where it tells the reader what else moved.
func TestAPromptChangeIsReportedNotRefused(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeBaseline(t, root, base)
	setPromptHash(t, base, "aaaa1111")

	now := []string{
		fakeRunWith(t, filepath.Join(root, "n1"), "claude-sonnet-4-6", 0.79, 0.88, "bbbb2222"),
		fakeRunWith(t, filepath.Join(root, "n2"), "claude-sonnet-4-6", 0.77, 0.89, "bbbb2222"),
	}
	got, code := aggregate(t, append([]string{"--compare", base}, now...)...)

	if code == 2 {
		t.Fatalf("a prompt change was refused as a different configuration: %v", got["verdict"])
	}
	hash, ok := got["promptHash"].(map[string]any)
	if !ok {
		t.Fatal("the comparison does not report the prompt hash")
	}
	if hash["changed"] != true {
		t.Errorf("promptHash.changed = %v, want true (%v -> %v)", hash["changed"], hash["baseline"], hash["now"])
	}
}

// setPromptHash rewrites a baseline's recorded prompt hash.
func setPromptHash(t *testing.T, path, hash string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["config"].(map[string]any)["promptHash"] = hash
	out, _ := json.Marshal(doc)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAChangedInstrumentIsNotComparable: the checks themselves are part of the
// measurement. When a round of review changed what the Fix suite's checks
// accept, the recorded baseline kept a configKey that said "same configuration"
// — so the next comparison would have reported the instrument's move as the
// model's, in the exact voice of a regression.
func TestAChangedInstrumentIsNotComparable(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeBaseline(t, root, base) // checks version 1, by omission

	now := []string{
		fakeRunWithChecks(t, filepath.Join(root, "n1"), "claude-sonnet-4-6", 0.79, 0.88, 2),
		fakeRunWithChecks(t, filepath.Join(root, "n2"), "claude-sonnet-4-6", 0.77, 0.89, 2),
	}
	got, code := aggregate(t, append([]string{"--compare", base}, now...)...)

	if code != 2 {
		t.Errorf("exit = %d, want 2 — these runs were scored by different checks", code)
	}
	if got["verdict"] != "not comparable: different configuration" {
		t.Errorf("verdict = %v", got["verdict"])
	}
}

// TestTheTwoHalvesAreNotComparable: the Fix samples are split into a tuning half
// and a held-out half, and the whole point is that a number from one is not a
// number from the other. Ten samples against fifteen is a different measurement
// however similar the score looks, and comparing them is the mistake the split
// exists to prevent.
func TestTheTwoHalvesAreNotComparable(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeBaseline(t, root, base) // no split recorded, so it keys as "all"

	now := []string{
		fakeRunWithSplit(t, filepath.Join(root, "n1"), "claude-sonnet-4-6", 0.79, 0.88, "tune"),
		fakeRunWithSplit(t, filepath.Join(root, "n2"), "claude-sonnet-4-6", 0.77, 0.89, "tune"),
	}
	got, code := aggregate(t, append([]string{"--compare", base}, now...)...)

	if code != 2 {
		t.Errorf("exit = %d, want 2 — a tune-half run is not comparable with an all-samples baseline", code)
	}
	if got["verdict"] != "not comparable: different configuration" {
		t.Errorf("verdict = %v", got["verdict"])
	}
}
