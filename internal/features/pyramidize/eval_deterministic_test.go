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
			result := checkNoHallucination(tc.input, tc.output)
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

// TestCoverageCountsOnlyTheAuthorsMessage: a reply that answers without
// restating the mails it answers is the intended behaviour since 2026-10-11.
// Scored on the whole input it lost every history term; scored on the
// author's message it is complete.
func TestCoverageCountsOnlyTheAuthorsMessage(t *testing.T) {
	reply := "Diagnose-Update Montag in der FinReg\n\nGrüße,\n\n**Diagnose-Update kommt am Montag in die FinReg**\n- Nach Rücksprache mit Ethan\n\nMfg\nMichael"

	card := RunDeterministicChecks(threadInput, threadAuthor, reply)
	coverage := checkNamed(t, card, "info_coverage")
	if coverage.Score != 1.0 || !coverage.Pass {
		t.Errorf("coverage of a reply without the history = %.2f (%s), want 1.00: history terms count against it", coverage.Score, coverage.Detail)
	}
	if whole := checkInfoCoverage(threadInput, reply); whole.Pass {
		t.Fatalf("the example is too weak: scored on the whole input it passes too (%s)", whole.Detail)
	}

	// The author's own facts still count: drop them and coverage falls.
	dropped := "Update\n\n**Ein Update kommt bald**\n- abgestimmt"
	if c := checkNamed(t, RunDeterministicChecks(threadInput, threadAuthor, dropped), "info_coverage"); c.Pass {
		t.Errorf("a reply that drops the author's facts passes coverage: %s", c.Detail)
	}
}

// TestAReferenceToTheHistoryIsNotInvented: the hallucination check still reads
// the whole input, so a reply that refers to a fact from the thread — Leo's
// "weiße Seite" under Citrix — is not charged with inventing it.
func TestAReferenceToTheHistoryIsNotInvented(t *testing.T) {
	reply := "Diagnose-Update Montag | Weiße Seite unter Citrix\n\n**Diagnose-Update soll die Weiße Seite unter Citrix klären**\n- Leo: Logs landen nicht im Backend\n- Mit Ethan abgestimmt, Montag in die FinReg"

	card := RunDeterministicChecks(threadInput, threadAuthor, reply)
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
