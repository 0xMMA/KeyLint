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
