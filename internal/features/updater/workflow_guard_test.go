package updater

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Clients up to v4.1.8-alpha (and v3.5.0) update from
// releases/latest/download/latest.json. A dev-channel prerelease that became
// "latest" would hand them an unreviewed PR build — so every place the
// workflow creates or edits one must say --prerelease --latest=false.
func TestPublishDevBuild_NeverMarksAReleaseLatest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "build-linux.yml"))
	if err != nil {
		t.Fatalf("reading the workflow: %v", err)
	}
	// Join shell line continuations, so a flag on the next line still counts.
	text := strings.ReplaceAll(string(data), "\\\n", " ")

	calls := regexp.MustCompile(`gh release (create|edit) [^\n]*`).FindAllString(text, -1)
	if len(calls) < 2 {
		t.Fatalf("found %d `gh release create/edit` calls, want the publish step's create and edit", len(calls))
	}
	for _, c := range calls {
		if !strings.Contains(c, "--latest=false") || !strings.Contains(c, "--prerelease") {
			t.Errorf("missing --prerelease --latest=false:\n  %s", strings.TrimSpace(c))
		}
	}
}
