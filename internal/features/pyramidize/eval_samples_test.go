package pyramidize

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Mail history in a sample is context, never part of the output (ADR-003,
// addendum of 2026-10-11). The eval marks it so info coverage can be computed
// on the author's own message — and must never let the marker reach the model.

const pyramidSamplesDir = "../../../test-data/pyramidal-emails"

func sampleFile(raw, baseline string) string {
	return "# Raw Input\n```\n" + raw + "\n```\n\n# User accepted output\n```\n" + baseline + "\n```\n"
}

// TestHistoryMarkersSplitTheInput: history after the author's text (a reply
// with the thread below) and before it (quoted mails first) both come out of
// AuthorText, and neither marker survives in RawInput.
func TestHistoryMarkersSplitTheInput(t *testing.T) {
	for _, tc := range []struct {
		name, raw, wantInput, wantAuthor, wantHistory string
	}{
		{
			name:        "history below",
			raw:         "Hallo Leo,\nDeploy am Montag.\n\n<!-- history -->\n---- Mail historie:\nHallo, wir brauchen Logs.\nLeo\n<!-- /history -->",
			wantInput:   "Hallo Leo,\nDeploy am Montag.\n\n---- Mail historie:\nHallo, wir brauchen Logs.\nLeo",
			wantAuthor:  "Hallo Leo,\nDeploy am Montag.",
			wantHistory: "---- Mail historie:\nHallo, wir brauchen Logs.\nLeo",
		},
		{
			name:        "history above",
			raw:         "<!-- history -->\nHi Michael,\nbitte Forecast.\nAlex\n----\n<!-- /history -->\n\nHallo Alex\n- Forecast morgen",
			wantInput:   "Hi Michael,\nbitte Forecast.\nAlex\n----\n\nHallo Alex\n- Forecast morgen",
			wantAuthor:  "Hallo Alex\n- Forecast morgen",
			wantHistory: "Hi Michael,\nbitte Forecast.\nAlex\n----",
		},
		{
			name:        "two regions",
			raw:         "<!-- history -->\nerste Mail\n<!-- /history -->\nmeine Antwort\n<!-- history -->\nzweite Mail\n<!-- /history -->",
			wantInput:   "erste Mail\nmeine Antwort\nzweite Mail",
			wantAuthor:  "meine Antwort",
			wantHistory: "erste Mail\nzweite Mail",
		},
		{
			name:       "no history",
			raw:        "Hallo Alex,\nalles erledigt.",
			wantInput:  "Hallo Alex,\nalles erledigt.",
			wantAuthor: "Hallo Alex,\nalles erledigt.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, ok, err := parseTestData(sampleFile(tc.raw, "accepted"))
			if err != nil || !ok {
				t.Fatalf("parseTestData: ok=%v err=%v", ok, err)
			}
			if s.RawInput != tc.wantInput {
				t.Errorf("RawInput = %q\nwant       %q", s.RawInput, tc.wantInput)
			}
			if s.AuthorText != tc.wantAuthor {
				t.Errorf("AuthorText = %q\nwant         %q", s.AuthorText, tc.wantAuthor)
			}
			if s.History != tc.wantHistory {
				t.Errorf("History = %q\nwant      %q", s.History, tc.wantHistory)
			}
			if s.Baseline != "accepted" {
				t.Errorf("Baseline = %q", s.Baseline)
			}
			if s.HasHistory() != (tc.wantHistory != "") {
				t.Errorf("HasHistory = %v", s.HasHistory())
			}
		})
	}
}

// TestAMalformedMarkingIsRefused: every way a marking can be wrong is an error,
// because the alternative is a marker sent to the model or a coverage number
// computed on the wrong text.
func TestAMalformedMarkingIsRefused(t *testing.T) {
	for name, file := range map[string]string{
		"unclosed":           sampleFile("mine\n<!-- history -->\ntheirs", "ok"),
		"close without open": sampleFile("mine\n<!-- /history -->", "ok"),
		"nested":             sampleFile("<!-- history -->\na\n<!-- history -->\nb\n<!-- /history -->\n<!-- /history -->\nmine", "ok"),
		"empty region":       sampleFile("mine\n<!-- history -->\n\n<!-- /history -->", "ok"),
		"all history":        sampleFile("<!-- history -->\ntheirs\n<!-- /history -->", "ok"),
		"marker in output":   sampleFile("mine", "<!-- history -->\nx\n<!-- /history -->"),
		// Near misses: each would otherwise pass as plain text, reach the
		// model and leave the history counted as the author's message.
		"no spaces":                sampleFile("mine\n<!--history-->\ntheirs\n<!--/history-->", "ok"),
		"capitalised":              sampleFile("mine\n<!-- History -->\ntheirs\n<!-- /History -->", "ok"),
		"inline open":              sampleFile("mine <!-- history -->\ntheirs\n<!-- /history -->", "ok"),
		"inline close":             sampleFile("mine\n<!-- history -->\ntheirs <!-- /history -->", "ok"),
		"near miss in output":      sampleFile("mine", "ok <!--history-->"),
		"marker outside the fence": "# Raw Input\n<!-- history -->\n```\nmine\n```\n<!-- /history -->\n\n# User accepted output\n```\nok\n```\n",
	} {
		if _, _, err := parseTestData(file); err == nil {
			t.Errorf("%s: parsed without an error", name)
		}
	}
}

