package enhance

import (
	"fmt"
	"regexp"
	"strings"
)

// Deterministic checks for the Fix prompt. No network, no model — these are the
// properties that can be decided by looking at the text, so that the judge is
// only asked about the things that genuinely need judgement.
//
// The design constraint: a wrong answer must not be able to score well. The
// first version of this file only defended against echoing the input, which is
// one cheap wrong answer out of many — a review showed that an output made of
// nothing but the expected strings, padded to the right length, scored 1.000 on
// 13 of the 15 samples. Presence tests and a rune count cannot tell prose from
// word salad.
//
// So two checks bind the output to the actual text: contentRetained holds it to
// the input's own words, and resemblesReference holds it to a human correction
// of that input. Salad fails both; a truncation fails the first.

// CheckResult is one named check with a 0–1 score.
type CheckResult struct {
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Pass   bool    `json:"pass"`
	Detail string  `json:"detail"`
}

// Scorecard is the deterministic half of one sample's result.
type Scorecard struct {
	Checks       []CheckResult `json:"checks"`
	OverallScore float64       `json:"overallScore"`
	AllPassed    bool          `json:"allPassed"`
}

// SampleNotes is the machine-readable half of a sample's notes.md.
type SampleNotes struct {
	Language        string
	LanguageAnchors []string
	Forbidden       []string
	RequiredFixes   []fixPair
	ToneAnchors     []string
	Unchanged       bool
	Markdown        bool
	MinLengthRatio  float64
	MaxLengthRatio  float64
}

// fixPair is "this must be gone, that must be there" — written `wrong->right`.
type fixPair struct {
	From string
	To   string
}

// RunDeterministicChecks scores one output against what its notes demand.
func RunDeterministicChecks(input, reference, output string, n SampleNotes) Scorecard {
	checks := []CheckResult{
		checkRequiredFixes(output, n),
		checkContentRetained(input, output, n),
		checkResemblesReference(reference, output),
		checkLanguagePreserved(output, n),
		checkNoCommentary(output),
		checkToneAnchors(output, n),
		checkLengthRatio(input, output, n),
	}
	if n.Unchanged {
		checks = append(checks, checkUnchanged(input, output))
	}
	if n.Markdown {
		checks = append(checks, checkMarkdownPreserved(input, output))
	}

	var sum float64
	allPassed := true
	for _, c := range checks {
		sum += c.Score
		if !c.Pass {
			allPassed = false
		}
	}
	return Scorecard{Checks: checks, OverallScore: sum / float64(len(checks)), AllPassed: allPassed}
}

// checkRequiredFixes is the anti-gaming check: the corrections the sample exists
// for have to be in the output. Returning the input scores zero here.
func checkRequiredFixes(output string, n SampleNotes) CheckResult {
	if len(n.RequiredFixes) == 0 {
		// Only the already-correct sample, where "change nothing" is the answer.
		return CheckResult{Name: "required_fixes", Score: 1, Pass: true, Detail: "no fixes required"}
	}
	var missing []string
	for _, f := range n.RequiredFixes {
		if !fixApplied(output, f) {
			missing = append(missing, fmt.Sprintf("%q missing", f.To))
		}
	}
	score := 1 - float64(len(missing))/float64(len(n.RequiredFixes))
	detail := fmt.Sprintf("%d of %d applied", len(n.RequiredFixes)-len(missing), len(n.RequiredFixes))
	if len(missing) > 0 {
		detail += ": " + strings.Join(missing, ", ")
	}
	return CheckResult{Name: "required_fixes", Score: score, Pass: len(missing) == 0, Detail: detail}
}

// fixApplied decides whether one correction landed, without punishing a model
// for being right in a way the note did not anticipate.
//
// A literal substring test does that: `ausserdem->außerdem` failed against an
// output that split the sentence and correctly wrote "Außerdem" at the start of
// the new one. The model was more right than the note, and the check called it
// wrong.
//
// So a correction that is only about capitalisation is matched case-sensitively
// — that is the whole point of it — and every other correction is matched
// case-insensitively, with the wrong form required to be gone as well.
func fixApplied(output string, f fixPair) bool {
	if strings.EqualFold(f.From, f.To) {
		return strings.Contains(output, f.To)
	}
	lower := strings.ToLower(output)
	from, to := strings.ToLower(f.From), strings.ToLower(f.To)
	if !strings.Contains(lower, to) {
		return false
	}
	// `ich hab->ich habe` contains the wrong form inside the right one, so
	// "the wrong form is gone" is not a question that can be asked.
	if strings.Contains(to, from) {
		return true
	}
	return !strings.Contains(lower, from)
}

