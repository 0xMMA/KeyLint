package enhance

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func mkdirAll(path string) error { return os.MkdirAll(path, 0o755) }

const samplesRoot = "../../../test-data/fix-samples"

// TestTheCommittedSplitMatchesTheRule is the guard the split needs, and the
// reason the rule takes a recorded ordering rather than reading names off disk.
//
// A review showed the first version could be steered: the rule was a function
// of filenames, so renaming one sample rotated four of the five holdout members,
// and whoever wrote a new sample chose the outcome by choosing where it sorted.
// Nothing would have objected — the configKey records how MANY samples a run
// measured, never which.
//
// So membership is frozen in SPLIT.json and checked here. A rename, an addition
// or a hand-edited directory fails this test before anything is measured, which
// is where the objection belongs.
func TestTheCommittedSplitMatchesTheRule(t *testing.T) {
	manifest, err := LoadSplitManifest(samplesRoot)
	if err != nil {
		t.Fatalf("reading the split manifest: %v", err)
	}

	ordered := make([]string, len(manifest.OrderedBySpread))
	for i, e := range manifest.OrderedBySpread {
		ordered[i] = e.Name
	}
	tune, holdout := DeriveSplit(ordered)

	if !reflect.DeepEqual(tune, manifest.Tune) {
		t.Errorf("the manifest's tune half is not what the rule produces:\n rule: %v\n file: %v", tune, manifest.Tune)
	}
	if !reflect.DeepEqual(holdout, manifest.Holdout) {
		t.Errorf("the manifest's holdout is not what the rule produces:\n rule: %v\n file: %v", holdout, manifest.Holdout)
	}

	// And what is on disk has to be what the manifest says.
	refs, err := FixSampleDirs(samplesRoot)
	if err != nil {
		t.Fatalf("reading samples: %v", err)
	}
	onDisk := map[Split][]string{}
	for _, ref := range refs {
		onDisk[ref.Split] = append(onDisk[ref.Split], ref.Name)
	}
	if !reflect.DeepEqual(onDisk[SplitTune], manifest.Tune) {
		t.Errorf("tune on disk = %v, manifest = %v", onDisk[SplitTune], manifest.Tune)
	}
	if !reflect.DeepEqual(onDisk[SplitHoldout], manifest.Holdout) {
		t.Errorf("holdout on disk = %v, manifest = %v", onDisk[SplitHoldout], manifest.Holdout)
	}

	// The ordering the rule was applied to has to be the samples themselves,
	// or the rule was applied to something else.
	if len(ordered) != len(refs) {
		t.Errorf("the manifest orders %d samples, %d are on disk", len(ordered), len(refs))
	}
}

// TestTheRuleIsBlindToNames: ordering by measured difficulty is what makes a
// rename harmless. The first version sorted alphabetically and could be steered
// by one.
func TestTheRuleIsBlindToNames(t *testing.T) {
	ordered := []string{"a", "b", "c", "d", "e", "f"}
	_, holdout := DeriveSplit(ordered)
	if !reflect.DeepEqual(holdout, []string{"c", "f"}) {
		t.Fatalf("holdout = %v, want every third", holdout)
	}

	// The same samples, renamed so the alphabetical order reverses: the rule
	// still picks the third and sixth by difficulty.
	renamed := []string{"zz-a", "yy-b", "xx-c", "ww-d", "vv-e", "uu-f"}
	_, holdout = DeriveSplit(renamed)
	if !reflect.DeepEqual(holdout, []string{"uu-f", "xx-c"}) {
		t.Fatalf("holdout = %v — the rule read the names instead of the order", holdout)
	}
}

// TestASampleInBothHalvesIsRefused: it would be counted twice, and
// eval-aggregate.sh's group_by(.name) would merge the two into one per-sample
// entry with double the run count.
func TestASampleInBothHalvesIsRefused(t *testing.T) {
	root := t.TempDir()
	for _, split := range []string{"tune", "holdout"} {
		if err := mkdirAll(filepath.Join(root, split, "same-name")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := FixSampleDirs(root); err == nil {
		t.Error("a sample present in both halves was accepted")
	}
}

// TestAnEmptyHalfIsRefused: an empty tune directory used to pass, and an `all`
// run would then measure five samples and record that it measured everything.
func TestAnEmptyHalfIsRefused(t *testing.T) {
	root := t.TempDir()
	if err := mkdirAll(filepath.Join(root, "tune", "a")); err != nil {
		t.Fatal(err)
	}
	if err := mkdirAll(filepath.Join(root, "holdout")); err != nil {
		t.Fatal(err)
	}
	if _, err := FixSampleDirs(root); err == nil {
		t.Error("an empty half was accepted")
	}
}
