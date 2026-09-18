package enhance

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The samples are split so that a prompt can be tuned against one half and
// measured once against the other. Six prompt variants were once measured on
// all fifteen and the best shipped; re-measuring the winner on its own came out
// lower on both counts that mattered, because picking the best of six draws is
// itself a measurement. A held-out half is the only thing that answers that.
//
// See docs/fix/quality-status.md for the rule that produced the split and the
// protocol for using it.

// Split names the half a sample belongs to.
type Split string

const (
	SplitTune    Split = "tune"
	SplitHoldout Split = "holdout"
	// SplitAll is not a directory. It is what a run records when it measured
	// both halves, which is what a baseline should do.
	SplitAll Split = "all"
)

// SampleRef locates one sample and names the half it belongs to.
type SampleRef struct {
	Split Split
	Name  string
	Dir   string
}

// FixSampleDirs lists every sample under root, which must contain exactly the
// two split directories and nothing else.
//
// The shape is enforced rather than assumed: a sample directory sitting loose
// at the top level would be silently invisible to a split run and silently
// present in `all`, which is the kind of difference that turns up as an
// unexplained number months later.
func FixSampleDirs(root string) ([]SampleRef, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var refs []SampleRef
	seen := map[Split]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		split := Split(e.Name())
		if split != SplitTune && split != SplitHoldout {
			return nil, fmt.Errorf("%s: %q is neither %q nor %q — samples live in one of the two split directories",
				root, e.Name(), SplitTune, SplitHoldout)
		}
		seen[split] = true
		dir := filepath.Join(root, e.Name())
		samples, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, s := range samples {
			if !s.IsDir() {
				continue
			}
			refs = append(refs, SampleRef{Split: split, Name: s.Name(), Dir: filepath.Join(dir, s.Name())})
		}
	}
	for _, split := range []Split{SplitTune, SplitHoldout} {
		if !seen[split] {
			return nil, fmt.Errorf("%s has no %q directory", root, split)
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	return refs, nil
}

// SplitFromEnv resolves which half to run. Anything but the three names is an
// error rather than a silent fallback to all: a typo that quietly measures
// fifteen samples and labels the result "holdout" is worse than a failed run.
func SplitFromEnv(value string) (Split, error) {
	switch split := Split(strings.TrimSpace(strings.ToLower(value))); split {
	case "":
		return SplitAll, nil
	case SplitAll, SplitTune, SplitHoldout:
		return split, nil
	default:
		return "", fmt.Errorf("EVAL_SPLIT=%q — expected %q, %q or %q", value, SplitAll, SplitTune, SplitHoldout)
	}
}

// Selects reports whether a sample belongs in a run of this split.
func (s Split) Selects(of Split) bool { return s == SplitAll || s == of }