// wordRe splits text into words, keeping letters and digits across scripts so
// German umlauts and ß are not chopped into pieces.
var wordRe = regexp.MustCompile(`[\p{L}\p{N}]+`)

func words(s string) []string {
	var out []string
	for _, w := range wordRe.FindAllString(strings.ToLower(s), -1) {
		out = append(out, w)
	}
	return out
}

// checkContentRetained: the words the author wrote have to still be there.
//
// A correction changes spelling and punctuation; it does not remove content.
// This is what stops an output that drops half the paragraph — and, with the
// reference check below, what stops one that is a bag of expected keywords.
//
// Words that a required fix is supposed to remove are excluded, or the check
// would demand the error be kept. Short words are ignored: they are the ones
// corrections legitimately add and drop.
func checkContentRetained(input, output string, n SampleNotes) CheckResult {
	corrected := map[string]bool{}
	for _, f := range n.RequiredFixes {
		for _, w := range words(f.From) {
			corrected[w] = true
		}
	}

	have := map[string]bool{}
	for _, w := range words(output) {
		have[w] = true
	}

	var missing []string
	total := 0
	for _, w := range words(input) {
		if len([]rune(w)) < 4 || corrected[w] {
			continue
		}
		total++
		if !have[w] {
			missing = append(missing, w)
		}
	}
	if total == 0 {
		return CheckResult{Name: "content_retained", Score: 1, Pass: true, Detail: "nothing to hold on to"}
	}

	kept := float64(total-len(missing)) / float64(total)
	detail := fmt.Sprintf("%.0f%% of the author's words kept", kept*100)
	if len(missing) > 0 {
		shown := missing
		if len(shown) > 6 {
			shown = shown[:6]
		}
		detail += fmt.Sprintf(" (lost %v)", shown)
	}
	// 0.8 rather than 1.0: a genuine correction may replace a word, and rule 8
	// replacements are the point of two samples.
	return CheckResult{Name: "content_retained", Score: kept, Pass: kept >= 0.8, Detail: detail}
}

// checkResemblesReference scores the output against a human correction of the
// same input, as token F1.
//
// Not to demand the reference verbatim — the judge is told explicitly that the
// reference is one acceptable answer among several. This is a floor: a real
// correction of this input lands near the reference, and a bag of keywords does
// not. It is the check that makes the suite resistant to gaming rather than
// merely resistant to echoing.
func checkResemblesReference(reference, output string) CheckResult {
	ref, out := words(reference), words(output)
	if len(ref) == 0 || len(out) == 0 {
		return CheckResult{Name: "resembles_reference", Score: 0, Pass: false, Detail: "empty"}
	}

	refCount := map[string]int{}
	for _, w := range ref {
		refCount[w]++
	}
	overlap := 0
	for _, w := range out {
		if refCount[w] > 0 {
			refCount[w]--
			overlap++
		}
	}
	precision := float64(overlap) / float64(len(out))
	recall := float64(overlap) / float64(len(ref))
	f1 := 0.0
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}
	return CheckResult{
		Name: "resembles_reference", Score: f1, Pass: f1 >= 0.7,
		Detail: fmt.Sprintf("F1 %.2f against the reference", f1),
	}
}

