package enhance

import (
	"strings"

	"keylint/internal/logger"
)

// The prompt asks the model not to do three things. This file assumes it will
// anyway, because the eval says it does: the silent hotkey pastes whatever comes
// back over the user's selection, and there is no UI between the two to notice.
//
// Both guards are narrow on purpose. A guard that edits the model's answer on a
// hunch is worse than no guard — it would change correct output for reasons
// nobody can reconstruct from a log line — so each one fires only on a shape
// that cannot be a correction of this input.

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

// GuardVersion names this file the way ChecksVersion names the deterministic
// checks. The eval scores what Enhance returns, so the guard is part of the
// instrument: changing a threshold here moves every number in a run while the
// prompt hash stays identical. It is carried into the run summary and the
// baseline's configKey so that a comparison across a change like this reads
// "not comparable" instead of crediting the prompt.
//
//	1 — first version: word-retention reply guard, bracketed-block stripper
//	2 — reply guard measures how much of the author's text survives in order
//	    (word retention could not separate a reply from a heavily corrected
//	    text); the stripper works on any added block, not only a bracketed one;
//	    markers are only stripped where they are actually wrappers
const GuardVersion = 2

// guardAction names what the guard did, for the log and for the eval record.
type guardAction string

const (
	guardNone       guardAction = ""
	guardDroppedAdd guardAction = "dropped-addition"
	guardRefused    guardAction = "refused-reply"
)

// guardOutput is what Enhance returns rather than the raw reply. The action is
// returned rather than only logged so the eval can record it: without that, a
// sample that passes because the guard cleaned up is indistinguishable from one
// that passes because the model got it right.
func guardOutput(input, output string) (string, guardAction) {
	cleaned := stripMarkers(output)

	if stripped, ok := stripAddition(input, cleaned); ok {
		logger.Warn("enhance: the model returned the text unchanged with something added; dropped the addition",
			"feature", logFeature, "dropped_len", len(cleaned)-len(stripped))
		return stripped, guardDroppedAdd
	}

	if looksLikeAReply(input, cleaned) {
		logger.Warn("enhance: the model answered the text instead of correcting it; returning the original unchanged",
			"feature", logFeature, "input_len", len(input), "reply_len", len(cleaned))
		return input, guardRefused
	}
	return cleaned, guardNone
}

// stripMarkers removes the delimiters when the model wrapped its answer in them.
//
// Anchored to the ends, which the first version was not: it searched for the
// markers anywhere and cut there, so text that merely mentioned
// `<text-to-correct>` — which anyone working on KeyLint might write — came back
// with everything before it deleted, and the truncation was pasted over the
// user's selection.
func stripMarkers(output string) string {
	trimmed := strings.TrimSpace(output)
	changed := false
	if rest, ok := strings.CutPrefix(trimmed, inputOpen); ok {
		trimmed, changed = rest, true
	}
	if rest, ok := strings.CutSuffix(strings.TrimSpace(trimmed), inputClose); ok {
		trimmed, changed = rest, true
	}
	if !changed {
		return output
	}
	return strings.TrimSpace(trimmed)
}

// minGuardedRunes is the length below which neither guard has an opinion. A
// handful of characters cannot carry the evidence either decision needs, and a
// short input can appear inside a longer answer by coincidence.
const minGuardedRunes = 24

// stripAddition removes whatever the model put around the text when it left the
// text itself alone.
//
// The test is the whole design and it needs no list of phrases: if the output
// still contains the input verbatim, the model changed nothing in it, so
// everything else it returned is something it added — a note about its own
// work, a preamble, a code fence, a pair of quotation marks. All three runs of
// the first baseline returned the already-correct sentence with `(No
// corrections needed—this text is grammatically correct and clear.)` appended,
// and the first version of this function only recognised it because it happened
// to be bracketed.
//
// An output that does NOT contain the input verbatim is a correction, however
// much commentary it also carries, and this guard leaves it alone rather than
// guessing which half is which.
func stripAddition(input, output string) (string, bool) {
	in, out := strings.TrimSpace(input), strings.TrimSpace(output)
	if in == out || len([]rune(in)) < minGuardedRunes {
		return output, false
	}
	if !strings.Contains(out, in) {
		return output, false
	}
	return in, true
}

// replyCoverageFloor is how much of the author's text has to survive in the
// answer, measured as the share of the input's characters that appear in the
// output in order.
//
// Two measures were tried and discarded before this one, and both failures are
// the same shape — a measure that cannot tell "the model replaced my text" from
// "the model changed my text a lot":
//
//   - Word identity. On the recorded runs the two replies scored 0.08 and 0.23
//     while a correct umlaut fix scored 0.36 and an English text with heavy
//     typos scored 0.08. The populations overlapped, so every threshold either
//     passed a reply or destroyed a correction, and the suite's own
//     de-umlaute-ascii sample sat six points above the floor.
//   - Symmetric character similarity. It separated replies (0.27, 0.30) from
//     corrections (0.72 and up) cleanly, but it counts an ADDITION as damage:
//     a short sentence returned correctly with a note appended scored 0.49 and
//     was refused.
//
// Coverage is asymmetric on purpose. It asks only whether the author's
// characters are still there, in order; anything the model adds around them
// costs nothing here, because adding is what the other guard is for.
const replyCoverageFloor = 0.6

// looksLikeAReply decides whether the model wrote about the text instead of
// correcting it.
//
// One signal, deliberately. The heuristic this guard was specified with had a
// second condition — "addresses the reader, or asks a question the input did
// not" — which was dropped after checking it against the recorded failures: the
// message that provoked the replies is itself a question in the second person,
// so requiring it would have missed every observed case.
//
// The guard fails safe. When it fires the user keeps their own text and a
// warning is logged; the cost of a false positive is a correction that did not
// happen, against a false negative that overwrites the user's document with a
// chat reply.
func looksLikeAReply(input, output string) bool {
	in, out := []rune(collapseSpace(input)), []rune(collapseSpace(output))
	if len(in) < minGuardedRunes || len(out) == 0 {
		return false
	}
	return float64(commonSubsequence(in, out))/float64(len(in)) < replyCoverageFloor
}

// commonSubsequence is the length of the longest common subsequence, two rows
// at a time. The texts here are a paragraph at most.
func commonSubsequence(a, b []rune) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] >= curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}
