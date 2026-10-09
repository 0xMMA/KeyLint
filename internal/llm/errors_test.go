package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetryWorthwhile pins the retry decision of #93: a quick failure is always
// retried, a timed-out attempt only when another full attempt still fits.
func TestRetryWorthwhile(t *testing.T) {
	const attempt = 90 * time.Second
	tests := []struct {
		name         string
		prevTimedOut bool
		hasDeadline  bool
		remaining    time.Duration
		want         bool
	}{
		{"quick failure, little time left", false, true, time.Second, true},
		{"timed out, no caller deadline", true, false, 0, true},
		{"timed out, a full attempt still fits", true, true, 95 * time.Second, true},
		{"timed out, exactly one attempt left", true, true, attempt, true},
		// The case from the issue: 90 s attempt inside a 120 s deadline.
		{"timed out, 30 s left of 120 s", true, true, 30 * time.Second, false},
		{"timed out, deadline already gone", true, true, -time.Second, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryWorthwhile(tc.prevTimedOut, tc.hasDeadline, tc.remaining, attempt); got != tc.want {
				t.Errorf("retryWorthwhile = %v, want %v", got, tc.want)
			}
		})
	}
	if !retryWorthwhile(true, true, time.Second, 0) {
		t.Error("with no per-attempt timeout there is nothing to fit, so the retry must go ahead")
	}
}

// hangingServer answers nothing for longer than any test waits, and counts the
// requests that reached it.
func hangingServer(t *testing.T, hits *atomic.Int32) string {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	return srv.URL
}

// TestTimedOutAttemptIsNotRetriedIntoTheDeadline is the issue's scenario,
// scaled down: the first attempt times out, and what is left of the caller's
// deadline is shorter than an attempt. The retry must not be sent, and the user
// must hear "took too long" straight away.
func TestTimedOutAttemptIsNotRetriedIntoTheDeadline(t *testing.T) {
	for _, provider := range []string{ProviderClaude, ProviderOpenAI} {
		t.Run(provider, func(t *testing.T) {
			var hits atomic.Int32
			base := hangingServer(t, &hits)
			client, err := New(provider, Config{
				APIKey:     "sk-test",
				BaseURL:    base,
				HTTPClient: &http.Client{Timeout: time.Second},
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			// After the first attempt's 1 s and the SDK's retry backoff (up to
			// 0.5 s), well under one attempt is left of the 1.8 s deadline.
			ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Millisecond)
			defer cancel()

			start := time.Now()
			_, err = client.Complete(ctx, Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 16})
			elapsed := time.Since(start)

			if err == nil {
				t.Fatal("expected an error")
			}
			if got := hits.Load(); got != 1 {
				t.Errorf("requests = %d, want 1 — the retry could not fit and must not be sent", got)
			}
			if !IsTimeout(err) {
				t.Errorf("error = %v, want a TimeoutError", err)
			}
			if !strings.Contains(err.Error(), "try a shorter selection or a faster model") {
				t.Errorf("error = %q, want it to say what to do", err.Error())
			}
			if elapsed >= 1700*time.Millisecond {
				t.Errorf("returned after %v, want it at the first attempt's timeout, not the deadline", elapsed)
			}
		})
	}
}

// TestTimedOutAttemptIsRetriedWhenThereIsRoom keeps the retry where it can
// still help: plenty of deadline left after the first timeout. (The Anthropic
// SDK retries a timed-out attempt; openai-go never does, so only it can show
// the retry being left alone.)
func TestTimedOutAttemptIsRetriedWhenThereIsRoom(t *testing.T) {
	var hits atomic.Int32
	base := hangingServer(t, &hits)
	client := newAnthropic(Config{
		APIKey:     "sk-ant-test",
		BaseURL:    base,
		HTTPClient: &http.Client{Timeout: 200 * time.Millisecond},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Complete(ctx, Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 16}); err == nil {
		t.Fatal("expected an error")
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("requests = %d, want 2 — a full attempt still fitted, so the retry should go", got)
	}
}

// TestTypedErrorsKeepTheirWording: the typed errors exist for callers that act
// on the reason, and must not change a word of what users already read.
func TestTypedErrorsKeepTheirWording(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		var got capture
		srv := newServer(t, &got, http.StatusUnauthorized, `{"type":"error","error":{"type":"authentication_error","message":"x"}}`)
		_, err := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL}).Complete(context.Background(),
			Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 16})
		var status *StatusError
		if !errors.As(err, &status) || status.Status != http.StatusUnauthorized {
			t.Fatalf("error = %v, want a StatusError with 401", err)
		}
		if want := "Claude error 401: the API key was not accepted"; err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})
	t.Run("output limit", func(t *testing.T) {
		var got capture
		srv := newServer(t, &got, http.StatusOK, `{"content":[{"type":"text","text":"half"}],"stop_reason":"max_tokens"}`)
		_, err := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL}).Complete(context.Background(),
			Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 16})
		if !errors.Is(err, ErrOutputLimit) {
			t.Fatalf("error = %v, want ErrOutputLimit", err)
		}
		if want := "Claude: " + outputLimitMessage; err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})
	t.Run("cancel is not a timeout", func(t *testing.T) {
		err := transportError(anthropicProvider, context.Canceled)
		if IsTimeout(err) {
			t.Errorf("a cancelled request reported as a timeout: %v", err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want it to still wrap context.Canceled", err)
		}
	})
	t.Run("deadline is a timeout and still unwraps", func(t *testing.T) {
		err := transportError(anthropicProvider, context.DeadlineExceeded)
		if !IsTimeout(err) || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error = %v, want a TimeoutError wrapping context.DeadlineExceeded", err)
		}
	})
}

// TestClaudeCodeTypedErrors: a signed-out CLI and a timed-out one are what the
// hotkey turns into "go to AI Providers" and "try a shorter selection".
func TestClaudeCodeTypedErrors(t *testing.T) {
	t.Run("signed out", func(t *testing.T) {
		s := newStub(t)
		s.replies(`{"result":"Not logged in · Please run /login","is_error":true,"terminal_reason":"api_error"}`)
		_, err := s.client().Complete(context.Background(), Request{Model: "haiku", User: "x"})
		if !errors.Is(err, ErrNotSignedIn) {
			t.Fatalf("error = %v, want ErrNotSignedIn", err)
		}
		if want := "Claude Code CLI: " + notSignedInMessage; err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})
	t.Run("timed out", func(t *testing.T) {
		s := newStub(t)
		s.hangs()
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		_, err := s.client().Complete(ctx, Request{Model: "haiku", User: "x"})
		if !IsTimeout(err) {
			t.Fatalf("error = %v, want a TimeoutError", err)
		}
	})
	t.Run("not installed", func(t *testing.T) {
		original := systemCandidates
		systemCandidates = nil
		t.Cleanup(func() { systemCandidates = original })
		t.Setenv("PATH", t.TempDir())
		t.Setenv("HOME", t.TempDir())
		t.Setenv("USERPROFILE", t.TempDir())
		t.Setenv("APPDATA", t.TempDir())
		t.Setenv("LOCALAPPDATA", t.TempDir())
		_, err := newClaudeCode(Config{}).Complete(context.Background(), Request{Model: "haiku", User: "x"})
		if !errors.Is(err, ErrCLINotFound) {
			t.Fatalf("error = %v, want ErrCLINotFound", err)
		}
	})
}
