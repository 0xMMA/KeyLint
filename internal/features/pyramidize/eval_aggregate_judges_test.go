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
	const want = "pyramidize|claude-code|claude-sonnet-5-5|claude-code+claude-code|claude-haiku-5-5+claude-sonnet-5-5|2|false|0.65|2|1|all|null+null|on|unrecorded"
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
	// No metric is named as flagging a comparison that was not made.
	if flagged, _ := got["regressedMetrics"].([]any); len(flagged) != 0 {
		t.Errorf("regressedMetrics = %v on a non-comparison", flagged)
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
	// Named on its own, so a one-judge regression reads as one.
	if flagged, _ := got["regressedMetrics"].([]any); len(flagged) != 1 || flagged[0] != "judge "+judgeSonnet {
		t.Errorf("regressedMetrics = %v, want only the Sonnet judge", got["regressedMetrics"])
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

// fakeRunSpelled is a one-judge run whose judge was requested under one name
// and answered under another — EVAL_JUDGES=haiku against claude-haiku-5-5.
// Everything the Go side writes is keyed by the requested name.
func fakeRunSpelled(t *testing.T, dir, requested, resolved string, det, score float64) string {
	t.Helper()
	dir = fakeRunJudged(t, dir, det, map[string]float64{requested: score}, func(s map[string]any) {
		s["judges"].([]any)[0].(map[string]any)["resolvedModel"] = resolved
	})
	return dir
}

// TestAJudgeIsTrackedByTheIDThatAnswered: the key names the resolved judge,
// so the columns must too. Keyed by the requested name, a baseline judged as
// "haiku" and a run judged as "claude-haiku-5-5" share a key but no column,
// and a collapse from 0.95 to 0.40 came back "inconclusive", exit 0.
func TestAJudgeIsTrackedByTheIDThatAnswered(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeJudgedBaseline(t, base,
		fakeRunSpelled(t, filepath.Join(root, "b1"), "haiku", judgeHaiku, 0.80, 0.95),
		fakeRunSpelled(t, filepath.Join(root, "b2"), "haiku", judgeHaiku, 0.79, 0.96))

	got, code := aggregate(t, "--compare", base,
		fakeRunSpelled(t, filepath.Join(root, "n1"), judgeHaiku, judgeHaiku, 0.80, 0.40),
		fakeRunSpelled(t, filepath.Join(root, "n2"), judgeHaiku, judgeHaiku, 0.79, 0.41))
	if code != 1 || got["verdict"] != "regression: judge "+judgeHaiku+" regression" {
		t.Errorf("exit %d, verdict %v — want the judge's regression", code, got["verdict"])
	}
}

// TestMixedSpellingsAggregateIntoOneJudge: two runs of the same judge under two
// names are one judge with a two-run interval, not two judges with one run each.
func TestMixedSpellingsAggregateIntoOneJudge(t *testing.T) {
	root := t.TempDir()
	got, code := aggregate(t,
		fakeRunSpelled(t, filepath.Join(root, "r1"), "haiku", judgeHaiku, 0.80, 0.90),
		fakeRunSpelled(t, filepath.Join(root, "r2"), judgeHaiku, judgeHaiku, 0.78, 0.86))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	judges := got["judges"].(map[string]any)
	if len(judges) != 1 {
		t.Fatalf("judges = %v, want one column", judges)
	}
	s, _ := judges[judgeHaiku].(map[string]any)
	if s == nil || s["min"] != 0.86 || s["max"] != 0.90 {
		t.Errorf("judges[%s] = %v, want the interval over both runs", judgeHaiku, s)
	}
	sample := got["perSample"].([]any)[0].(map[string]any)
	if ps, _ := sample["judges"].(map[string]any)[judgeHaiku].(map[string]any); ps == nil || ps["spread"] != 0.04 {
		t.Errorf("perSample judges = %v, want both runs in one column", sample["judges"])
	}
}

// TestAMissingIntervalIsNotComparable: equal keys and full coverage, yet one
// side has no interval for a judge. That is a broken record, and reading it as
// "overlap" would hide a regression; it must say "not comparable".
func TestAMissingIntervalIsNotComparable(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeJudgedBaseline(t, base,
		fakeRunJudged(t, filepath.Join(root, "b1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "b2"), 0.78, twoJudges(0.86, 0.74), nil))
	data, _ := os.ReadFile(base)
	var doc map[string]any
	_ = json.Unmarshal(data, &doc)
	delete(doc["judges"].(map[string]any), judgeSonnet)
	data, _ = json.Marshal(doc)
	_ = os.WriteFile(base, data, 0o644)

	got, code := aggregate(t, "--compare", base,
		fakeRunJudged(t, filepath.Join(root, "n1"), 0.79, twoJudges(0.88, 0.40), nil),
		fakeRunJudged(t, filepath.Join(root, "n2"), 0.80, twoJudges(0.87, 0.41), nil))
	if code != 2 || got["verdict"] != "not comparable: no interval for judge "+judgeSonnet+" on one side" {
		t.Errorf("exit %d, verdict %v", code, got["verdict"])
	}
}

// TestARefusedComparisonFlagsNoMetric: different configurations, and the new
// deterministic range sits wholly below the old. The verdict is "not
// comparable", so no metric may be reported as having flagged a regression.
func TestARefusedComparisonFlagsNoMetric(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "baseline.json")
	writeJudgedBaseline(t, base,
		fakeRunJudged(t, filepath.Join(root, "b1"), 0.80, twoJudges(0.90, 0.70), nil),
		fakeRunJudged(t, filepath.Join(root, "b2"), 0.78, twoJudges(0.86, 0.74), nil))
	other := func(s map[string]any) { s["checksVersion"] = 9 }
	got, code := aggregate(t, "--compare", base,
		fakeRunJudged(t, filepath.Join(root, "n1"), 0.50, twoJudges(0.90, 0.70), other),
		fakeRunJudged(t, filepath.Join(root, "n2"), 0.51, twoJudges(0.86, 0.74), other))
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if flagged, _ := got["regressedMetrics"].([]any); len(flagged) != 0 {
		t.Errorf("regressedMetrics = %v on a refused comparison", flagged)
	}
}

// v3Run is a run with the configuration of the recorded v3 baseline
// (2026-10-11T22-39-34): v3 email, 13 samples, both judges on all of them.
func v3Run(t *testing.T, dir string, det float64, edit func(map[string]any)) string {
	t.Helper()
	return fakeRunJudged(t, dir, det, twoJudges(0.85, 0.79), func(s map[string]any) {
		s["promptVariant"] = 3
		s["sampleCount"] = 13
		s["judgeCounts"] = map[string]any{judgeHaiku: 13, judgeSonnet: 13}
		if edit != nil {
			edit(s)
		}
	})
}

// TestTheMailHistoryInstrumentIsNotComparableWithV3sBaseline: since
// 2026-10-11 info coverage reads only the author's message (checksVersion 2)
// and the judges are told that mail history is context. Both change what a
// score means, so the recorded v3 baseline — scored the old way — must read
// "not comparable", whichever of the two a run differs in. The control run
// shows the refusal comes from the instrument and nothing else.
func TestTheMailHistoryInstrumentIsNotComparableWithV3sBaseline(t *testing.T) {
	base := filepath.Join("..", "..", "..", "test-data", "eval-baselines", "2026-10-11T22-39-34", "baseline.json")
	if _, err := os.Stat(base); err != nil {
		t.Skipf("the v3 baseline is not in this checkout: %v", err)
	}
	root := t.TempDir()
	compare := func(name string, edit func(map[string]any)) (string, int) {
		got, code := aggregate(t, "--compare", base,
			v3Run(t, filepath.Join(root, name+"1"), 0.92, edit),
			v3Run(t, filepath.Join(root, name+"2"), 0.93, edit))
		v, _ := got["verdict"].(string)
		return v, code
	}

	if v, code := compare("old", nil); code == 2 || strings.HasPrefix(v, "not comparable") {
		t.Fatalf("control: a run on the old instrument should compare with the v3 baseline, got exit %d %q", code, v)
	}
	for name, edit := range map[string]func(map[string]any){
		"instrument": func(s map[string]any) {
			for k, v := range instrumentSummary() {
				s[k] = v
			}
		},
		"checks": func(s map[string]any) { s["checksVersion"] = ChecksVersion },
		"judge":  func(s map[string]any) { s["judgePromptHash"] = judgePromptHash() },
	} {
		if v, code := compare(name, edit); code != 2 || v != "not comparable: different configuration" {
			t.Errorf("%s: exit %d, verdict %q — want not comparable", name, code, v)
		}
	}
}

// TestTheJudgePromptIsInTheKey: two runs whose judges read different prompts
// were scored by different instruments, and their mean describes neither. The
// baseline records the hash, so a reader can see which prompt scored it.
func TestTheJudgePromptIsInTheKey(t *testing.T) {
	root := t.TempDir()
	with := func(hash string) func(map[string]any) {
		return func(s map[string]any) { s["judgePromptHash"] = hash }
	}
	got, code := aggregate(t,
		fakeRunJudged(t, filepath.Join(root, "a1"), 0.80, twoJudges(0.90, 0.70), with("j1-aaaa")),
		fakeRunJudged(t, filepath.Join(root, "a2"), 0.78, twoJudges(0.86, 0.74), with("j1-aaaa")),
	)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if key, _ := got["configKey"].(string); !strings.HasSuffix(key, "|j1-aaaa") {
		t.Errorf("configKey = %q, want the judge prompt hash as its last field", key)
	}
	if cfg := got["config"].(map[string]any); cfg["judgePromptHash"] != "j1-aaaa" {
		t.Errorf("config.judgePromptHash = %v", cfg["judgePromptHash"])
	}
	if _, code := aggregate(t,
		fakeRunJudged(t, filepath.Join(root, "b1"), 0.80, twoJudges(0.90, 0.70), with("j1-aaaa")),
		fakeRunJudged(t, filepath.Join(root, "b2"), 0.78, twoJudges(0.86, 0.74), with("j1-bbbb")),
	); code != 2 {
		t.Errorf("runs judged by two different prompts averaged together: exit %d, want 2", code)
	}
}
