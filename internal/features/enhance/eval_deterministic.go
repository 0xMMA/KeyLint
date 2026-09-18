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
// The design constraint worth stating: none of this may be satisfiable by
// returning the input unchanged. That is the cheapest possible wrong answer and
// an eval that rewards it measures nothing. RequiredFixes is what forbids it —
// every sample but one lists corrections the output must actually contain.

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
func RunDeterministicChecks(input, output string, n SampleNotes) Scorecard {
	checks := []CheckResult{
		checkRequiredFixes(output, n),
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
		if strings.Contains(lower, strings.ToLower(w)) {
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

// commentaryPrefix matches the ways a model announces what it did instead of
// just doing it — rule 6. Anchored to the start of a line, because "Note" can
// legitimately appear mid-sentence.
var commentaryPrefix = regexp.MustCompile(`(?im)^\s*(here('s| is| are)?\b|note:|corrected( version)?:|fixed( version)?:|output:|sure[,!]|i (have |'ve )?(corrected|fixed|revised)|the corrected)`)

// commentaryTrailer matches a meta-remark appended after the text, which is the
// shape the shipped prompt actually produces: it returns the sentence correctly
// and then adds "(No corrections needed—this text is grammatically correct.)".
//
// The prefix pattern above missed every occurrence of that, because the remark
// is at the end and in parentheses. A commentary check that does not catch the
// commentary it was written for is not a check, so it is matched here too — but
// narrowly: only a bracketed aside that talks about correcting, changing or
// grammar, so ordinary parentheses in real text stay untouched. German terms
// too — most of these samples are German, and a check that only speaks English
// would miss the same failure in the language it happens in most.
var commentaryTrailer = regexp.MustCompile(`(?is)[(\[][^)\]]*\b(correct|correction|corrections|change[sd]?|edit(s|ed)?|error[s]?|grammar|grammatical|korrektur(en)?|korrigiert|änderung(en)?|fehler|grammatik)\b[^)\]]*[)\]]\s*$`)

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
