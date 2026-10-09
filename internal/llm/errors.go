package llm

import (
	"context"
	"errors"
	"net"
	"time"
)

// Typed failures.
//
// The text a user reads is unchanged by any of these: each one prints exactly
// what the plain fmt.Errorf it replaced printed. They exist for a caller that
// has to act on the reason rather than show it — the silent hotkey fix, which
// has one small notification to say what went wrong and where to fix it, and
// must not tell "go to AI Providers" from "try a shorter selection" by matching
// wording that is free to change.
var (
	// ErrNotSignedIn is the Claude Code CLI reporting a signed-out account.
	ErrNotSignedIn = errors.New(notSignedInMessage)
	// ErrCLINotFound is no Claude Code CLI on this machine.
	ErrCLINotFound = errors.New("not found. Install Claude Code, or pick a provider with an API key in Settings")
	// ErrOutputLimit is a reply that was cut off at the output limit.
	ErrOutputLimit = errors.New(outputLimitMessage)
	// ErrOutputLimitThinking is the same cut-off on a model that reasoned
	// first; see outputLimitThinkingMessage for why the two are kept apart.
	ErrOutputLimitThinking = errors.New(outputLimitThinkingMessage)
)

// StatusError is a provider that answered with an HTTP error status. Error()
// is the apiError wording; Status lets a caller tell a rejected key (401/403)
// from a provider that is merely down.
type StatusError struct {
	Provider string // display name, as in the message
	Status   int
	msg      string
}

func (e *StatusError) Error() string { return e.msg }

// timeoutAdvice is what a user can do about a model that took too long. Worded
// for every surface it reaches — the GUI, the hotkey and the CLI — so it names
// the two levers and no settings tab.
const timeoutAdvice = "try a shorter selection or a faster model"

// TimeoutError is a request that ran out of time: the per-attempt HTTP timeout,
// or the caller's own deadline. It still unwraps to the underlying error, so
// errors.Is(err, context.DeadlineExceeded) keeps working for callers that
// already test for it.
//
// It replaces "request failed: context deadline exceeded", which told the user
// nothing they could act on (#93).
type TimeoutError struct {
	Provider string
	// Detail is what the provider said on the way out, if anything — the
	// Claude Code CLI's last stderr line is often the only explanation for why
	// it went quiet.
	Detail string
	err    error
}

func (e *TimeoutError) Error() string {
	msg := e.Provider + " took too long to answer — " + timeoutAdvice
	if e.Detail != "" {
		msg += " (" + e.Detail + ")"
	}
	return msg
}

func (e *TimeoutError) Unwrap() error { return e.err }

// IsTimeout reports whether err is, or wraps, a request that ran out of time.
func IsTimeout(err error) bool {
	var t *TimeoutError
	return errors.As(err, &t)
}

// isTimeoutCause recognises a raw timeout before it has been given a
// TimeoutError: an expired context, or a net/http client timeout (which is a
// net.Error whether or not it also matches context.DeadlineExceeded).
func isTimeoutCause(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// retryWorthwhile decides whether the SDK's retry of a failed attempt is worth
// sending (#93).
//
// A retry after a quick failure — a 429, a 5xx, a dropped connection — is what
// the retry is for, and is always sent. A retry after an attempt that timed out
// is different: that attempt already needed more than attemptTimeout, so a
// second one with less time than that left cannot do better. It would cost the
// user the rest of the deadline in silence, and on a provider that bills by the
// generation possibly a second charge, only to fail the same way.
//
// hasDeadline false means the caller set no deadline, so there is always room.
func retryWorthwhile(prevTimedOut bool, hasDeadline bool, remaining, attemptTimeout time.Duration) bool {
	if !prevTimedOut || !hasDeadline || attemptTimeout <= 0 {
		return true
	}
	return remaining >= attemptTimeout
}
