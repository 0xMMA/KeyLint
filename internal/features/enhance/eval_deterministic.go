package enhance

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
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

// ChecksVersion names the instrument. Bump it whenever a change here alters
// what passes — a new check, a different threshold, a pattern that catches more.
//
// It is recorded in every run and carried into the baseline's configKey, so a
// comparison across a change like that reads "not comparable" instead of
// reporting the instrument's move as the model's. promptHash deliberately does
// NOT work this way: the prompt is what the suite measures, and a suite that
// refuses to compare across a prompt change measures nothing. The checks are
// what it measures WITH.
//
//	1 — the suite as first committed
//	2 — punctuation and sentence capitals added; resemblance scored on the raw
//	    text against both the reference and the input; forbidden words matched
//	    on inflections; casing fixes matched on word boundaries
const ChecksVersion = 2

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
	// MinInputSimilarity is the over-editing floor: how close the output must
	// stay to what the author wrote. Zero means the default.
	MinInputSimilarity float64
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
		checkResemblesReference(input, reference, output, n),
		checkLanguagePreserved(output, n),
		checkNoCommentary(output),
		checkToneAnchors(output, n),
		checkPunctuationRestored(input, reference, output),
		checkSentenceCapitals(reference, output),
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
		// On word boundaries, and case-sensitively: a casing fix is about one
		// token, and the check must not read "Cache" out of "cached" or accept
		// a fix that only landed on the second occurrence.
		if !containsWord(output, f.To) {
			return false
		}
		return f.From == f.To || !containsWord(output, f.From)
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
	// Both sides of every required fix are exempt: the wrong form because the
	// output is supposed to have lost it, and the right form because an input
	// word that a fix inflects ("schick" -> "schicke") would otherwise be
	// reported as content the model dropped.
	corrected := map[string]bool{}
	for _, f := range n.RequiredFixes {
		for _, w := range append(words(f.From), words(f.To)...) {
			corrected[w] = true
		}
	}

	have := words(output)
	haveSet := map[string]bool{}
	for _, w := range have {
		haveSet[w] = true
	}

	var missing []string
	total := 0
	for _, w := range words(input) {
		if len([]rune(w)) < 4 || corrected[w] {
			continue
		}
		total++
		if !haveSet[w] && !inflectionOf(w, have) {
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

// inflectionOf reports whether one of the output's words is the same word in a
// different form: same stem, at most two runes of ending between them. A
// correction legitimately changes an ending ("a upload" -> "an upload",
// "schick" -> "schicke"), and demanding the exact form back would score a
// correct output as content loss.
func inflectionOf(word string, candidates []string) bool {
	w := []rune(word)
	for _, c := range candidates {
		r := []rune(c)
		short, long := w, r
		if len(short) > len(long) {
			short, long = long, short
		}
		if len(long)-len(short) > 2 || len(short) < 4 {
			continue
		}
		if string(long[:len(short)]) == string(short) {
			return true
		}
	}
	return false
}

// checkResemblesReference asks whether the output is a correction OF THIS INPUT,
// by comparing it to two texts rather than one: the human correction, and the
// uncorrected input it was made from.
//
// The first version scored token F1 against the reference alone, with a 0.70
// floor. Measured against the suite it guards, the raw unfixed input cleared
// that floor on 14 of the 15 samples and scored a flat 1.000 on five of them —
// because the tokenizer lowercases and strips punctuation, so on a sample whose
// corrections are capitalisation or commas the input and the reference have
// identical token bags. The check that was supposed to make the suite resistant
// to gaming was blind to exactly the corrections most of these samples are
// about.
//
// Case and punctuation therefore stay in, and the comparison is relative: a
// correction of this input is closer to the corrected text than to the broken
// one. An output that leaves the input alone is not — whatever its absolute
// similarity — and that is a property no amount of keyword stuffing satisfies.
//
// The absolute floor stays as a second gate, because "closer to the reference
// than to the input" is cheap to satisfy by accident when both are far away.
const (
	resembleFloor     = 0.70
	defaultInputFloor = 0.65
)

func checkResemblesReference(input, reference, output string, n SampleNotes) CheckResult {
	ref, out, in := collapseSpace(reference), collapseSpace(output), collapseSpace(input)
	if ref == "" || out == "" {
		return CheckResult{Name: "resembles_reference", Score: 0, Pass: false, Detail: "empty"}
	}

	simRef, simIn := similarity(out, ref), similarity(out, in)
	detail := fmt.Sprintf("%.2f similar to the reference, %.2f to the original", simRef, simIn)

	// The already-correct sample has input == reference, so "closer to one than
	// the other" is not a question that can be asked there — and does not need
	// to be: leaving it alone is the right answer, and checkUnchanged owns it.
	if len(n.RequiredFixes) > 0 && simRef <= simIn {
		return CheckResult{
			Name: "resembles_reference", Score: 0, Pass: false,
			Detail: "no closer to the correction than to the original: " + detail,
		}
	}

	// The other direction is over-editing. A correction stays within reach of
	// what the author wrote; a rewrite that says the same thing in the model's
	// words drifts from the input without necessarily drifting from the
	// reference. The floor sits below every hand-written reference in the suite
	// (the lowest is 0.79) so that a correction is never punished for being
	// thorough, and a sample that legitimately restructures can lower it.
	floorIn := n.MinInputSimilarity
	if floorIn == 0 {
		floorIn = defaultInputFloor
	}
	if simIn < floorIn {
		return CheckResult{
			Name: "resembles_reference", Score: simRef, Pass: false,
			Detail: fmt.Sprintf("rewritten rather than corrected (%.2f to the original, want %.2f): %s", simIn, floorIn, detail),
		}
	}
	return CheckResult{
		Name: "resembles_reference", Score: simRef, Pass: simRef >= resembleFloor, Detail: detail,
	}
}

var spaceRe = regexp.MustCompile(`\s+`)

// collapseSpace normalises whitespace and nothing else. Case and punctuation are
// what several samples are about, so they must reach the distance.
func collapseSpace(s string) string { return strings.TrimSpace(spaceRe.ReplaceAllString(s, " ")) }

// similarity is 1 - normalised Levenshtein distance, on runes.
func similarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	longest := len(ra)
	if len(rb) > longest {
		longest = len(rb)
	}
	if longest == 0 {
		return 1
	}
	return 1 - float64(editDistance(ra, rb))/float64(longest)
}

// editDistance is Levenshtein with two rows rather than a full matrix: the
// longest sample is a few hundred runes, and the full matrix buys nothing.
func editDistance(a, b []rune) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, min(curr[j-1]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// checkPunctuationRestored measures the one correction the rest of this file is
// blind to: sentence boundaries.
//
// Found by measurement, not by reading. Applying only the `must-change`
// replacements to the raw input passed every check on 8 of the 15 samples, and
// the sample whose entire purpose is punctuation scored a flat 1.000 with no
// punctuation in it at all — the word-level checks strip it, and 15 missing
// characters barely move an edit distance over 120.
//
// It counts a DELTA rather than a total: what matters is whether the output
// closed the gap the reference had to close. A sample whose input is already
// punctuated asks nothing here.
//
// Full stops and commas are counted in ONE pool, which is the second version of
// this check. The first counted them separately and promptly called a correct
// output wrong: given "…der Dateien, ausserdem waere es gut wenn…", the model
// ended the sentence with a full stop where the reference used a comma. Both
// are correct German and the model restored exactly the boundary that was
// missing — the check reported a lost comma. Where a boundary goes is a
// judgement call; whether the text has one is not, and that is the only
// question this check is entitled to ask.
func checkPunctuationRestored(input, reference, output string) CheckResult {
	const marks = ".!?,;:"
	want := countAny(reference, marks) - countAny(input, marks)
	if want <= 0 {
		return CheckResult{Name: "punctuation_restored", Score: 1, Pass: true, Detail: "nothing to restore"}
	}
	got := countAny(output, marks) - countAny(input, marks)
	if got < 0 {
		got = 0
	}
	score := float64(got) / float64(want)
	if score > 1 {
		score = 1
	}
	if got == 0 {
		return CheckResult{
			Name: "punctuation_restored", Score: 0, Pass: false,
			Detail: fmt.Sprintf("none of the %d missing sentence boundaries restored", want),
		}
	}
	// Half the gap is enough: a model that writes one sentence where the
	// reference wrote two has still punctuated the text.
	return CheckResult{
		Name: "punctuation_restored", Score: score, Pass: score >= 0.5,
		Detail: fmt.Sprintf("%d of %d missing boundaries restored", got, want),
	}
}

func countAny(s, set string) int {
	n := 0
	for _, r := range s {
		if strings.ContainsRune(set, r) {
			n++
		}
	}
	return n
}

// checkSentenceCapitals counts sentences that start in lower case, and holds the
// output to the reference's count rather than to zero.
//
// This is the check that lets `must-change` name the word instead of the
// sentence. A pair like `das meeting->Das Meeting` was really two corrections
// glued together — the noun's capital, which is a fact about German, and the
// article's, which is a fact about where that phrase happened to sit in the
// reference's sentence. An output that wrote "Wir verschieben das Meeting"
// failed a pair it had actually applied. The noun is now named on its own, and
// the sentence-initial capital is measured here, where it is a property of the
// text rather than of one phrase.
//
// Counted against the reference and not against zero, so that an abbreviation
// or a deliberately lower-case line costs nothing: both sides contain it.
func checkSentenceCapitals(reference, output string) CheckResult {
	want, got := lowercaseSentenceStarts(reference), lowercaseSentenceStarts(output)
	if got <= want {
		return CheckResult{Name: "sentence_capitals", Score: 1, Pass: true, Detail: "sentences start with a capital"}
	}
	return CheckResult{
		Name: "sentence_capitals", Score: 0, Pass: false,
		Detail: fmt.Sprintf("%d sentences start in lower case, the reference has %d", got, want),
	}
}

// listMarker strips what a markdown line opens with, so that "- fixed the bug"
// is judged on "fixed".
var listMarker = regexp.MustCompile(`^[\s>]*(?:[-*+]|#{1,6}|\d+[.)])?[\s*_` + "`" + `]*`)

func lowercaseSentenceStarts(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		line = listMarker.ReplaceAllString(line, "")
		if line == "" {
			continue
		}
		for _, start := range sentenceStarts(line) {
			r := []rune(start)
			if len(r) > 0 && unicode.IsLower(r[0]) {
				n++
			}
		}
	}
	return n
}

// sentenceStarts returns the text at the start of the line and after every
// terminator that is followed by whitespace — which is what keeps "upgrade.md"
// and "z.B." from reading as sentence boundaries.
func sentenceStarts(line string) []string {
	starts := []string{strings.TrimLeft(line, " \t")}
	runes := []rune(line)
	for i := 0; i < len(runes)-1; i++ {
		if !strings.ContainsRune(".!?", runes[i]) {
			continue
		}
		j := i + 1
		for j < len(runes) && (runes[j] == '"' || runes[j] == ')' || runes[j] == ']' || strings.ContainsRune(".!?", runes[j])) {
			j++
		}
		if j >= len(runes) || !unicode.IsSpace(runes[j]) {
			continue
		}
		for j < len(runes) && unicode.IsSpace(runes[j]) {
			j++
		}
		if j < len(runes) {
			starts = append(starts, string(runes[j:]))
		}
	}
	return starts
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
		if containsWordForm(lower, strings.ToLower(w)) {
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

// germanEndings are the inflections a forbidden word may arrive in. The list is
// closed on purpose: a plain prefix test would read "Dienstag" as the forbidden
// "Dienst" — the exact false positive the word-boundary test was added to stop —
// while a translation that arrives as "Besprechungen" or "Dienste" escaped the
// boundary test entirely. "ag" is not a German ending; "e", "en" and "s" are.
//
// Compounds are NOT covered and cannot be: "Tagesordnungspunkte" is a different
// word built from "Tagesordnung", and the set of those is open. A sample that
// wants a compound refused lists it.
var germanEndings = []string{"", "e", "en", "er", "es", "em", "n", "s", "ns"}

// containsWordForm is containsWord over a word and its inflections.
func containsWordForm(haystack, needle string) bool {
	for _, end := range germanEndings {
		if containsWord(haystack, needle+end) {
			return true
		}
	}
	return false
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
//
// "Here" alone is not enough. It matched "Here is the report you asked for." —
// an ordinary sentence a user might type and expect back corrected, scored as
// if the model had narrated its own work. What makes the phrase commentary is
// what follows it, so the pattern requires that.
var commentaryPrefix = regexp.MustCompile(`(?im)^\s*(here('s| is| are)\s+(the\s+|your\s+|a\s+)?(corrected|fixed|revised|updated|proofread|correction)|note:|corrected( version)?:|fixed( version)?:|output:|sure[,!]|i (have |'ve )?(corrected|fixed|revised)|the corrected (text|version|sentence))`)

// commentaryTrailer matches a meta-remark about the correction itself, which is
// the shape the shipped prompt actually produces: it returns the sentence
// correctly and then adds "(No corrections needed—this text is grammatically
// correct.)".
//
// Two rounds of this check were defeated by punctuation, which is worth keeping
// written down because both looked fine when written:
//
//  1. The first version required the remark to be bracketed and to END with the
//     bracket. The baseline it was written for contains a run that wrote the
//     same sentence as "*(No corrections needed—…)*" — two asterisks, and the
//     check passed. The run that committed the violation in italics scored
//     HIGHER than the two that committed it in plain text.
//  2. The second version matched the phrase but kept `$` without `(?m)`. In Go
//     that anchor means end of TEXT, and unlike Perl it does not match before a
//     final newline — so "…(No corrections needed.)\n" escaped the check
//     entirely, as did any remark with text after it on a later line. The
//     recorded baseline caught the violation only because those outputs
//     happened to arrive without a trailing newline.
//
// So: the phrase, wherever it sits — its own line, after an opening bracket or
// dash, leading or trailing, in German or English, with or without emphasis.
// The inner \b anchors are deliberately absent from the "no X needed" branch:
// Go's \b is ASCII-only, so it never fires next to "Änderungen" and the German
// half of that branch silently matched nothing.
// It is anchored to a phrase that only makes sense as meta-commentary ("no
// corrections needed", "nothing to change", "already correct"), because the
// negation is what makes a phrase commentary rather than content. A user's own
// sentence containing "keine Änderungen nötig" is a false positive this design
// accepts; see the known-weaknesses section of docs/fix/quality-status.md.
var commentaryTrailer = regexp.MustCompile(`(?ims)(\n|[(\[*_—–-]|^)[^\n]*\b(no corrections?|nothing (to (do|change|correct)|was changed)|(already|schon|bereits) korrekt|already correct|(everything|alles) (looks|is|sieht|ist) (good|fine|correct|gut|korrekt|in ordnung)|keine korrekturen?|nichts (zu (tun|ändern|korrigieren)|geändert)|(no|keine)\b[^\n]{0,20}(corrections?|changes?|edits?|fehler|korrekturen|änderungen)[^\n]{0,40}(needed|necessary|required|made|applied|nötig|notwendig|erforderlich|vorgenommen))\b[^\n]*$`)

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
