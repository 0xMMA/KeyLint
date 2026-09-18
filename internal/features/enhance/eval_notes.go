package enhance

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// notes.md is half prose and half machine-readable, on purpose: the same file
// tells a human why a sample exists and tells the checks what to enforce. A
// separate JSON would drift from the explanation within a week.
//
// Format is `key: value` at the start of a line; everything else is prose. List
// values are comma-separated. `why:` is the prose field and is ignored here.
func parseNotes(path string) (SampleNotes, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SampleNotes{}, err
	}

	n := SampleNotes{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue // prose, or a continuation of it
		}
		key = strings.TrimSpace(strings.ToLower(key))
		value = strings.TrimSpace(value)

		if !knownKeys[key] {
			// A typo in a key used to disable the check it configures in
			// silence: `forbiden:` parsed as prose, and the sample went on
			// scoring 1.000 for a rule it no longer enforced.
			if keyShaped.MatchString(key) {
				return n, fmt.Errorf("%s: unknown key %q (indent prose so it is not read as one)", path, key)
			}
			continue
		}

		switch key {
		case "language":
			n.Language = value
		case "language-anchors":
			n.LanguageAnchors = splitList(value)
		case "forbidden":
			n.Forbidden = splitList(value)
		case "tone-anchors":
			n.ToneAnchors = splitList(value)
		case "must-change":
			// Semicolons, not commas: a correction like `hey,->Hey,` contains
			// the comma it is fixing.
			for _, pair := range splitOn(value, ";") {
				from, to, ok := strings.Cut(pair, "->")
				if !ok {
					return n, fmt.Errorf("must-change entry %q is not `wrong->right`", pair)
				}
				f := fixPair{From: strings.TrimSpace(from), To: strings.TrimSpace(to)}
				// A casing fix that spans several words glues two different
				// corrections together: the noun's capital, which is a fact
				// about the language, and the article's, which is a fact about
				// where the reference happened to put the phrase. Name the
				// word; sentence_capitals owns the sentence.
				if strings.EqualFold(f.From, f.To) && len(words(f.From)) > 1 {
					return n, fmt.Errorf("%s: casing fix %q spans several words — name the word whose capital is the correction", path, pair)
				}
				n.RequiredFixes = append(n.RequiredFixes, f)
			}
		case "unchanged":
			n.Unchanged = value == "true"
		case "markdown":
			n.Markdown = value == "true"
		case "min-length-ratio":
			n.MinLengthRatio, _ = strconv.ParseFloat(value, 64)
		case "max-length-ratio":
			n.MaxLengthRatio, _ = strconv.ParseFloat(value, 64)
		case "min-input-similarity":
			n.MinInputSimilarity, _ = strconv.ParseFloat(value, 64)
		}
	}

	// A sample with nothing to enforce would score 1.0 for doing nothing, which
	// is worse than having no sample at all.
	if !n.Unchanged && len(n.RequiredFixes) == 0 {
		return n, fmt.Errorf("%s lists no must-change entries and is not marked unchanged", path)
	}
	return n, nil
}

// keyShaped is what a key looks like, so that a prose line opening with a
// capitalised word and a colon stays prose.
var keyShaped = regexp.MustCompile(`^[a-z][a-z-]*$`)

var knownKeys = map[string]bool{
	"language": true, "language-anchors": true, "forbidden": true,
	"tone-anchors": true, "must-change": true, "unchanged": true,
	"markdown": true, "min-length-ratio": true, "max-length-ratio": true,
	"min-input-similarity": true, "why": true,
}

func splitList(v string) []string { return splitOn(v, ",") }

func splitOn(v, sep string) []string {
	var out []string
	for _, part := range strings.Split(v, sep) {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
