package pyramidize

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The Pyramidize eval samples live in test-data/pyramidal-emails/, one
// Markdown file each:
//
//	# Raw Input
//	```
//	the text exactly as the user would select it
//	```
//	# User accepted output
//	```
//	the reply the user accepted
//	```
//
// The headers are matched case-insensitively ("# raw input", and "accpted",
// a typo some files carry, is accepted). Each section is the first fenced
// block under its header, or the plain text when there is none.
//
// Mail history. A selection often carries more than the author's own message:
// quoted replies, forwarded mails, the thread above or below it. Since
// 2026-10-11 (ADR-003, addendum) that history is context — the output must not
// reproduce it — so the eval has to know where it is. A sample marks it with
// two lines of their own inside the raw input:
//
//	<!-- history -->
//	...the quoted or forwarded mails...
//	<!-- /history -->
//
// A sample may mark several regions, before or after the author's text; they
// do not nest. Only the exact lines, inside the fence, are markers: anything
// else that looks like one (no spaces, another case, inline, outside the
// fence) is refused. The marker lines are removed before anything else reads
// the input, so RawInput is byte for byte what the user would select — the
// pipeline and the judges see that. AuthorText is RawInput without the marked
// regions, History the regions; RunDeterministicChecks reads both
// (ChecksVersion 2). Mark only what really is an
// earlier message, and the author's own labels for it ("hier die zweite mail
// auf die ich antworte") with it.

const (
	historyOpen  = "<!-- history -->"
	historyClose = "<!-- /history -->"
)

// evalSample is one parsed sample file.
type evalSample struct {
	Name string
	// RawInput is the text as the user selected it, mail history included and
	// the history markers removed.
	RawInput string
	// AuthorText is RawInput without the marked mail history. Equal to
	// RawInput for a sample that marks none.
	AuthorText string
	// History is the marked mail history, regions joined by a line break;
	// empty for a sample that marks none.
	History string
	// Baseline is the output the user accepted; the judges read it.
	Baseline string
}

// HasHistory reports whether the sample marks any mail history.
func (s evalSample) HasHistory() bool { return s.History != "" }

// Input is what the deterministic checks read about the sample's input.
func (s evalSample) Input() EvalInput {
	return EvalInput{Raw: s.RawInput, Author: s.AuthorText, History: s.History}
}

// historyMarkerLike finds anything that looks like a history marker, in any
// spelling. Only the exact marker lines are markers; everything else this
// finds is refused, because as plain text it would reach the model and leave
// the history counted as the author's message.
var historyMarkerLike = regexp.MustCompile(`(?i)<!--\s*/?\s*history\s*-->`)

var fenceBlockRegex = regexp.MustCompile("(?s)```\\w*\\n?(.*?)```")

// rawInputHeader matches "# Raw Input" / "# Raw input" / "# raw input" (case-insensitive).
var rawInputHeader = regexp.MustCompile(`(?im)^#\s+raw\s+input\s*$`)

// baselineHeader matches "# User accepted output(s)" including the typo "accpted" (case-insensitive).
var baselineHeader = regexp.MustCompile(`(?im)^#\s+user\s+acce?pted\s+outputs?\s*$`)

