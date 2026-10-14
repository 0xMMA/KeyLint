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
// write in today. Words that are common in both (in, an, am, so, was, will,
// also) are left out, and so is every all-caps word of two letters or more:
// "IT" or "US" would otherwise count as the English pronouns.
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
	"über", "unter", "durch", "ohne", "gegen", "im", "vom", "ab",
	"noch", "nur", "auch", "schon", "erst", "dann", "wie", "als", "bitte",
)

var englishFunctionWords = setOf(
	"the", "a", "and", "or", "but", "not", "that", "this", "these", "those",
	"because", "if", "than", "then", "which", "who", "what", "there",
	"is", "are", "were", "be", "been", "would", "can", "could", "must", "should",
	"have", "has", "had", "do", "does", "did",
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
	words := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) })
	for _, w := range words {
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

// lineVotes gives each line of text one vote, for the language most of its
// function words belong to; a line with none, or a tie, has no vote. Lines,
// not words: an author who writes German and pastes a long English passage
// (a README's known issues, an error message) writes in German, and counted
// by words the passage would outvote them.
func lineVotes(text string) (de, en int) {
	for _, line := range strings.Split(text, "\n") {
		d, e := languageCounts(line)
		switch {
		case d > e:
			de++
		case e > d:
			en++
		}
	}
	return de, en
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
// message: no other majority language, and no larger share of lines in the
// other language than the author's own plus maxForeignShareRise. An output
// too terse to have a majority of its own is judged by that share alone.
func checkLanguage(author, output string) CheckResult {
	const name = "language"
	want := majorityLanguage(author)
	if want == "" {
		return CheckResult{Name: name, Pass: true, Score: 1, Detail: "input too short or too mixed to tell its language"}
	}
	inShare := foreignShare(author, want)
	outShare := foreignShare(output, want)
	got := majorityLanguage(output)
	pass := (got == want || got == "") && outShare <= inShare+maxForeignShareRise
	detail := fmt.Sprintf("input %s (%.0f%% of lines other), output %s (%.0f%% of lines other)",
		want, inShare*100, orUnknown(got), outShare*100)
	return CheckResult{Name: name, Pass: pass, Score: 1 - outShare, Detail: detail}
}

func orUnknown(lang string) string {
	if lang == "" {
		return "unknown"
	}
	return lang
}