// TestTheMarkedSamplesSendWhatTheUserSelected: the two samples that carry mail
// history are marked, and their RawInput is byte for byte what it was before
// the markers went in. The hashes were taken from main's parser (25755ef) on
// the unmarked files — not from this code, which would pin whatever it does.
func TestTheMarkedSamplesSendWhatTheUserSelected(t *testing.T) {
	samples, _, err := loadEvalSamples(pyramidSamplesDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{ rawSHA, authorHas, authorLacks string }{
		"email-diagnose-update": {
			rawSHA:      "1827d0e3fd5a3057670e4033824eb9aca8fc064a4a25c11169c1c7c81f0889c8",
			authorHas:   "Diagnose-Update in die FinReg deployen",
			authorLacks: "Notwendigkeit von Logs",
		},
		"email-dataquality-reply-to-feedback": {
			rawSHA:      "a5ebfe5c7f5a5cb7388a66568ff37f8a6a70bcbd09aff415cb2a71d72745259b",
			authorHas:   "die kpi liste für Jordan komme ich auch erst morgen zu",
			authorLacks: "Liebe Grüße",
		},
	}
	seen := map[string]bool{}
	for _, s := range samples {
		w, marked := want[s.Name]
		if s.HasHistory() != marked {
			t.Errorf("%s: HasHistory = %v, want %v", s.Name, s.HasHistory(), marked)
		}
		if !marked {
			continue
		}
		seen[s.Name] = true
		sum := sha256.Sum256([]byte(s.RawInput))
		if got := hex.EncodeToString(sum[:]); got != w.rawSHA {
			t.Errorf("%s: the raw input changed (sha256 %s): the marking must not alter what the user selected", s.Name, got)
		}
		if !strings.Contains(s.AuthorText, w.authorHas) {
			t.Errorf("%s: the author's text lacks %q", s.Name, w.authorHas)
		}
		if strings.Contains(s.AuthorText, w.authorLacks) {
			t.Errorf("%s: the author's text still holds history (%q)", s.Name, w.authorLacks)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("sample %s not found", name)
		}
	}
}

// TestNoMarkerReachesThePipeline: whatever a sample marks, what the eval sends
// to Pyramidize and to the judges holds no marker.
func TestNoMarkerReachesThePipeline(t *testing.T) {
	samples, skipped, err := loadEvalSamples(pyramidSamplesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) > 0 {
		t.Errorf("samples without a raw input: %v", skipped)
	}
	matches, _ := filepath.Glob(filepath.Join(pyramidSamplesDir, "*.md"))
	if len(samples) != len(matches) || len(samples) == 0 {
		t.Fatalf("parsed %d of %d sample files", len(samples), len(matches))
	}
	for _, s := range samples {
		for field, text := range map[string]string{"RawInput": s.RawInput, "Baseline": s.Baseline} {
			if strings.Contains(text, "<!--") || strings.Contains(text, "history -->") {
				t.Errorf("%s: %s holds a history marker", s.Name, field)
			}
		}
	}
}

// TestEvalHumanUsesTheGoParser: scripts/eval-human.sh pipes each sample's
// raw input to bin/KeyLint -pyramidize. Its own sed parsing read 6 of the 13
// samples (case-sensitive headers, the "accpted" typo, trailing spaces) and
// dropped "# " and ``` lines inside the input. It now asks this parser, through
// its dump_samples function: run here, every sample must come out non-empty,
// marker-free and byte for byte the RawInput the eval sends.
func TestEvalHumanUsesTheGoParser(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the eval-tagged package")
	}
	if runtime.GOOS == "windows" {
		t.Skip("eval-human.sh is bash; CI runs it on Linux")
	}
	for _, tool := range []string{"bash", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repo, "scripts", "eval-human.sh"))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(script), "dump_samples() {")
	if start < 0 {
		t.Fatal("eval-human.sh has no dump_samples function")
	}
	end := strings.Index(string(script)[start:], "\n}\n")
	if end < 0 {
		t.Fatal("eval-human.sh's dump_samples function does not end")
	}
	fn := string(script)[start : start+end+3]

	dump := t.TempDir()
	cmd := exec.Command("bash", "-c", fn+`dump_samples "$1"`, "bash", dump)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("dump_samples: %v\n%s", err, out)
	}

	samples, _, err := loadEvalSamples(pyramidSamplesDir)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(pyramidSamplesDir, "*.md"))
	if len(samples) != len(files) {
		t.Fatalf("the Go parser read %d of %d samples", len(samples), len(files))
	}
	raws, _ := filepath.Glob(filepath.Join(dump, "*.raw"))
	if len(raws) != len(samples) {
		t.Errorf("dump_samples wrote %d raw inputs, want %d", len(raws), len(samples))
	}
	for _, s := range samples {
		raw, err := os.ReadFile(filepath.Join(dump, s.Name+".raw"))
		if err != nil {
			t.Errorf("%s: %v", s.Name, err)
			continue
		}
		accepted, err := os.ReadFile(filepath.Join(dump, s.Name+".accepted"))
		if err != nil {
			t.Errorf("%s: %v", s.Name, err)
			continue
		}
		if len(raw) == 0 || len(accepted) == 0 {
			t.Errorf("%s: empty raw input or accepted output", s.Name)
		}
		if string(raw) != s.RawInput {
			t.Errorf("%s: the raw input eval-human.sh sends differs from the eval's", s.Name)
		}
		if string(accepted) != s.Baseline {
			t.Errorf("%s: the accepted output eval-human.sh shows differs from the eval's", s.Name)
		}
		if historyMarkerLike.Match(raw) {
			t.Errorf("%s: eval-human.sh would send a history marker", s.Name)
		}
	}
}
