package enhance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
			if e.Name() == splitManifestName {
				continue
			}
			return nil, fmt.Errorf("%s: unexpected file %q — this directory holds the two split directories and the manifest", root, e.Name())
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
		n := 0
		for _, sample := range samples {
			// Stat rather than trust the dirent: a symlinked sample directory
			// reports IsDir() false and would have been skipped in silence,
			// which is a four-sample "holdout" that looks like five.
			info, err := os.Stat(filepath.Join(dir, sample.Name()))
			if err != nil {
				return nil, err
			}
			if !info.IsDir() {
				continue
			}
			n++
			refs = append(refs, SampleRef{Split: split, Name: sample.Name(), Dir: filepath.Join(dir, sample.Name())})
		}
		if n == 0 {
			return nil, fmt.Errorf("%s/%s holds no samples", root, split)
		}
	}
	for _, split := range []Split{SplitTune, SplitHoldout} {
		if !seen[split] {
			return nil, fmt.Errorf("%s has no %q directory", root, split)
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	for i := 1; i < len(refs); i++ {
		if refs[i].Name == refs[i-1].Name {
			return nil, fmt.Errorf("%s: sample %q exists in both halves", root, refs[i].Name)
		}
	}
	return refs, nil
}

const splitManifestName = "SPLIT.json"

// SplitManifest is the frozen record of how the halves were derived. It is
// committed so that the split can be checked rather than trusted, and so that a
// rename or an addition fails a test instead of quietly producing a different
// holdout.
type SplitManifest struct {
	DerivedFrom     string `json:"derivedFrom"`
	Rule            string `json:"rule"`
	OrderedBySpread []struct {
		Name   string  `json:"name"`
		Spread float64 `json:"spread"`
	} `json:"orderedBySpread"`
	Tune    []string `json:"tune"`
	Holdout []string `json:"holdout"`
}

// LoadSplitManifest reads the frozen split record next to the samples.
func LoadSplitManifest(root string) (SplitManifest, error) {
	var m SplitManifest
	data, err := os.ReadFile(filepath.Join(root, splitManifestName))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("%s: %w", splitManifestName, err)
	}
	return m, nil
}

// DeriveSplit applies the rule to a recorded ordering and returns the holdout.
// Taking the ordering as an argument is the point: the rule is a function of
// measured difficulty, not of the names on disk, so renaming a sample cannot
// move it between halves.
func DeriveSplit(orderedNames []string) (tune, holdout []string) {
	for i, name := range orderedNames {
		if (i+1)%3 == 0 {
			holdout = append(holdout, name)
		} else {
			tune = append(tune, name)
		}
	}
	sort.Strings(tune)
	sort.Strings(holdout)
	return tune, holdout
}

// SplitHash fingerprints which samples a run measured. It is recorded next to
// the numbers and reported by a comparison, but is deliberately NOT part of the
// configKey: membership is enforced by a test against the manifest, which fails
// before anything is measured, rather than by a key that only speaks up once
// two incomparable runs are already in hand.
func SplitHash(names []string) string {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:8])
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
