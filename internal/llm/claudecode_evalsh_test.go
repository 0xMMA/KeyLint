package llm

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestEvalShStripsWhatTheCLIClientStrips: scripts/eval.sh checks the sign-in
// with `env -u … claude auth status` before a run. If its list drifts from
// blockedCLIEnv, the check sees a different account than every eval call —
// a signed-in API-key session that the calls, which never see the key, then
// fail as "not signed in". Kept in sync by this test rather than by hand.
func TestEvalShStripsWhatTheCLIClientStrips(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "eval.sh"))
	if os.IsNotExist(err) {
		t.Skip("scripts/eval.sh is not in this tree")
	}
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	start := strings.Index(script, "env -u ")
	end := strings.Index(script, "claude auth status")
	if start < 0 || end < start {
		t.Fatal("eval.sh has no `env -u … claude auth status` pre-check")
	}
	var unset []string
	for _, m := range regexp.MustCompile(`-u\s+([A-Z_][A-Z0-9_]*)`).FindAllStringSubmatch(script[start:end], -1) {
		unset = append(unset, m[1])
	}
	want := slices.Clone(blockedCLIEnv)
	slices.Sort(want)
	slices.Sort(unset)
	if !slices.Equal(unset, want) {
		t.Errorf("eval.sh unsets %v\nblockedCLIEnv is %v", unset, want)
	}
}
