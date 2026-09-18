package enhance

import (
	"fmt"
	"os"
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
				n.RequiredFixes = append(n.RequiredFixes,
					fixPair{From: strings.TrimSpace(from), To: strings.TrimSpace(to)})
			}
		case "unchanged":
			n.Unchanged = value == "true"
		case "markdown":
			n.Markdown = value == "true"
		case "min-length-ratio":
			n.MinLengthRatio, _ = strconv.ParseFloat(value, 64)
		case "max-length-ratio":
			n.MaxLengthRatio, _ = strconv.ParseFloat(value, 64)
		}
	}

	// A sample with nothing to enforce would score 1.0 for doing nothing, which
	// is worse than having no sample at all.
	if !n.Unchanged && len(n.RequiredFixes) == 0 {
		return n, fmt.Errorf("%s lists no must-change entries and is not marked unchanged", path)
	}
	return n, nil
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
