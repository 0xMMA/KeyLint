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
	input := "Meeting mit Hans und Maria über das Projekt Alpha."

	tests := []struct {
		name   string
		output string
		wantOK bool
	}{
		{
			name:   "no new proper nouns",
			output: "## Kernergebnis\nHans und Maria besprechen Projekt Alpha.",
			wantOK: true,
		},
		{
			name:   "introduces new proper noun",
			output: "## Kernergebnis\nHans, Maria und Stefan besprechen Projekt Alpha und Beta.",
			wantOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := checkNoHallucination(input, tc.output)
			if result.Pass != tc.wantOK {
				t.Errorf("checkNoHallucination() Pass = %v, want %v; detail: %s", result.Pass, tc.wantOK, result.Detail)
			}
		})
	}
}
