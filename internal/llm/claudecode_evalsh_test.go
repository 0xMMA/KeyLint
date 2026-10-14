package llm

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestEvalShStripsWhatTheCLIClientStrips: scripts/eval.sh and
// scripts/website/refresh-samples.sh check the sign-in with
// `env -u … claude auth status` before they spend calls. If a list drifts
// from blockedCLIEnv, the check sees a different account than every call —
// a signed-in API-key session that the calls, which never see the key, then
// fail as "not signed in". Kept in sync by this test rather than by hand.
func TestEvalShStripsWhatTheCLIClientStrips(t *testing.T) {
	for _, script := range []string{
		filepath.Join("scripts", "eval.sh"),
		filepath.Join("scripts", "website", "refresh-samples.sh"),
	} {
		t.Run(script, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", script))
			if os.IsNotExist(err) {
				t.Skipf("%s is not in this tree", script)
			}
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			start := strings.Index(text, "env -u ")
			end := strings.Index(text, "claude auth status")
			if start < 0 || end < start {
				t.Fatalf("%s has no `env -u … claude auth status` pre-check", script)
			}
			var unset []string
			for _, m := range regexp.MustCompile(`-u\s+([A-Z_][A-Z0-9_]*)`).FindAllStringSubmatch(text[start:end], -1) {
				unset = append(unset, m[1])
			}
			want := slices.Clone(blockedCLIEnv)
			slices.Sort(want)
			slices.Sort(unset)
			if !slices.Equal(unset, want) {
				t.Errorf("%s unsets %v\nblockedCLIEnv is %v", script, unset, want)
			}
		})
	}
}