// parseTestData parses one sample file. A file without the baseline header has
// no raw input either (the header delimits the two sections): ok is false and
// the caller skips it. A malformed history marking is an error, never a guess —
// a marker that leaked into the input would be sent to the model.
func parseTestData(content string) (sample evalSample, ok bool, err error) {
	// Find the baseline header position first — it delimits the two sections.
	blLoc := baselineHeader.FindStringIndex(content)
	if blLoc == nil {
		return evalSample{}, false, nil
	}

	rawSection := content[:blLoc[0]]
	baselineSection := content[blLoc[1]:]

	// Strip the raw-input header from the first section.
	rawSection = rawInputHeader.ReplaceAllString(rawSection, "")

	// Try fenced block first; fall back to plain text after stripping the header.
	raw := rawSection
	if loc := fenceBlockRegex.FindStringSubmatchIndex(rawSection); loc != nil {
		raw = rawSection[loc[2]:loc[3]]
		if historyMarkerLike.MatchString(rawSection[:loc[0]] + "\n" + rawSection[loc[1]:]) {
			return evalSample{}, false, fmt.Errorf("a history marker outside the raw input's fence: it marks only text inside it")
		}
	}
	baseline := baselineSection
	if m := fenceBlockRegex.FindStringSubmatch(baselineSection); len(m) > 1 {
		baseline = m[1]
	}
	if historyMarkerLike.MatchString(baselineSection) {
		return evalSample{}, false, fmt.Errorf("a history marker in the accepted output: it marks only the raw input")
	}

	input, author, history, err := splitHistory(raw)
	if err != nil {
		return evalSample{}, false, err
	}
	sample = evalSample{
		RawInput:   strings.TrimSpace(input),
		AuthorText: strings.TrimSpace(author),
		History:    strings.TrimSpace(history),
		Baseline:   strings.TrimSpace(baseline),
	}
	if sample.RawInput == "" {
		return evalSample{}, false, nil
	}
	if sample.AuthorText == "" {
		return evalSample{}, false, fmt.Errorf("the whole raw input is marked as history: no author's message left to measure")
	}
	return sample, true, nil
}

// splitHistory removes the history marker lines from raw. input is raw without
// them; author is input without the marked regions; history is the regions.
func splitHistory(raw string) (input, author, history string, err error) {
	var inputLines, authorLines, historyLines []string
	inHistory, regionHasText := false, false
	for i, line := range strings.Split(raw, "\n") {
		switch strings.TrimSpace(line) {
		case historyOpen:
			if inHistory {
				return "", "", "", fmt.Errorf("line %d: %s inside a history region (regions do not nest)", i+1, historyOpen)
			}
			inHistory, regionHasText = true, false
			continue
		case historyClose:
			if !inHistory {
				return "", "", "", fmt.Errorf("line %d: %s without %s", i+1, historyClose, historyOpen)
			}
			if !regionHasText {
				return "", "", "", fmt.Errorf("line %d: an empty history region", i+1)
			}
			inHistory = false
			continue
		}
		if historyMarkerLike.MatchString(line) {
			return "", "", "", fmt.Errorf("line %d: %q looks like a history marker but is not one — a marker is %q or %q on a line of its own", i+1, strings.TrimSpace(line), historyOpen, historyClose)
		}
		inputLines = append(inputLines, line)
		if inHistory {
			regionHasText = regionHasText || strings.TrimSpace(line) != ""
			historyLines = append(historyLines, line)
		} else {
			authorLines = append(authorLines, line)
		}
	}
	if inHistory {
		return "", "", "", fmt.Errorf("%s is never closed", historyOpen)
	}
	return strings.Join(inputLines, "\n"), strings.Join(authorLines, "\n"), strings.Join(historyLines, "\n"), nil
}

// loadEvalSamples parses every .md file in dir, in name order. skipped names
// the files without a raw input, which the eval logs and leaves out.
func loadEvalSamples(dir string) (samples []evalSample, skipped []string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read the sample directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, nil, fmt.Errorf("reading %s: %w", e.Name(), err)
		}
		sample, ok, err := parseTestData(string(data))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if !ok {
			skipped = append(skipped, e.Name())
			continue
		}
		sample.Name = strings.TrimSuffix(e.Name(), ".md")
		samples = append(samples, sample)
	}
	return samples, skipped, nil
}

// dumpEvalSamples writes every sample of srcDir into dstDir as
// <name>.raw (the raw input, exactly what the eval sends) and <name>.accepted
// (the accepted output), so scripts/eval-human.sh shows and sends the same
// texts the eval does instead of parsing the files a second time in shell.
func dumpEvalSamples(srcDir, dstDir string) error {
	samples, _, err := loadEvalSamples(srcDir)
	if err != nil {
		return err
	}
	for _, s := range samples {
		for ext, text := range map[string]string{".raw": s.RawInput, ".accepted": s.Baseline} {
			if err := os.WriteFile(filepath.Join(dstDir, s.Name+ext), []byte(text), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