// checkLanguagePreserved covers rule 4 from both sides: the words that mark the
// original language must survive, and the translations that would mean the text
// was turned into another language must not appear.
func checkLanguagePreserved(output string, n SampleNotes) CheckResult {
	lower := strings.ToLower(output)
	var lost, appeared []string
	for _, w := range n.LanguageAnchors {
		if !strings.Contains(lower, strings.ToLower(w)) {
			lost = append(lost, w)
		}
	}
	for _, w := range n.Forbidden {
		// Whole words. A substring test reported "Der Service läuft seit
		// Dienstag wieder" as a translation, because "Dienst" is inside
		// "Dienstag".
		if containsWord(lower, strings.ToLower(w)) {
			appeared = append(appeared, w)
		}
	}
	total := len(n.LanguageAnchors) + len(n.Forbidden)
	if total == 0 {
		return CheckResult{Name: "language_preserved", Score: 1, Pass: true, Detail: "nothing pinned"}
	}
	bad := len(lost) + len(appeared)
	detail := "language held"
	if bad > 0 {
		detail = fmt.Sprintf("lost %v, translated in %v", lost, appeared)
	}
	return CheckResult{
		Name:   "language_preserved",
		Score:  1 - float64(bad)/float64(total),
		Pass:   bad == 0,
		Detail: detail,
	}
}

// containsWord reports whether needle occurs in haystack on word boundaries.
// needle may be several words; the boundary test applies at both ends.
func containsWord(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); {
		j := strings.Index(haystack[i:], needle)
		if j < 0 {
			return false
		}
		start := i + j
		end := start + len(needle)
		beforeOK := start == 0 || !isWordByte(haystack[start-1])
		afterOK := end == len(haystack) || !isWordByte(haystack[end])
		if beforeOK && afterOK {
			return true
		}
		i = start + 1
	}
	return false
}

// isWordByte treats every non-ASCII byte as part of a word, so that a German
// umlaut does not look like a boundary.
func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b >= 0x80
}

// commentaryPrefix matches the ways a model announces what it did instead of
// just doing it — rule 6. Anchored to the start of a line, because "Note" can
// legitimately appear mid-sentence.
var commentaryPrefix = regexp.MustCompile(`(?im)^\s*(here('s| is| are)?\b|note:|corrected( version)?:|fixed( version)?:|output:|sure[,!]|i (have |'ve )?(corrected|fixed|revised)|the corrected)`)

// commentaryTrailer matches a meta-remark appended after the text, which is the
// shape the shipped prompt actually produces: it returns the sentence correctly
// and then adds "(No corrections needed—this text is grammatically correct.)".
//
// The first version required the remark to be bracketed and to END with the
// bracket. The baseline it was written for contains a run that wrote the same
// sentence as "*(No corrections needed—…)*" — two asterisks, and the check
// passed. The run that committed the violation in italics scored HIGHER than
// the two that committed it in plain text.
//
// So it matches the phrase rather than the punctuation: on its own line or after
// any opening bracket or dash, in German or English, with or without emphasis.
// Anchored to a phrase that only makes sense as meta-commentary — "no
// corrections needed", "nothing to change", "already correct" — so a sentence
// like "We discussed the changes needed for the launch" is left alone. The
// negation is what makes a phrase meta-commentary rather than content.
var commentaryTrailer = regexp.MustCompile(`(?is)(\n|[(\[*_—–-]|^)[^\n]*\b(no corrections?|nothing (to (do|change|correct)|was changed)|already correct|keine korrekturen?|nichts (zu (tun|ändern|korrigieren)|geändert)|bereits korrekt|(no|keine)\b[^\n]{0,20}\b(corrections?|changes?|edits?|fehler|korrekturen)\b[^\n]{0,40}\b(needed|required|made|applied|nötig|erforderlich|vorgenommen))\b[^\n]*$`)

func checkNoCommentary(output string) CheckResult {
	if m := commentaryPrefix.FindString(output); m != "" {
		return CheckResult{
			Name: "no_commentary", Score: 0, Pass: false,
			Detail: fmt.Sprintf("output explains itself: %q", strings.TrimSpace(m)),
		}
	}
	if m := commentaryTrailer.FindString(output); m != "" {
		return CheckResult{
			Name: "no_commentary", Score: 0, Pass: false,
			Detail: fmt.Sprintf("output comments on its own work: %q", strings.TrimSpace(m)),
		}
	}
	return CheckResult{Name: "no_commentary", Score: 1, Pass: true, Detail: "correction only"}
}

