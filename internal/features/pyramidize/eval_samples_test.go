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
		name, raw, wantInput, wantAuthor string
	}{
		{
			name:       "history below",
			raw:        "Hallo Leo,\nDeploy am Montag.\n\n<!-- history -->\n---- Mail historie:\nHallo, wir brauchen Logs.\nLeo\n<!-- /history -->",
			wantInput:  "Hallo Leo,\nDeploy am Montag.\n\n---- Mail historie:\nHallo, wir brauchen Logs.\nLeo",
			wantAuthor: "Hallo Leo,\nDeploy am Montag.",
		},
		{
			name:       "history above",
			raw:        "<!-- history -->\nHi Michael,\nbitte Forecast.\nAlex\n----\n<!-- /history -->\n\nHallo Alex\n- Forecast morgen",
			wantInput:  "Hi Michael,\nbitte Forecast.\nAlex\n----\n\nHallo Alex\n- Forecast morgen",
			wantAuthor: "Hallo Alex\n- Forecast morgen",
		},
		{
			name:       "two regions",
			raw:        "<!-- history -->\nerste Mail\n<!-- /history -->\nmeine Antwort\n<!-- history -->\nzweite Mail\n<!-- /history -->",
			wantInput:  "erste Mail\nmeine Antwort\nzweite Mail",
			wantAuthor: "meine Antwort",
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
			if s.Baseline != "accepted" {
				t.Errorf("Baseline = %q", s.Baseline)
			}
			if s.HasHistory() != (tc.wantInput != tc.wantAuthor) {
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

// TestEvalHumanSendsNoMarker: scripts/eval-human.sh parses the samples in
// shell and pipes the raw input to bin/KeyLint -pyramidize. Its raw_input
// function is run on the marked reply sample (whose headers it can read) and
// must hand on the text without a marker line.
func TestEvalHumanSendsNoMarker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("eval-human.sh is bash; CI runs it on Linux")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "eval-human.sh"))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(script), "raw_input() {")
	if start < 0 {
		t.Fatal("eval-human.sh has no raw_input function")
	}
	end := strings.Index(string(script)[start:], "\n}\n")
	if end < 0 {
		t.Fatal("eval-human.sh's raw_input function does not end")
	}
	fn := string(script)[start : start+end+3]
	sample, err := filepath.Abs(filepath.Join(pyramidSamplesDir, "email-dataquality-reply-to-feedback.md"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", "-c", fn+`raw_input "$1"`, "bash", sample).Output()
	if err != nil {
		t.Fatalf("raw_input: %v", err)
	}
	if !strings.Contains(string(out), "Im nachfolgenden die erste mail") || !strings.Contains(string(out), "versuch aus meinen vorgeschriebenen infos") {
		t.Fatalf("raw_input did not return the sample's raw input:\n%s", out)
	}
	if strings.Contains(string(out), "<!--") {
		t.Errorf("eval-human.sh sends a history marker to the model:\n%s", out)
	}
}
