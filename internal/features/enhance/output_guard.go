package enhance

import (
	"regexp"
	"strings"

	"keylint/internal/logger"
)

// The prompt asks the model not to do three things. This file assumes it will
// anyway, because the eval says it does: the silent hotkey pastes whatever comes
// back over the user's selection, and there is no UI between the two to notice.
//
// Both guards are deliberately narrow. A guard that edits the model's answer on
// a hunch is worse than no guard: it would change correct output for reasons
// nobody can reconstruct from a log line. So each fires only on a shape that
// cannot be a correction of the input.

const (
	inputOpen  = "<text-to-correct>"
	inputClose = "</text-to-correct>"
)

// buildUserMessage delimits the text so the model has a name for what it is
// looking at. Without it, a message that reads as addressed to a reader is
// indistinguishable from one addressed to the assistant — which is how two of
// three baseline runs came to answer a chat message instead of correcting it.
func buildUserMessage(text string) string {
	return inputOpen + "\n" + text + "\n" + inputClose
}

// metaTrailer is a parenthesised or bracketed block at the very start or end of
// the output, optionally wrapped in markdown emphasis.
//
// It is not matched on what it says. Whether the block is commentary is decided
// by what is left after removing it: if that is the input, the block is the only
// thing the model added, and the prompt says it should have added nothing.
var metaTrailer = regexp.MustCompile(`(?s)^\s*[*_]{0,2}[(\[][^)\]]*[)\]][*_]{0,2}\s*|\s*[*_]{0,2}[(\[][^)\]]*[)\]][*_]{0,2}\s*$`)

// GuardOutput is what Enhance returns rather than the raw reply.
//
// Order matters: the markers come off first (the model sometimes echoes them),
// then an added meta block, and only then is what remains tested for being a
// reply rather than a correction.
func guardOutput(input, output string) string {
	cleaned := stripMarkers(output)

	if stripped, ok := stripAddedMeta(input, cleaned); ok {
		logger.Warn("enhance: dropped a note the model added to correct text",
			"feature", logFeature, "dropped_len", len(cleaned)-len(stripped))
		cleaned = stripped
	}

	if looksLikeAReply(input, cleaned) {
		logger.Warn("enhance: the model answered the text instead of correcting it; returning the original unchanged",
			"feature", logFeature, "input_len", len(input), "reply_len", len(cleaned))
		return input
	}
	return cleaned
}

// stripMarkers removes the delimiters if the model echoed them back.
func stripMarkers(output string) string {
	trimmed := strings.TrimSpace(output)
	if i := strings.Index(trimmed, inputOpen); i >= 0 {
		trimmed = trimmed[i+len(inputOpen):]
	}
	if i := strings.LastIndex(trimmed, inputClose); i >= 0 {
		trimmed = trimmed[:i]
	}
	if trimmed == strings.TrimSpace(output) {
		return output // nothing echoed; leave whitespace as the model sent it
	}
	return strings.TrimSpace(trimmed)
}

// stripAddedMeta removes a leading or trailing bracketed block when doing so
// leaves exactly the input.
//
// This is the whole test, and it is why the guard needs no list of phrases to
// recognise. All three baseline runs returned the already-correct sentence with
// `(No corrections needed—this text is grammatically correct and clear.)`
// appended; remove that and what is left is the input, character for character,
// which means the model changed nothing and said so anyway. A block whose
// removal does NOT leave the input is left alone: it might be the author's own
// parenthesis, and this guard does not get to guess.
func stripAddedMeta(input, output string) (string, bool) {
	if strings.TrimSpace(output) == strings.TrimSpace(input) {
		return output, false
	}
	for range 2 { // one at each end at most
		candidate := strings.TrimSpace(metaTrailer.ReplaceAllString(output, ""))
		if candidate == output || candidate == "" {
			break
		}
		output = candidate
		if strings.TrimSpace(output) == strings.TrimSpace(input) {
			return output, true
		}
	}
	return output, false
}

// replyRetentionFloor is how much of the author's own vocabulary a correction
// has to carry. Measured rather than chosen: across the two recorded baselines,
// the replies kept 8% and 25% of the input's words, a run that mistranslated a
// whole clause still kept 55%, and a correction that merely fixed spelling kept
// 95% or more. Anything below this is not a correction of this text.
const replyRetentionFloor = 0.4

// looksLikeAReply decides whether the model wrote about the text instead of
// correcting it.
//
// Word retention is the only signal, and that is a deliberate narrowing of the
// heuristic this guard was specified with. The obvious second condition —
// "addresses the reader, or asks a question the input did not" — does not work:
// the message that actually provoked the replies is itself a question in the
// second person, so requiring it would have missed every observed case.
//
// The guard fails safe. When it fires, the user keeps their own text and a
// warning is logged; the cost of a false positive is a correction that did not
// happen, against a false negative that overwrites the user's document with a
// chat reply. Short inputs carry no signal at all and are left alone.
func looksLikeAReply(input, output string) bool {
	kept, total := retainedWords(input, output)
	if total < minWordsToJudge {
		return false
	}
	return float64(kept)/float64(total) < replyRetentionFloor
}

// minWordsToJudge is where the ratio starts meaning something. Below it a single
// corrected word swings the share past any threshold.
const minWordsToJudge = 8

// retainedWords counts how many of the input's substantial words survive in the
// output, allowing an ending to change — the same measure the eval's
// content_retained check uses, so the number in a log line and the number in a
// scorecard mean the same thing.
func retainedWords(input, output string) (kept, total int) {
	have := words(output)
	haveSet := map[string]bool{}
	for _, w := range have {
		haveSet[w] = true
	}
	for _, w := range words(input) {
		if len([]rune(w)) < 4 {
			continue
		}
		total++
		if haveSet[w] || inflectionOf(w, have) {
			kept++
		}
	}
	return kept, total
}