// checkToneAnchors covers rule 3. The anchors are the fragments that carry the
// register — a greeting, a smiley, a hedge — and losing them is how a model
// turns a chat message into correspondence.
func checkToneAnchors(output string, n SampleNotes) CheckResult {
	if len(n.ToneAnchors) == 0 {
		return CheckResult{Name: "tone_preserved", Score: 1, Pass: true, Detail: "nothing pinned"}
	}
	lower := strings.ToLower(output)
	var lost []string
	for _, a := range n.ToneAnchors {
		if !strings.Contains(lower, strings.ToLower(a)) {
			lost = append(lost, a)
		}
	}
	detail := "register held"
	if len(lost) > 0 {
		detail = fmt.Sprintf("lost %v", lost)
	}
	return CheckResult{
		Name:   "tone_preserved",
		Score:  1 - float64(len(lost))/float64(len(n.ToneAnchors)),
		Pass:   len(lost) == 0,
		Detail: detail,
	}
}

// checkLengthRatio catches the two ways a fix stops being a fix: an expansion
// into commentary or a rewrite, and a truncation that drops content.
func checkLengthRatio(input, output string, n SampleNotes) CheckResult {
	if len(input) == 0 {
		return CheckResult{Name: "length_ratio", Score: 1, Pass: true, Detail: "empty input"}
	}
	ratio := float64(len([]rune(output))) / float64(len([]rune(input)))
	lo, hi := n.MinLengthRatio, n.MaxLengthRatio
	if lo == 0 {
		lo = 0.85
	}
	if hi == 0 {
		hi = 1.3
	}
	ok := ratio >= lo && ratio <= hi
	score := 1.0
	if !ok {
		score = 0
	}
	return CheckResult{
		Name: "length_ratio", Score: score, Pass: ok,
		Detail: fmt.Sprintf("%.2f (want %.2f–%.2f)", ratio, lo, hi),
	}
}

// checkUnchanged is the over-editing check. Only the already-correct sample
// asks for it, and there "do nothing" is the whole job.
func checkUnchanged(input, output string) CheckResult {
	if strings.TrimSpace(input) == strings.TrimSpace(output) {
		return CheckResult{Name: "left_alone", Score: 1, Pass: true, Detail: "identical"}
	}
	return CheckResult{
		Name: "left_alone", Score: 0, Pass: false,
		Detail: "correct text was edited anyway",
	}
}

var (
	mdHeading = regexp.MustCompile(`(?m)^#{1,6} `)
	mdBullet  = regexp.MustCompile(`(?m)^\s*[-*+] `)
	mdFence   = regexp.MustCompile("(?m)^```")
	mdCode    = regexp.MustCompile("`[^`\n]+`")
	mdBold    = regexp.MustCompile(`\*\*[^*\n]+\*\*`)
)

// checkMarkdownPreserved counts the structural markers on both sides. A model
// that spells every word right and flattens the document has still destroyed it.
func checkMarkdownPreserved(input, output string) CheckResult {
	kinds := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"headings", mdHeading}, {"bullets", mdBullet}, {"fences", mdFence},
		{"inline code", mdCode}, {"bold", mdBold},
	}
	var lost []string
	for _, k := range kinds {
		in := len(k.re.FindAllString(input, -1))
		out := len(k.re.FindAllString(output, -1))
		if out < in {
			lost = append(lost, fmt.Sprintf("%s %d→%d", k.name, in, out))
		}
	}
	if len(lost) == 0 {
		return CheckResult{Name: "markdown_preserved", Score: 1, Pass: true, Detail: "structure held"}
	}
	return CheckResult{
		Name: "markdown_preserved", Score: 0, Pass: false,
		Detail: strings.Join(lost, ", "),
	}
}

// stripCodeFence removes a markdown fence around a JSON object.
//
// The judge prompt says "ONLY a JSON object" and the request asks for JSON mode,
// and the model wraps the answer in ```json anyway. Measured, not guessed: the
// first baseline run lost every judge score to exactly this.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Drop the opening fence and its optional language tag.
	if _, rest, ok := strings.Cut(s, "\n"); ok {
		s = rest
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
