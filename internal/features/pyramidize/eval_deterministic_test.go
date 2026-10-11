package pyramidize

import "testing"

// --- checkStructure ---

func TestCheckStructure(t *testing.T) {
	tests := []struct {
		name   string
		output string
		wantOK bool
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
			wantOK: true,
		},
		{
			name:   "bold headers instead of markdown headers",
			output: "CarService-Fix erfolgreich deployed\n\n**Kernergebnis**\nDeploy war erfolgreich.\n\n**Nächste Schritte**\nMonitoring prüfen.",
			wantOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := checkStructure(tc.output)
			if result.Pass != tc.wantOK {
				t.Errorf("checkStructure() Pass = %v, want %v; detail: %s", result.Pass, tc.wantOK, result.Detail)
			}
		})
	}
}

// --- checkInfoCoverage ---

func TestCheckInfoCoverage(t *testing.T) {
	input := "Die Contoso Bank hat beschlossen von Adverity zu Airbyte zu wechseln. Jordan Fischer ist der externe Berater."

	tests := []struct {
		name   string
		output string
		wantOK bool
	}{
		{
			name:   "all key terms present",
			output: "Contoso Bank wechselt von Adverity zu Airbyte. Jordan Fischer berät extern.",
			wantOK: true,
		},
		{
			name:   "missing key entities",
			output: "Eine Bank wechselt die Datenplattform. Ein externer Berater unterstützt.",
			wantOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := checkInfoCoverage(input, tc.output)
			if result.Pass != tc.wantOK {
				t.Errorf("checkInfoCoverage() Pass = %v, want %v; detail: %s", result.Pass, tc.wantOK, result.Detail)
			}
		})
	}
}

// --- checkNoHallucination ---

func TestCheckNoHallucination(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
		wantOK bool
	}{
		{
			name:   "no new proper nouns",
			input:  "Meeting mit Hans und Maria über das Projekt Alpha.",
			output: "## Kernergebnis\nHans und Maria besprechen Projekt Alpha.",
			wantOK: true,
		},
		{
			name:   "introduces new proper noun",
			input:  "Meeting mit Hans und Maria über das Projekt Alpha.",
			output: "## Kernergebnis\nHans, Maria und Stefan besprechen Projekt Alpha und Beta.",
			wantOK: false,
		},
		{
			name:   "German compound words from input parts are not hallucinations",
			input:  "Wir haben Datenqualität verbessert und mehrere Fixes deployed.",
			output: "## Kernergebnis\nDatenqualitäts-Fixes wurden deployed.",
			wantOK: true,
		},
		{
			name:   "common German structural words are not hallucinations",
			input:  "Projekt Alpha startet nächste Woche.",
			output: "**Hinweis**\nProjekt Alpha startet bald.\n\n**Nächste Schritte**\nThemen klären.",
			wantOK: true,
		},
		{
			name:   "lowercase input word capitalized in output is not a hallucination",
			input:  "der status der migration ist unklar.",
			output: "## Kernergebnis\nStatus der Migration ist unklar.",
			wantOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := checkNoHallucination(tc.input, "", tc.output)
			if result.Pass != tc.wantOK {
				t.Errorf("checkNoHallucination() Pass = %v, want %v; detail: %s", result.Pass, tc.wantOK, result.Detail)
			}
		})
	}
}

// --- mail history (ChecksVersion 2) ---

// A reply with the thread below it. The author's message names Ethan, the
// FinReg and Montag; the history names Leo, Citrix, Tableau and the backend.
const (
	threadAuthor  = "Grüße,\nNach Rücksprache mit Ethan deployen wir das Diagnose-Update am Montag in die FinReg.\nMfg\nMichael"
	threadHistory = "---- Mail historie:\nHallo Ethan, die Extension zeigt unter Tableau Desktop (Citrix) nur eine weiße Seite. Die Logs landen nicht im Backend.\nViele Grüße\nLeo"
	threadInput   = threadAuthor + "\n\n" + threadHistory
)

var threadEvalInput = EvalInput{Raw: threadInput, Author: threadAuthor, History: threadHistory}

// TestCoverageCountsOnlyTheAuthorsMessage: a reply that answers without
// restating the mails it answers is the intended behaviour since 2026-10-11.
// Scored on the whole input it lost every history term; scored on the
// author's message it is complete.
func TestCoverageCountsOnlyTheAuthorsMessage(t *testing.T) {
	reply := "Diagnose-Update Montag in der FinReg\n\nGrüße,\n\n**Diagnose-Update kommt am Montag in die FinReg**\n- Nach Rücksprache mit Ethan\n\nMfg\nMichael"

	card := RunDeterministicChecks(threadEvalInput, reply)
	coverage := checkNamed(t, card, "info_coverage")
	if coverage.Score != 1.0 || !coverage.Pass {
		t.Errorf("coverage of a reply without the history = %.2f (%s), want 1.00: history terms count against it", coverage.Score, coverage.Detail)
	}
	if whole := checkInfoCoverage(threadInput, reply); whole.Pass {
		t.Fatalf("the example is too weak: scored on the whole input it passes too (%s)", whole.Detail)
	}

	// The author's own facts still count: drop them and coverage falls.
	dropped := "Update\n\n**Ein Update kommt bald**\n- abgestimmt"
	if c := checkNamed(t, RunDeterministicChecks(threadEvalInput, dropped), "info_coverage"); c.Pass {
		t.Errorf("a reply that drops the author's facts passes coverage: %s", c.Detail)
	}
}

