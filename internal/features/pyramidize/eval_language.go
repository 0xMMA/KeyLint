package pyramidize

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The language check behind TestEvalLanguage: is the output written in the
// language of the author's message?
//
// It counts function words — articles, pronouns, prepositions, conjunctions,
// auxiliaries — because those are what a translation replaces, while names,
// numbers and the English terms a German author mixes in survive one. Only
// German and English are told apart; they are the languages KeyLint's users
// write in today, and another language reads as one of them or as unknown.
// Words common to both, or used in one as a short form of something else,
// stay off the lists: in, an, am, so, was, will, also; "do" (German for
// Thursday), "im" (casual English for I'm). Skipped as well: every all-caps
// word of two letters or more ("IT", "US" would count as pronouns) and every
// word with a dot, hyphen, slash, digit or apostrophe inside it — "i.A.",
// "u.a.", "To-do", "Add-on", URLs, code — which are not running text.
//
// Known limit: an author who quotes another language inside one paragraph
// ("der Kunde schreibt: 'We have tested …'") can read as that language, since
// lines, not quotes, are what vote. The samples avoid it; the check is not
// meant for arbitrary text.
//
// Deliberately not part of RunDeterministicChecks: adding a check there moves
// ChecksVersion, which makes every recorded Pyramidize baseline "not
// comparable", and the main suite's samples are all German, so it would only
// ever confirm that German stays German.

var germanFunctionWords = setOf(
	"der", "die", "das", "den", "dem", "des",
	"ein", "eine", "einen", "einem", "einer", "eines", "kein", "keine",
	"und", "oder", "aber", "nicht", "dass", "weil", "wenn", "damit", "statt",
	"ist", "sind", "wird", "werden", "wurde", "hat", "haben", "kann", "können", "muss", "müssen",
	"ich", "wir", "du", "ihr", "sie", "es", "uns", "euch", "sich", "mein", "unser",
	"mit", "für", "von", "zu", "zum", "zur", "auf", "bei", "beim", "bis", "nach",
	"über", "unter", "durch", "ohne", "gegen", "vom", "ab",
	"noch", "nur", "auch", "schon", "erst", "dann", "wie", "als", "bitte",
)

var englishFunctionWords = setOf(
	"the", "a", "and", "or", "but", "not", "that", "this", "these", "those",
	"because", "if", "than", "then", "which", "who", "what", "there",
	"is", "are", "were", "be", "been", "would", "can", "could", "must", "should",
	"have", "has", "had", "does", "did",
	"i", "it", "we", "us", "you", "they", "he", "she", "my", "our", "your", "their", "them", "its",
	"with", "for", "of", "to", "on", "at", "by", "from", "about", "into", "after", "before",
	"already", "only", "just", "please",
)

func setOf(words ...string) map[string]bool {
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[w] = true
	}
	return set
}

// languageCounts counts text's German and English function words.
func languageCounts(text string) (de, en int) {
	for _, field := range strings.Fields(text) {
		w := strings.TrimFunc(field, func(r rune) bool { return !unicode.IsLetter(r) })
		if w == "" || strings.IndexFunc(w, func(r rune) bool { return !unicode.IsLetter(r) }) >= 0 {
			continue // not a plain word: "i.A", "To-do", a URL, "can't"
		}
		if utf8.RuneCountInString(w) >= 2 && strings.ToUpper(w) == w {
			continue // an acronym: "IT", "US", "EUR"
		}
		lower := strings.ToLower(w)
		switch {
		case germanFunctionWords[lower]:
			de++
		case englishFunctionWords[lower]:
			en++
		}
	}
	return de, en
}

// minVoteWords is the fewest function words a line needs to vote: one alone
// ("a)", a name that happens to be a word) says too little.
const minVoteWords = 2

// lineLanguage is "de" or "en" for a line that votes, "" for one that does
// not: fewer than minVoteWords function words, or a tie.
func lineLanguage(line string) string {
	d, e := languageCounts(line)
	switch {
	case d+e < minVoteWords:
		return ""
	case d > e:
		return "de"
	case e > d:
		return "en"
	}
	return ""
}

// lineVotes gives each line of text one vote (lineLanguage). Lines, not
// words: an author who writes German and pastes a long English passage (a
// README's known issues, an error message) writes in German, and counted by
// words the passage would outvote them.
func lineVotes(text string) (de, en int) {
	for _, line := range strings.Split(text, "\n") {
		switch lineLanguage(line) {
		case "de":
			de++
		case "en":
			en++
		}
	}
	return de, en
}

// firstVote is the language of text's first voting line: in an email the
// subject, the line a reader sees first.
func firstVote(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if lang := lineLanguage(line); lang != "" {
			return lang
		}
	}
	return ""
}

// minLanguageWords is the fewest function words a text needs for its language
// to be told; a three-word input says nothing reliable.
const minLanguageWords = 5

// maxForeignShareRise is how much larger the share of lines in the other
// language may be in the output than in the input. An author who mixes
// languages keeps that mix (the prompt asks for it), so the input's own share
// is allowed; anything above it by more than this is a translation, whole or
// partial.
const maxForeignShareRise = 0.15

// majorityLanguage is "de" or "en", the language most of text's lines are in,
// or "" when the text has too few function words, or as many lines in each.
func majorityLanguage(text string) string {
	if de, en := languageCounts(text); de+en < minLanguageWords {
		return ""
	}
	de, en := lineVotes(text)
	switch {
	case de > en:
		return "de"
	case en > de:
		return "en"
	}
	return ""
}

// foreignShare is the share of text's voting lines that are not in lang.
func foreignShare(text, lang string) float64 {
	de, en := lineVotes(text)
	if de+en == 0 {
		return 0
	}
	if lang == "de" {
		return float64(en) / float64(de+en)
	}
	return float64(de) / float64(de+en)
}

// checkLanguage passes when output is written in the language of the author's
// message: no other majority language, no larger share of lines in the other
// language than the author's own plus maxForeignShareRise, and — when the
// author did not mix — a first line in the author's language. An output too
// terse to have a majority of its own is judged by the rest alone.
func checkLanguage(author, output string) CheckResult {
	want := majorityLanguage(author)
	if want == "" {
		return CheckResult{Name: "language", Pass: true, Score: 1, Detail: "input too short or too mixed to tell its language"}
	}
	return judgeLanguage(want, foreignShare(author, want), "input "+want, output)
}

// checkLanguageAs passes when output is written in lang throughout: the check
// for a reply whose instructions asked for a language other than the
// author's ("in English"), where there is no mix of the author's to allow.
func checkLanguageAs(lang, output string) CheckResult {
	return judgeLanguage(lang, 0, "asked for "+lang, output)
}

func judgeLanguage(want string, inShare float64, wantLabel, output string) CheckResult {
	outShare := foreignShare(output, want)
	got := majorityLanguage(output)
	// The first line is held to the wanted language on its own when there is
	// no mix to allow: a translated subject over an otherwise right document
	// is one line of many, but the one every reader sees.
	first := firstVote(output)
	firstForeign := inShare == 0 && first != "" && first != want
	pass := (got == want || got == "") && outShare <= inShare+maxForeignShareRise && !firstForeign
	detail := fmt.Sprintf("%s (%.0f%% of lines other), output %s (%.0f%% of lines other, first line %s)",
		wantLabel, inShare*100, orUnknown(got), outShare*100, orUnknown(first))
	return CheckResult{Name: "language", Pass: pass, Score: 1 - outShare, Detail: detail}
}

func orUnknown(lang string) string {
	if lang == "" {
		return "unknown"
	}
	return lang
}
