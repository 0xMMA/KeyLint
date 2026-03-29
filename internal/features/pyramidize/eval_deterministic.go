package pyramidize

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CheckResult is the outcome of a single deterministic evaluation check.
type CheckResult struct {
	Name   string  `json:"name"`
	Pass   bool    `json:"pass"`
	Score  float64 `json:"score"`  // 0.0–1.0
	Detail string  `json:"detail"` // human-readable
}

// EvalScorecard aggregates all deterministic check results.
type EvalScorecard struct {
	Checks       []CheckResult `json:"checks"`
	OverallScore float64       `json:"overallScore"`
	AllPassed    bool          `json:"allPassed"`
}

// RunDeterministicChecks runs all deterministic checks and returns a scorecard.
func RunDeterministicChecks(input, output string) EvalScorecard {
	checks := []CheckResult{
		checkStructure(output),
		checkInfoCoverage(input, output),
		checkNoHallucination(input, output),
	}

	var sum float64
	allPassed := true
	for _, c := range checks {
		sum += c.Score
		if !c.Pass {
			allPassed = false
		}
	}

	return EvalScorecard{
		Checks:       checks,
		OverallScore: sum / float64(len(checks)),
		AllPassed:    allPassed,
	}
}

var headerRe = regexp.MustCompile(`(?m)^#{1,3}\s+.+`)

// checkStructure verifies the output has a subject line (text before first
// blank line) and at least one markdown header.
func checkStructure(output string) CheckResult {
	name := "structure"

	if strings.TrimSpace(output) == "" {
		return CheckResult{Name: name, Pass: false, Score: 0, Detail: "output is empty"}
	}

	var score float64
	var details []string

	// Subject line: text before the first double-newline.
	parts := strings.SplitN(output, "\n\n", 2)
	hasSubject := strings.TrimSpace(parts[0]) != ""
	if hasSubject {
		score += 0.5
	} else {
		details = append(details, "missing subject line")
	}

	// Markdown headers.
	hasHeaders := headerRe.MatchString(output)
	if hasHeaders {
		score += 0.5
	} else {
		details = append(details, "no markdown headers found")
	}

	pass := hasSubject && hasHeaders
	detail := "subject and headers present"
	if len(details) > 0 {
		detail = strings.Join(details, "; ")
	}

	return CheckResult{Name: name, Pass: pass, Score: score, Detail: detail}
}

// checkInfoCoverage extracts key terms from the input and checks what fraction
// appear in the output. Pass if >= 70% coverage.
func checkInfoCoverage(input, output string) CheckResult {
	name := "info_coverage"

	terms := extractKeyTerms(input)
	if len(terms) == 0 {
		return CheckResult{Name: name, Pass: true, Score: 1.0, Detail: "no key terms to check"}
	}

	outputLower := strings.ToLower(output)
	var found int
	var missing []string
	for _, term := range terms {
		if strings.Contains(outputLower, strings.ToLower(term)) {
			found++
		} else {
			missing = append(missing, term)
		}
	}

	coverage := float64(found) / float64(len(terms))
	pass := coverage >= 0.70

	detail := fmt.Sprintf("%.0f%% coverage (%d/%d terms)", coverage*100, found, len(terms))
	if len(missing) > 0 {
		detail += fmt.Sprintf("; missing: %s", strings.Join(missing, ", "))
	}

	return CheckResult{Name: name, Pass: pass, Score: coverage, Detail: detail}
}

// extractKeyTerms returns capitalized words from text that are not common
// stop words. Words shorter than 2 chars are skipped. Results are deduplicated.
func extractKeyTerms(text string) []string {
	skip := map[string]bool{
		"die": true, "der": true, "das": true, "den": true, "dem": true,
		"ein": true, "eine": true, "einer": true, "eines": true,
		"und": true, "oder": true, "aber": true, "mit": true, "von": true,
		"für": true, "ist": true, "hat": true, "sind": true, "wird": true,
		"ich": true, "wir": true, "sie": true, "the": true, "and": true,
		"for": true, "has": true, "was": true, "our": true, "not": true,
	}

	words := strings.Fields(text)
	seen := make(map[string]bool)
	var terms []string

	for _, w := range words {
		// Strip punctuation from edges.
		w = strings.TrimFunc(w, func(r rune) bool {
			return unicode.IsPunct(r)
		})

		if utf8.RuneCountInString(w) < 2 {
			continue
		}

		first, _ := utf8.DecodeRuneInString(w)
		if !unicode.IsUpper(first) {
			continue
		}

		if skip[strings.ToLower(w)] {
			continue
		}

		if !seen[w] {
			seen[w] = true
			terms = append(terms, w)
		}
	}

	return terms
}

// structuralHeadings are terms that pyramidize commonly introduces as section
// headings. They are excluded from hallucination counting.
var structuralHeadings = map[string]bool{
	"kernergebnis":    true,
	"hintergrund":     true,
	"kontext":         true,
	"zusammenfassung": true,
	"next":            true,
	"steps":           true,
	"background":      true,
	"summary":         true,
	"overview":        true,
	"details":         true,
	"action":          true,
	"actions":         true,
	"key":             true,
	"result":          true,
	"results":         true,
	"appendix":        true,
	"anhang":          true,
}

// checkNoHallucination finds proper nouns in the output that do not appear in
// the input. Known structural heading words are excluded. Pass if <= 1 new
// term remains (allows one incidental addition; catches real hallucinations).
func checkNoHallucination(input, output string) CheckResult {
	name := "no_hallucination"

	inputTerms := extractKeyTerms(input)
	outputTerms := extractKeyTerms(output)

	inputSet := make(map[string]bool, len(inputTerms))
	for _, t := range inputTerms {
		inputSet[strings.ToLower(t)] = true
	}

	var newTerms []string
	for _, t := range outputTerms {
		low := strings.ToLower(t)
		if inputSet[low] || structuralHeadings[low] {
			continue
		}
		newTerms = append(newTerms, t)
	}

	pass := len(newTerms) <= 1
	score := 1.0
	if len(newTerms) > 0 {
		// Degrade score proportionally, but floor at 0.
		score = 1.0 - float64(len(newTerms))*0.15
		if score < 0 {
			score = 0
		}
	}

	detail := fmt.Sprintf("%d new term(s) in output", len(newTerms))
	if len(newTerms) > 0 {
		detail += fmt.Sprintf(": %s", strings.Join(newTerms, ", "))
	}

	return CheckResult{Name: name, Pass: pass, Score: score, Detail: detail}
}