// TestAReferenceToTheHistoryIsNotInvented: the hallucination check still reads
// the whole input, so a reply that refers to a fact from the thread — Leo's
// "weiße Seite" under Citrix — is not charged with inventing it.
func TestAReferenceToTheHistoryIsNotInvented(t *testing.T) {
	reply := "Diagnose-Update Montag | Weiße Seite unter Citrix\n\n**Diagnose-Update soll die Weiße Seite unter Citrix klären**\n- Leo: Logs landen nicht im Backend\n- Mit Ethan abgestimmt, Montag in die FinReg"

	card := RunDeterministicChecks(threadEvalInput, reply)
	if h := checkNamed(t, card, "no_hallucination"); h.Score != 1.0 {
		t.Errorf("a reference to the thread scored as invented: %.2f (%s)", h.Score, h.Detail)
	}
}

// TestTheChecksVersionNamesTheHistoryChange: coverage on the author's message
// is a different instrument from coverage on the whole input, so the version
// moved with it, and a run records it.
func TestTheChecksVersionNamesTheHistoryChange(t *testing.T) {
	if ChecksVersion != 2 {
		t.Errorf("ChecksVersion = %d, want 2 (1 = coverage on the whole input, up to 2026-10-11)", ChecksVersion)
	}
	fields := instrumentSummary()
	if fields["checksVersion"] != ChecksVersion {
		t.Errorf("summary.json records checksVersion %v, want %d", fields["checksVersion"], ChecksVersion)
	}
}

func checkNamed(t *testing.T, card EvalScorecard, name string) CheckResult {
	t.Helper()
	for _, c := range card.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s check in the scorecard", name)
	return CheckResult{}
}

// --- no_history_reproduction ---

// TestReproducingTheHistoryScoresLower: the output must not reproduce the
// mail history, so appending it to the accepted output has to cost score. It
// used to earn some: the hallucination ratio was taken over every output term,
// and the copied history's terms — all "found in the input" — diluted it.
func TestReproducingTheHistoryScoresLower(t *testing.T) {
	samples, _, err := loadEvalSamples(pyramidSamplesDir)
	if err != nil {
		t.Fatal(err)
	}
	marked := 0
	for _, s := range samples {
		if !s.HasHistory() {
			continue
		}
		marked++
		t.Run(s.Name, func(t *testing.T) {
			accepted := RunDeterministicChecks(s.Input(), s.Baseline)
			recap := RunDeterministicChecks(s.Input(), s.Baseline+"\n\n"+s.History)
			if accepted.OverallScore <= recap.OverallScore {
				t.Errorf("accepted output %.3f, accepted + history %.3f: reproducing the history must score lower", accepted.OverallScore, recap.OverallScore)
			}
			if c := checkNamed(t, accepted, "no_history_reproduction"); !c.Pass {
				t.Errorf("the accepted output fails no_history_reproduction: %s", c.Detail)
			}
			if c := checkNamed(t, recap, "no_history_reproduction"); c.Pass {
				t.Errorf("accepted output + history passes no_history_reproduction: %s", c.Detail)
			}
			a, r := checkNamed(t, accepted, "no_hallucination"), checkNamed(t, recap, "no_hallucination")
			if r.Score > a.Score {
				t.Errorf("copied history improves no_hallucination: %.3f -> %.3f", a.Score, r.Score)
			}
			t.Logf("overall %.3f -> %.3f; %s | %s", accepted.OverallScore, recap.OverallScore,
				checkNamed(t, accepted, "no_history_reproduction").Detail, checkNamed(t, recap, "no_history_reproduction").Detail)
		})
	}
	if marked != 2 {
		t.Errorf("%d marked samples, want 2", marked)
	}
}

// TestAReferencePassesAQuoteFails: a reply that refers to a fact from the
// thread keeps a phrase or a name; a quoted original keeps whole sentences.
func TestAReferencePassesAQuoteFails(t *testing.T) {
	reference := "Diagnose-Update Montag | Weiße Seite unter Citrix\n\n**Diagnose-Update soll die Weiße Seite unter Citrix klären**\n- Leo: Logs landen nicht im Backend\n- Mit Ethan abgestimmt, Montag in die FinReg"
	if c := checkNamed(t, RunDeterministicChecks(threadEvalInput, reference), "no_history_reproduction"); !c.Pass || c.Score < 0.9 {
		t.Errorf("a reference to the thread fails: %.2f (%s)", c.Score, c.Detail)
	}
	quote := reference + "\n\n> Hallo Ethan, die Extension zeigt unter Tableau Desktop (Citrix) nur eine weiße Seite. Die Logs landen nicht im Backend."
	if c := checkNamed(t, RunDeterministicChecks(threadEvalInput, quote), "no_history_reproduction"); c.Pass {
		t.Errorf("a quoted sentence of the thread passes: %s", c.Detail)
	}
	// Without history there is nothing to reproduce, and no check for it.
	card := RunDeterministicChecks(EvalInput{Raw: threadAuthor, Author: threadAuthor}, reference)
	for _, c := range card.Checks {
		if c.Name == "no_history_reproduction" {
			t.Error("a sample without history is scored on history reproduction")
		}
	}
}

// TestCopiedHistoryDoesNotDiluteInventedTerms: an output with invented names
// scores the same on no_hallucination whether or not the history is pasted
// after it.
func TestCopiedHistoryDoesNotDiluteInventedTerms(t *testing.T) {
	invented := "Update\n\n**Stefan und Petra deployen Projekt Orion**\n- Ethan informiert"
	plain := checkNamed(t, RunDeterministicChecks(threadEvalInput, invented), "no_hallucination")
	padded := checkNamed(t, RunDeterministicChecks(threadEvalInput, invented+"\n\n"+threadHistory), "no_hallucination")
	if padded.Score != plain.Score {
		t.Errorf("no_hallucination %.3f alone, %.3f with the history pasted after it", plain.Score, padded.Score)
	}
	if plain.Pass {
		t.Fatalf("the example is too weak: invented names pass (%s)", plain.Detail)
	}
}
