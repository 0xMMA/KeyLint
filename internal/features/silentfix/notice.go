package silentfix

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"keylint/internal/features/enhance"
	"keylint/internal/llm"
)

// Target is where a click on a notification takes the user.
type Target string

const (
	// TargetProviders is Settings › AI Providers: the key, the CLI sign-in or
	// the model is what needs fixing.
	TargetProviders Target = "providers"
	// TargetFix is the Fix page, showing what happened and the text involved.
	TargetFix Target = "fix"
)

// Notice is one notification, and what KeyLint shows when it is clicked.
//
// Title and Body are all the notification itself carries: Windows keeps
// toasts in the Action Center's database on disk, so they never contain the
// user's text. Detail, Input and Output stay in memory and reach only the Fix
// page.
type Notice struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Target Target `json:"target"`
	// Detail is the full error, as the Fix page would have shown it.
	Detail string `json:"detail"`
	// Input is the text the fix ran on; Output what it produced, if it got
	// that far. Either may be empty.
	Input  string `json:"input"`
	Output string `json:"output"`
}

// Titles. Short, because a toast shows one line of title.
const (
	titleNotRun     = "Fix didn't run"
	titleTooLong    = "Fix took too long"
	titleCutOff     = "Fix was cut off"
	titleNotPasted  = "Fixed, not pasted"
	titleNothingSel = "Nothing to fix"
)

// describe turns a failed enhance into a notice: a short reason in plain words
// and the page where it can be fixed. It decides on types, never on wording —
// llm and enhance are free to rephrase what they say.
func describe(err error) Notice {
	n := Notice{Title: titleNotRun, Target: TargetFix, Detail: err.Error()}

	var missingKey *enhance.MissingKeyError
	var status *llm.StatusError
	switch {
	case errors.As(err, &missingKey):
		n.Body = fmt.Sprintf("No API key for %s.", missingKey.Provider)
		n.Target = TargetProviders
	case errors.Is(err, llm.ErrNotSignedIn):
		n.Body = "Claude Code CLI isn't signed in."
		n.Target = TargetProviders
	case errors.Is(err, llm.ErrCLINotFound):
		n.Body = "Claude Code CLI isn't installed."
		n.Target = TargetProviders
	case llm.IsTimeout(err):
		n.Title = titleTooLong
		n.Body = "The model took too long — try a shorter selection or a faster model."
	case errors.Is(err, llm.ErrOutputLimitThinking):
		// The remedy is a different model, which is chosen on AI Providers.
		n.Title = titleCutOff
		n.Body = "The model used its answer on reasoning — pick one that doesn't reason first."
		n.Target = TargetProviders
	case errors.Is(err, llm.ErrOutputLimit):
		n.Title = titleCutOff
		n.Body = "The answer was cut off — try a shorter selection."
	case errors.As(err, &status):
		n.Body, n.Target = describeStatus(status)
	case isUnreachable(err):
		n.Body = "Couldn't reach the AI provider."
	default:
		n.Body = "Something went wrong — click for details."
	}
	return n
}

func describeStatus(e *llm.StatusError) (string, Target) {
	switch {
	case e.Status == http.StatusUnauthorized:
		return fmt.Sprintf("%s didn't accept the API key.", e.Provider), TargetProviders
	case e.Status == http.StatusForbidden:
		return "This key can't use the chosen model.", TargetProviders
	case e.Status == http.StatusNotFound:
		return "The model wasn't found — check it in AI Providers.", TargetProviders
	case e.Status == http.StatusPaymentRequired:
		return fmt.Sprintf("The %s account is out of credit.", e.Provider), TargetFix
	case e.Status == http.StatusTooManyRequests:
		return "Rate limited — try again shortly.", TargetFix
	case e.Status >= 500:
		return fmt.Sprintf("%s isn't available right now.", e.Provider), TargetFix
	default:
		return fmt.Sprintf("%s rejected the request.", e.Provider), TargetFix
	}
}

// isUnreachable is a provider we never got an answer from.
func isUnreachable(err error) bool {
	var urlErr *url.Error
	var netErr net.Error
	return errors.As(err, &urlErr) || errors.As(err, &netErr)
}

// Notices that are not an enhance failure.
func noticeEmptySelection() Notice {
	return Notice{Title: titleNothingSel, Body: "No text was selected.", Target: TargetFix}
}

func noticeClipboardRead(err error) Notice {
	return Notice{Title: titleNotRun, Body: "Couldn't read the clipboard.", Target: TargetFix, Detail: err.Error()}
}

func noticeClipboardWrite(err error) Notice {
	return Notice{Title: titleNotPasted, Body: "Couldn't put the fix on the clipboard — it's on the Fix page.", Target: TargetFix, Detail: err.Error()}
}

func noticeStuck() Notice {
	return Notice{Title: titleTooLong, Body: "It didn't finish — try again, or a shorter selection.", Target: TargetFix}
}

// noticeNotPasted says why a finished fix was left on the clipboard.
func noticeNotPasted(d pasteDecision, pasteErr error) Notice {
	n := Notice{Title: titleNotPasted, Target: TargetFix}
	switch d {
	case skipSourceGone:
		n.Body = "The window you fixed text in was closed — the fix is on the clipboard."
	case skipSourceInBackground:
		n.Body = "You switched windows — the fix is on the clipboard."
	case skipSourceUnknown:
		n.Body = "Couldn't tell which window to paste into — the fix is on the clipboard."
	default:
		n.Body = "Pasting failed — the fix is on the clipboard."
	}
	if pasteErr != nil {
		n.Detail = pasteErr.Error()
	}
	return n
}
