package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSkippedRetryNeverReturnsNilNil: an attempt can time out with no error of
// its own (the response arrived as its context expired). Skipping the retry
// must still hand the SDK an error — (nil, nil) makes it read a status off a
// nil response and panic.
func TestSkippedRetryNeverReturnsNilNil(t *testing.T) {
	a := &httpAttempts{
		cfg:          Config{HTTPClient: &http.Client{Timeout: time.Second}},
		provider:     anthropicProvider,
		count:        1,
		lastTimedOut: true,
		deadline:     time.Now().Add(10 * time.Millisecond),
		hasDeadline:  true,
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.invalid/v1/messages", nil)
	called := false
	resp, err := a.middleware(req, func(*http.Request) (*http.Response, error) { called = true; return nil, nil })
	if called {
		t.Fatal("the retry was sent although it could not fit")
	}
	if resp != nil || err == nil {
		t.Fatalf("middleware = (%v, %v), want (nil, an error)", resp, err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
}

// TestRateLimitWaitedOutPastTheDeadline: a 429 whose Retry-After outlasts the
// deadline is a rate limit, not "took too long" — the remedy differs.
func TestRateLimitWaitedOutPastTheDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"x"}}`)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := newAnthropic(Config{APIKey: "sk-ant-test", BaseURL: srv.URL}).Complete(ctx,
		Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 16})

	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusTooManyRequests {
		t.Fatalf("error = %v, want a 429 StatusError", err)
	}
	if IsTimeout(err) {
		t.Errorf("error = %v, reported as a timeout", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to still unwrap to the deadline", err)
	}
}
