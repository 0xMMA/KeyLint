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
var boldHeaderRe = regexp.MustCompile(`(?m)^\*\*[^*]+\*\*`)

// checkStructure verifies the output has a subject line (text before first
// blank line) and at least one header (markdown # headers or **bold** headers).
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

	// Headers: accept both markdown # headers and **bold** line headers.
	hasHeaders := headerRe.MatchString(output) || boldHeaderRe.MatchString(output)
	if hasHeaders {
		score += 0.5
	} else {
		details = append(details, "no markdown or bold headers found")
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
		// German articles & pronouns
		"die": true, "der": true, "das": true, "den": true, "dem": true,
		"ein": true, "eine": true, "einer": true, "eines": true,
		"und": true, "oder": true, "aber": true, "mit": true, "von": true,
		"für": true, "ist": true, "hat": true, "sind": true, "wird": true,
		"ich": true, "wir": true, "sie": true,
		// German common words
		"hallo": true, "bitte": true, "danke": true, "viele": true,
		"grüße": true, "beste": true, "guten": true, "guter": true,
		"gute": true, "gutes": true, "herr": true, "frau": true,
		"sehr": true, "auch": true, "noch": true, "schon": true,
		"dann": true, "hier": true, "dort": true, "dieser": true,
		"diese": true, "dieses": true, "welche": true, "welcher": true,
		"welches": true, "wenn": true, "weil": true, "dass": true,
		"nach": true, "über": true, "unter": true, "zwischen": true,
		"durch": true, "ohne": true, "gegen": true, "vor": true,
		"hinter": true, "neben": true, "bei": true, "auf": true,
		"aus": true, "bis": true, "seit": true,
		// English articles & pronouns
		"the": true, "and": true, "for": true, "has": true, "was": true,
		"our": true, "not": true, "this": true, "that": true, "with": true,
		"from": true, "have": true, "will": true, "been": true, "also": true,
		"just": true, "more": true, "some": true, "than": true, "when": true,
		"what": true, "which": true, "their": true, "there": true,
		"these": true, "those": true, "about": true, "into": true,
		"only": true, "other": true, "could": true, "would": true,
		"should": true, "where": true, "after": true, "before": true,
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

// commonRestructuringVocab contains words that are legitimate restructuring
// vocabulary — common in business emails but often absent from casual input.
// These are NOT hallucinations; they're the language of structured communication.
var commonRestructuringVocab = map[string]bool{
	// German structural headings
	"kernergebnis": true, "hintergrund": true, "kontext": true,
	"zusammenfassung": true, "anhang": true, "ergebnis": true,
	"empfehlung": true, "schritte": true,
	// German common business vocabulary
	"rückmeldung": true, "abstimmung": true, "konfiguration": true,
	"dokumentation": true, "beauftragung": true, "beauftragungen": true,
	"entscheidung": true, "entscheidungsbedarf": true, "handlungsbedarf": true,
	"klärung": true, "freigabe": true, "umsetzung": true,
	"zeitplanung": true, "fertigstellung": true, "einschätzung": true,
	"priorisierung": true, "verfügbarkeit": true, "anforderung": true,
	"anforderungen": true, "voraussetzung": true, "voraussetzungen": true,
	"stellungnahme": true, "erläuterung": true, "auflistung": true,
	"auswertung": true, "übersicht": true, "aufstellung": true,
	"maßnahmen": true, "ursache": true, "gründe": true, "grund": true,
	"schwerpunkte": true, "perspektive": true, "tendenz": true,
	"dateien": true, "datei": true, "zugang": true, "zugriff": true,
	"feedback": true, "info": true, "basis": true, "start": true,
	"fokus": true, "blick": true, "sicht": true, "ersatz": true,
	"slot": true, "minuten": true, "abend": true, "woche": true,
	// German greeting / closing / connector words
	"hallo": true, "bitte": true, "hinweis": true, "grüße": true,
	"viele": true, "beste": true, "damit": true, "falls": true,
	"kannst": true, "darüber": true, "seite": true, "sobald": true,
	"aufgrund": true, "angesichts": true, "zusätzlich": true,
	"zur": true, "zum": true, "beim": true, "zwei": true, "drei": true,
	"kein": true, "keine": true, "ggf": true, "bzw": true,
	// German status / action words
	"stand": true, "status": true, "update": true, "offene": true,
	"nächste": true, "nächster": true, "nächstes": true, "punkte": true,
	"themen": true, "offener": true, "punkt": true, "laufende": true,
	"bekannte": true, "behobene": true, "fehlende": true, "mögliche": true,
	"erhoffte": true, "konkrete": true, "aktuelle": true, "sichtbares": true,
	// English structural headings
	"next": true, "steps": true, "background": true, "summary": true,
	"overview": true, "details": true, "action": true, "actions": true,
	"key": true, "result": true, "results": true, "appendix": true,
	"items": true, "decision": true, "context": true,
}

// checkNoHallucination finds proper nouns in the output that do not appear in
// the input. Known structural heading words are excluded. Compound words
// (split on hyphens) are accepted if any part appears in the input. Pass if
// new terms are <= 15% of total output terms. Score = max(0, 1.0 - ratio*3).
func checkNoHallucination(input, output string) CheckResult {
	name := "no_hallucination"

	inputTerms := extractKeyTerms(input)
	outputTerms := extractKeyTerms(output)

	inputSet := make(map[string]bool, len(inputTerms))
	for _, t := range inputTerms {
		inputSet[strings.ToLower(t)] = true
	}

	// Lowercased full input text for substring matching — catches words that
	// were lowercase in input but capitalized in output (common in German).
	inputLower := strings.ToLower(input)

	var newTerms []string
	for _, t := range outputTerms {
		low := strings.ToLower(t)

		// Direct match in input key terms or common restructuring vocabulary.
		if inputSet[low] || commonRestructuringVocab[low] {
			continue
		}

		// Check if the lowercased term appears anywhere in the lowercased input.
		if strings.Contains(inputLower, low) {
			continue
		}

		// Compound word check: split on "-" or "/" and accept if any part
		// exists in the lowercased input text.
		if strings.ContainsAny(t, "-/") {
			parts := strings.FieldsFunc(low, func(r rune) bool { return r == '-' || r == '/' })
			foundPart := false
			for _, part := range parts {
				if utf8.RuneCountInString(part) < 2 {
					continue
				}
				if strings.Contains(inputLower, part) {
					foundPart = true
					break
				}
			}
			if foundPart {
				continue
			}
		}

		// German compound noun heuristic: for words > 6 chars without
		// separators, check if any prefix or suffix (>= 4 chars) appears
		// in the input. Catches "Diagnosefunktion" when "Diagnose" is in input.
		runeLen := utf8.RuneCountInString(low)
		if runeLen > 6 {
			runes := []rune(low)
			compoundMatch := false
			for i := 4; i <= runeLen-3; i++ {
				prefix := string(runes[:i])
				suffix := string(runes[i:])
				if strings.Contains(inputLower, prefix) && utf8.RuneCountInString(prefix) >= 4 {
					compoundMatch = true
					break
				}
				if strings.Contains(inputLower, suffix) && utf8.RuneCountInString(suffix) >= 4 {
					compoundMatch = true
					break
				}
			}
			if compoundMatch {
				continue
			}
		}

		newTerms = append(newTerms, t)
	}

	// Percentage-based pass threshold: pass if new terms <= 15% of output terms.
	totalOutputTerms := len(outputTerms)
	var newTermRatio float64
	if totalOutputTerms > 0 {
		newTermRatio = float64(len(newTerms)) / float64(totalOutputTerms)
	}

	pass := newTermRatio <= 0.15
	score := 1.0 - newTermRatio*3
	if score < 0 {
		score = 0
	}
	if score > 1.0 {
		score = 1.0
	}

	detail := fmt.Sprintf("%d new term(s) in output (%.0f%% of %d output terms)", len(newTerms), newTermRatio*100, totalOutputTerms)
	if len(newTerms) > 0 {
		detail += fmt.Sprintf(": %s", strings.Join(newTerms, ", "))
	}

	return CheckResult{Name: name, Pass: pass, Score: score, Detail: detail}
}
