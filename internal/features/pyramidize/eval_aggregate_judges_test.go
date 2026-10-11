package pyramidize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Two judges score every sample (E3, 2026-10-11): Haiku 5.5 and Sonnet 5.5,
// reported separately and never averaged into one number. These pin how the
// aggregate and the comparison treat them.

const (
	judgeHaiku  = "claude-haiku-5-5"
	judgeSonnet = "claude-sonnet-5-5"
)

// fakeRunJudged writes a CLI-era run scored by the given judges, each with its
// own average. Every judge scores both samples unless edit says otherwise.
func fakeRunJudged(t *testing.T, dir string, det float64, scores map[string]float64, edit func(map[string]any)) string {
	t.Helper()
	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	judges := make([]any, 0, len(ids))
	avgs := map[string]any{}
	counts := map[string]any{}
	perSample := map[string]any{}
	for _, id := range ids {
		judges = append(judges, map[string]any{
			"provider":        "claude-code",
			"model":           id,
			"temperature":     nil,
			"temperatureNote": "not pinned",
			"resolvedModel":   id,
		})
		avgs[id] = scores[id]
		counts[id] = 2
		perSample[id] = map[string]any{"overall": scores[id]}
	}
	dir = fakeRunEdited(t, dir, "claude-sonnet-5-5", det, 0, func(s map[string]any) {
		s["provider"] = "claude-code"
		s["modelRequested"] = "sonnet"
		s["thinking"] = "on"
		s["claudeCodeVersion"] = "2.1.296"
		delete(s, "judge")
		delete(s, "avgJudge")
		delete(s, "judgeCount")
		s["judges"] = judges
		s["avgJudges"] = avgs
		s["judgeCounts"] = counts
		if edit != nil {
			edit(s)
		}
	})
	sampleJudges, _ := json.Marshal(perSample)
	lines := fmt.Sprintf(
		`{"name":"sample-a","deterministic":{"overallScore":%.2f,"allPassed":true},"judges":%s}`+"\n"+
			`{"name":"sample-b","deterministic":{"overallScore":%.2f,"allPassed":false},"judges":%s}`+"\n",
		det, sampleJudges, det, sampleJudges)
	if err := os.WriteFile(filepath.Join(dir, "results.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeJudgedBaseline aggregates runs into a baseline file.
func writeJudgedBaseline(t *testing.T, out string, runs ...string) {
	t.Helper()
	doc, code := aggregate(t, runs...)
	if code != 0 {
		t.Fatalf("building the baseline exited %d", code)
	}
	data, _ := json.Marshal(doc)
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func twoJudges(haiku, sonnet float64) map[string]float64 {
	return map[string]float64{judgeHaiku: haiku, judgeSonnet: sonnet}
}

// TestEachJudgeIsAggregatedSeparately: the two judges get an interval each, at
// run and at sample level, and no averaged judge number exists to quote.
func TestEachJudgeIsAggregatedSeparately(t *testing.T) {
	root := t.TempDir()
	got, code := aggregate(t,
		fakeRunJudged(t, filepath.Join(root, "r1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "r2"), 0.78, twoJudges(0.86, 0.74), nil),
	)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if _, ok := got["judge"]; ok {
		t.Errorf("the aggregate carries a single judge field (%v); the judges must stay apart", got["judge"])
	}
	judges, ok := got["judges"].(map[string]any)
	if !ok {
		t.Fatalf("judges = %v, want one interval per judge", got["judges"])
	}
	want := map[string][3]float64{judgeHaiku: {0.88, 0.86, 0.90}, judgeSonnet: {0.72, 0.70, 0.74}}
	for id, w := range want {
		s, ok := judges[id].(map[string]any)
		if !ok {
			t.Fatalf("judges[%s] = %v", id, judges[id])
		}
		if s["mean"] != w[0] || s["min"] != w[1] || s["max"] != w[2] {
			t.Errorf("judges[%s] = %v, want mean %v min %v max %v", id, s, w[0], w[1], w[2])
		}
	}
	coverage := got["judgeCoverage"].(map[string]any)
	for _, id := range []string{judgeHaiku, judgeSonnet} {
		c := coverage[id].(map[string]any)
		if c["min"] != float64(2) || c["of"] != float64(2) {
			t.Errorf("judgeCoverage[%s] = %v, want 2 of 2", id, c)
		}
	}
	sample := got["perSample"].([]any)[0].(map[string]any)
	sj, ok := sample["judges"].(map[string]any)
	if !ok || sj[judgeSonnet].(map[string]any)["spread"] != 0.04 {
		t.Errorf("perSample judges = %v, want per-judge stats (sonnet spread 0.04)", sample["judges"])
	}
	cfg := got["config"].(map[string]any)
	if list, _ := cfg["judges"].([]any); len(list) != 2 {
		t.Errorf("config.judges = %v, want both judges", cfg["judges"])
	}
	if cfg["modelRequested"] != "sonnet" {
		t.Errorf("config.modelRequested = %v, want sonnet", cfg["modelRequested"])
	}
}

// TestTheJudgeSetIsInTheKey: the judges are the instrument. Both appear in the
// key, ordered by ID, so the order a run listed them in does not matter.
func TestTheJudgeSetIsInTheKey(t *testing.T) {
	root := t.TempDir()
	reversed := func(s map[string]any) {
		js := s["judges"].([]any)
		s["judges"] = []any{js[1], js[0]}
	}
	got, code := aggregate(t,
		fakeRunJudged(t, filepath.Join(root, "r1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "r2"), 0.78, twoJudges(0.86, 0.74), reversed),
	)
	if code != 0 {
		t.Fatalf("exit = %d — the same two judges in another order are the same instrument", code)
	}
	const want = "pyramidize|claude-code|claude-sonnet-5-5|claude-code+claude-code|claude-haiku-5-5+claude-sonnet-5-5|2|false|0.65|2|1|all|null+null|on|2.1.296"
	if got["configKey"] != want {
		t.Errorf("configKey = %v\nwant       %v", got["configKey"], want)
	}
}

// TestADifferentJudgeSetIsNotComparable: adding a third judge, or dropping one,
// changes the instrument — and a single-judge baseline from before the change
// must read "not comparable" rather than crash or compare one judge.
func TestADifferentJudgeSetIsNotComparable(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeJudgedBaseline(t, base,
		fakeRunJudged(t, filepath.Join(root, "b1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "b2"), 0.78, twoJudges(0.86, 0.74), nil))

	bridge := map[string]float64{judgeHaiku: 0.88, judgeSonnet: 0.72, "claude-sonnet-4-5-20250929": 0.80}
	got, code := aggregate(t, "--compare", base,
		fakeRunJudged(t, filepath.Join(root, "n1"), 0.80, bridge, nil),
		fakeRunJudged(t, filepath.Join(root, "n2"), 0.78, bridge, nil))
	if code != 2 || got["verdict"] != "not comparable: different configuration" {
		t.Errorf("three judges vs two: exit %d, verdict %v — want not comparable", code, got["verdict"])
	}

	// A single-judge baseline (API era, as test-data holds them) against
	// two-judge runs.
	old := filepath.Join(root, "old.json")
	writeBaseline(t, root, old)
	got, code = aggregate(t, "--compare", old,
		fakeRunJudged(t, filepath.Join(root, "m1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "m2"), 0.78, twoJudges(0.86, 0.74), nil))
	if code != 2 || got["verdict"] != "not comparable: different configuration" {
		t.Errorf("single-judge baseline vs two judges: exit %d, verdict %v — want not comparable", code, got["verdict"])
	}
}

// TestTheRecordedBaselinesReadNotComparable: every baseline in test-data was
// judged by one judge. Each must answer "not comparable" against a two-judge
// run, with exit 2 — not a jq error, not a verdict.
func TestTheRecordedBaselinesReadNotComparable(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "..", "test-data", "eval-baselines", "*", "baseline.json"))
	if len(files) == 0 {
		t.Skip("no recorded baselines")
	}
	root := t.TempDir()
	now := []string{
		fakeRunJudged(t, filepath.Join(root, "n1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "n2"), 0.78, twoJudges(0.86, 0.74), nil),
	}
	for _, f := range files {
		got, code := aggregate(t, append([]string{"--compare", f}, now...)...)
		if code != 2 || got["verdict"] != "not comparable: different configuration" {
			t.Errorf("%s: exit %d, verdict %v — want not comparable", f, code, got["verdict"])
		}
	}
}

// TestOneJudgeClearlyWorseIsARegression: the deterministic score and one judge
// overlap the baseline, the other judge's range sits entirely below it. Any
// metric clearly worse is a regression, and the verdict names which.
func TestOneJudgeClearlyWorseIsARegression(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeJudgedBaseline(t, base,
		fakeRunJudged(t, filepath.Join(root, "b1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "b2"), 0.78, twoJudges(0.86, 0.74), nil))

	got, code := aggregate(t, "--compare", base,
		fakeRunJudged(t, filepath.Join(root, "n1"), 0.79, twoJudges(0.88, 0.60), nil),
		fakeRunJudged(t, filepath.Join(root, "n2"), 0.80, twoJudges(0.87, 0.62), nil))
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if got["verdict"] != "regression: judge "+judgeSonnet+" regression" {
		t.Errorf("verdict = %v", got["verdict"])
	}
	verdicts, _ := got["judgeVerdicts"].(map[string]any)
	if verdicts[judgeHaiku] != "overlap" || verdicts[judgeSonnet] != "regression" {
		t.Errorf("judgeVerdicts = %v", got["judgeVerdicts"])
	}
}

// TestAnImprovementNeedsNoMetricWorse: better on one metric and worse on
// another is a regression, not a trade to net out; better on one and level on
// the rest is an improvement.
func TestAnImprovementNeedsNoMetricWorse(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeJudgedBaseline(t, base,
		fakeRunJudged(t, filepath.Join(root, "b1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "b2"), 0.78, twoJudges(0.86, 0.74), nil))

	got, code := aggregate(t, "--compare", base,
		fakeRunJudged(t, filepath.Join(root, "m1"), 0.95, twoJudges(0.97, 0.60), nil),
		fakeRunJudged(t, filepath.Join(root, "m2"), 0.96, twoJudges(0.98, 0.61), nil))
	if code != 1 || !strings.HasPrefix(got["verdict"].(string), "regression: judge "+judgeSonnet) {
		t.Errorf("mixed: exit %d, verdict %v — want a regression naming the Sonnet judge", code, got["verdict"])
	}

	got, code = aggregate(t, "--compare", base,
		fakeRunJudged(t, filepath.Join(root, "i1"), 0.95, twoJudges(0.88, 0.72), nil),
		fakeRunJudged(t, filepath.Join(root, "i2"), 0.96, twoJudges(0.87, 0.73), nil))
	if code != 0 || got["verdict"] != "improvement: deterministic improvement" {
		t.Errorf("better and level: exit %d, verdict %v — want an improvement", code, got["verdict"])
	}
}

// TestAJudgeThatSkippedASampleIsNamed: one judge dropping an answer leaves its
// mean over fewer samples, whatever the other judge did.
func TestAJudgeThatSkippedASampleIsNamed(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeJudgedBaseline(t, base,
		fakeRunJudged(t, filepath.Join(root, "b1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "b2"), 0.78, twoJudges(0.86, 0.74), nil))
	skipped := func(s map[string]any) { s["judgeCounts"].(map[string]any)[judgeHaiku] = 1 }
	got, code := aggregate(t, "--compare", base,
		fakeRunJudged(t, filepath.Join(root, "n1"), 0.79, twoJudges(0.88, 0.72), skipped),
		fakeRunJudged(t, filepath.Join(root, "n2"), 0.80, twoJudges(0.87, 0.73), nil))
	if code != 2 || got["verdict"] != "not comparable: judge "+judgeHaiku+" did not score every sample" {
		t.Errorf("exit %d, verdict %v", code, got["verdict"])
	}
}
