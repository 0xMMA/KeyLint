package silentfix

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"keylint/internal/features/clipboard"
	"keylint/internal/features/enhance"
	"keylint/internal/llm"
)

// --- fakes ---

type fakeDesktop struct {
	mu         sync.Mutex
	foreground Window
	closed     map[Window]bool
	copies     int
	copyErr    error
	pastes     int
	pasteErr   error
}

func (d *fakeDesktop) Foreground() Window {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.foreground
}
func (d *fakeDesktop) Exists(w Window) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return w.known() && !d.closed[w]
}
func (d *fakeDesktop) Copy() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.copies++
	return d.copyErr
}
func (d *fakeDesktop) SendPaste() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pastes++
	return d.pasteErr
}
func (d *fakeDesktop) focus(w Window) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.foreground = w
}
func (d *fakeDesktop) close(w Window) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed == nil {
		d.closed = map[Window]bool{}
	}
	d.closed[w] = true
}
func (d *fakeDesktop) counts() (copies, pastes int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.copies, d.pastes
}

type fakeClipboard struct {
	mu      sync.Mutex
	text    string
	readErr error
	writes  []string
}

func (c *fakeClipboard) Read() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text, c.readErr
}
func (c *fakeClipboard) Write(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes = append(c.writes, text)
	c.text = text
	return nil
}
func (c *fakeClipboard) written() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.writes...)
}

// harness wires a Service to fakes and records what the user would see.
type harness struct {
	t       *testing.T
	svc     *Service
	desk    *fakeDesktop
	clip    *fakeClipboard
	enhance func(string) (string, error)

	mu       sync.Mutex
	notices  []Notice
	busy     []bool
	enhanced int
	done     chan struct{} // one value per finished run (notified or not)
}

var editor = Window{Handle: 0x1001, PID: 42}
var chat = Window{Handle: 0x2002, PID: 77}

func newHarness(t *testing.T) *harness {
	h := &harness{
		t:    t,
		desk: &fakeDesktop{foreground: editor},
		clip: &fakeClipboard{text: "their going home"},
		done: make(chan struct{}, 16),
	}
	h.enhance = func(string) (string, error) { return "They're going home.", nil }
	h.svc = New(Deps{
		Desktop:   h.desk,
		Clipboard: h.clip,
		Enhance: func(text string) (string, error) {
			h.mu.Lock()
			h.enhanced++
			fn := h.enhance
			h.mu.Unlock()
			return fn(text)
		},
		Notify: func(n Notice) {
			h.mu.Lock()
			h.notices = append(h.notices, n)
			h.mu.Unlock()
		},
		Busy: func(b bool) {
			h.mu.Lock()
			h.busy = append(h.busy, b)
			if !b {
				h.done <- struct{}{}
			}
			h.mu.Unlock()
		},
	})
	h.svc.sleep = func(time.Duration) {} // no settle delay in tests
	return h
}

func (h *harness) setEnhance(fn func(string) (string, error)) {
	h.mu.Lock()
	h.enhance = fn
	h.mu.Unlock()
}

// waitIdle waits for n runs to release the guard.
func (h *harness) waitIdle(n int) {
	h.t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-h.done:
		case <-time.After(5 * time.Second):
			h.t.Fatal("a run never finished")
		}
	}
}

// waitNotices waits until n notices were shown. The guard is released
// before the notification goes out, so waitIdle alone can be early.
func (h *harness) waitNotices(n int) []Notice {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got := h.shown(); len(got) >= n {
			return got
		}
		time.Sleep(time.Millisecond)
	}
	h.t.Fatalf("waited for %d notice(s), got %+v", n, h.shown())
	return nil
}

// settleQuiet gives a notification that should not come a moment to show up.
func (h *harness) settleQuiet() { time.Sleep(20 * time.Millisecond) }

func (h *harness) shown() []Notice {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Notice(nil), h.notices...)
}

func (h *harness) enhanceCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.enhanced
}

// --- the flow ---

func TestSuccessPastesIntoTheSourceWindowSilently(t *testing.T) {
	h := newHarness(t)
	if !h.svc.Trigger() {
		t.Fatal("Trigger refused the first press")
	}
	h.waitIdle(1)

	if _, pastes := h.desk.counts(); pastes != 1 {
		t.Errorf("pastes = %d, want 1", pastes)
	}
	if got := h.clip.written(); len(got) != 1 || got[0] != "They're going home." {
		t.Errorf("clipboard writes = %q, want the fix once", got)
	}
	h.settleQuiet()
	if n := h.shown(); len(n) != 0 {
		t.Errorf("a successful fix showed %d notification(s): %+v", len(n), n)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.busy) != 2 || !h.busy[0] || h.busy[1] {
		t.Errorf("busy = %v, want [true false] — the tray dot on while it runs, off after", h.busy)
	}
}

func TestSecondPressWhileRunningIsDropped(t *testing.T) {
	h := newHarness(t)
	release := make(chan struct{})
	h.setEnhance(func(string) (string, error) { <-release; return "Fixed.", nil })

	if !h.svc.Trigger() {
		t.Fatal("first press refused")
	}
	if h.svc.Trigger() {
		t.Error("a second press while a fix runs started another one")
	}
	close(release)
	h.waitIdle(1)

	copies, pastes := h.desk.counts()
	if copies != 1 || pastes != 1 {
		t.Errorf("copies = %d, pastes = %d, want 1 and 1 — the dropped press must not copy or paste", copies, pastes)
	}
	if got := h.enhanceCalls(); got != 1 {
		t.Errorf("enhance calls = %d, want 1", got)
	}
	if !h.svc.Trigger() {
		t.Error("a press after the fix finished was refused — the guard was not released")
	}
	h.waitIdle(1)
}

// TestLateRunCannotClearANewerGuard is the per-run token: a run released by
// the safety timer that comes back later must not paste, write, notify, or
// release the run that started after it.
func TestLateRunCannotClearANewerGuard(t *testing.T) {
	h := newHarness(t)
	h.svc.safety = 50 * time.Millisecond
	releaseA := make(chan struct{})
	h.setEnhance(func(string) (string, error) { <-releaseA; return "late A", nil })

	if !h.svc.Trigger() {
		t.Fatal("run A refused")
	}
	h.waitIdle(1) // the safety timer released A

	if n := h.waitNotices(1); len(n) != 1 || n[0].Title != titleTooLong {
		t.Fatalf("notices after the safety release = %+v, want one %q", n, titleTooLong)
	}

	releaseB := make(chan struct{})
	h.setEnhance(func(string) (string, error) { <-releaseB; return "B", nil })
	h.svc.safety = time.Minute
	if !h.svc.Trigger() {
		t.Fatal("run B refused after A was released")
	}

	close(releaseA) // A comes back now
	time.Sleep(50 * time.Millisecond)

	if got := h.clip.written(); len(got) != 0 {
		t.Errorf("the late run wrote %q to the clipboard", got)
	}
	if _, pastes := h.desk.counts(); pastes != 0 {
		t.Errorf("the late run pasted")
	}
	if h.svc.Trigger() {
		t.Error("a press while B runs was accepted — the late run released B's guard")
	}

	close(releaseB)
	h.waitIdle(1)
	if got := h.clip.written(); len(got) != 1 || got[0] != "B" {
		t.Errorf("clipboard writes = %q, want only B's result", got)
	}
	if _, pastes := h.desk.counts(); pastes != 1 {
		t.Errorf("pastes = %d, want 1 (B only)", pastes)
	}
	h.settleQuiet()
	if n := h.shown(); len(n) != 1 {
		t.Errorf("notices = %+v, want only the safety one — the late run must stay quiet", n)
	}
}

func TestSwitchedWindowLeavesTheFixOnTheClipboard(t *testing.T) {
	h := newHarness(t)
	release := make(chan struct{})
	h.setEnhance(func(string) (string, error) { <-release; return "Fixed.", nil })

	h.svc.Trigger()
	h.desk.focus(chat) // the user moves on while the model thinks
	close(release)
	h.waitIdle(1)

	if _, pastes := h.desk.counts(); pastes != 0 {
		t.Error("pasted into the window the user switched to")
	}
	if got := h.desk.Foreground(); got != chat {
		t.Error("the focus was moved back to the source window")
	}
	if got := h.clip.written(); len(got) != 1 || got[0] != "Fixed." {
		t.Errorf("clipboard = %q, want the fix waiting there", got)
	}
	n := h.waitNotices(1)
	if len(n) != 1 || n[0].Title != titleNotPasted || !strings.Contains(n[0].Body, "switched windows") {
		t.Fatalf("notices = %+v, want one saying the fix was not pasted because of the switch", n)
	}
	if n[0].Output != "Fixed." || n[0].Input != "their going home" || n[0].Target != TargetFix {
		t.Errorf("notice = %+v, want the Fix page with input and output", n[0])
	}
}

func TestClosedSourceWindowIsNotPastedAnywhere(t *testing.T) {
	h := newHarness(t)
	release := make(chan struct{})
	h.setEnhance(func(string) (string, error) { <-release; return "Fixed.", nil })

	h.svc.Trigger()
	h.desk.close(editor)
	h.desk.focus(chat)
	close(release)
	h.waitIdle(1)

	if _, pastes := h.desk.counts(); pastes != 0 {
		t.Error("pasted although the source window is gone")
	}
	n := h.waitNotices(1)
	if len(n) != 1 || !strings.Contains(n[0].Body, "closed") {
		t.Fatalf("notices = %+v, want one saying the window was closed", n)
	}
}

func TestEmptySelectionDoesNotCallTheModel(t *testing.T) {
	h := newHarness(t)
	h.clip.text = "  \n"
	h.svc.Trigger()
	h.waitIdle(1)

	if got := h.enhanceCalls(); got != 0 {
		t.Errorf("enhance calls = %d, want 0", got)
	}
	if n := h.waitNotices(1); len(n) != 1 || n[0].Title != titleNothingSel {
		t.Errorf("notices = %+v, want one %q", n, titleNothingSel)
	}
}

// TestNothingCopiedDoesNotFixTheOldClipboard: with nothing selected the copy
// leaves the clipboard as it was. That old text — this run's own last fix, or
// something private — must not be sent to the provider or pasted.
func TestNothingCopiedDoesNotFixTheOldClipboard(t *testing.T) {
	h := newHarness(t)
	h.clip.text = "They're going home." // the previous fix, still on the clipboard
	h.desk.copyErr = fmt.Errorf("copy: %w", clipboard.ErrNothingCopied)
	h.svc.Trigger()
	h.waitIdle(1)

	if got := h.enhanceCalls(); got != 0 {
		t.Errorf("enhance calls = %d, want 0 — the old clipboard was sent to the provider", got)
	}
	if _, pastes := h.desk.counts(); pastes != 0 {
		t.Error("pasted although nothing was selected")
	}
	if n := h.waitNotices(1); n[0].Title != titleNothingSel || n[0].Input != "" {
		t.Errorf("notice = %+v, want %q without the old clipboard text", n[0], titleNothingSel)
	}
}

// TestFocusIsCheckedAfterTheSettleDelay: the check and the keystroke must be
// back to back. A switch during the clipboard's settle delay must stop the
// paste, not land it in the new window.
func TestFocusIsCheckedAfterTheSettleDelay(t *testing.T) {
	h := newHarness(t)
	h.svc.sleep = func(time.Duration) { h.desk.focus(chat) } // the switch lands mid-settle
	h.svc.Trigger()
	h.waitIdle(1)

	if _, pastes := h.desk.counts(); pastes != 0 {
		t.Error("pasted into the window the user switched to during the settle delay")
	}
	if n := h.waitNotices(1); !strings.Contains(n[0].Body, "switched windows") {
		t.Errorf("notice = %+v, want the switched-windows one", n[0])
	}
}

// TestAnotherFieldOfTheSameWindowCountsAsMovingAway: where the platform can
// tell the focused control, moving to another field is moving away.
func TestAnotherFieldOfTheSameWindowCountsAsMovingAway(t *testing.T) {
	body := Window{Handle: 0x1001, PID: 42, Focus: 0x10}
	subject := Window{Handle: 0x1001, PID: 42, Focus: 0x11}
	if got := decidePaste(body, subject, true); got != skipSourceInBackground {
		t.Errorf("decidePaste = %v, want skipSourceInBackground", got)
	}
}

// TestPanicInARunIsContained: a panic below the run (a provider SDK, say)
// must not take the app — and its keyboard hook — down, and must release the
// guard.
func TestPanicInARunIsContained(t *testing.T) {
	h := newHarness(t)
	h.setEnhance(func(string) (string, error) { panic("boom") })
	h.svc.Trigger()
	h.waitIdle(1)

	if n := h.waitNotices(1); n[0].Title != titleNotRun {
		t.Errorf("notice = %+v, want a %q", n[0], titleNotRun)
	}
	h.setEnhance(func(string) (string, error) { return "Fixed.", nil })
	if !h.svc.Trigger() {
		t.Fatal("the guard was not released after the panic")
	}
	h.waitIdle(1)
}

// TestRepeatedFailuresDoNotStack: a user pressing the hotkey again and again
// with the key still missing gets one notification, until it is stale.
func TestRepeatedFailuresDoNotStack(t *testing.T) {
	h := newHarness(t)
	clock := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	h.svc.now = func() time.Time { return clock }
	h.setEnhance(func(string) (string, error) {
		return "", &enhance.MissingKeyError{Provider: "Anthropic"}
	})

	for i := 0; i < 3; i++ {
		h.svc.Trigger()
		h.waitIdle(1)
	}
	h.waitNotices(1)
	h.settleQuiet()
	if n := h.shown(); len(n) != 1 {
		t.Fatalf("three identical failures showed %d notifications, want 1", len(n))
	}

	// A different reason is news, and is shown.
	h.setEnhance(func(string) (string, error) { return "", fmt.Errorf("Claude: %w", llm.ErrOutputLimit) })
	h.svc.Trigger()
	h.waitIdle(1)
	if n := h.waitNotices(2); len(n) != 2 {
		t.Fatalf("a different failure showed %d notifications in total, want 2", len(n))
	}

	// After the quiet period the first one may be said again.
	clock = clock.Add(quietPeriod + time.Second)
	h.setEnhance(func(string) (string, error) {
		return "", &enhance.MissingKeyError{Provider: "Anthropic"}
	})
	h.svc.Trigger()
	h.waitIdle(1)
	if n := h.waitNotices(3); len(n) != 3 {
		t.Fatalf("after the quiet period: %d notifications, want 3", len(n))
	}
}

func TestNoticeCanBeLookedUpForAClick(t *testing.T) {
	h := newHarness(t)
	h.setEnhance(func(string) (string, error) { return "", &enhance.MissingKeyError{Provider: "OpenAI"} })
	h.svc.Trigger()
	h.waitIdle(1)

	n := h.waitNotices(1)
	if len(n) != 1 || n[0].ID == "" {
		t.Fatalf("notices = %+v, want one with an ID", n)
	}
	got, ok := h.svc.Notice(n[0].ID)
	if !ok || got.Target != TargetProviders || got.Input != "their going home" {
		t.Errorf("Notice(%q) = %+v, %v; want the providers notice with its input", n[0].ID, got, ok)
	}
	if _, ok := h.svc.Notice("silentfix-unknown"); ok {
		t.Error("an unknown ID was found")
	}
	// Another process numbers its notices from 1 too; a toast an earlier
	// session left in the Action Center must not open this session's text.
	if !strings.Contains(n[0].ID, fmt.Sprint(h.svc.boot)) {
		t.Errorf("ID %q does not carry the process nonce", n[0].ID)
	}
}

// TestSuppressedRepeatUpdatesTheVisibleToast: a click on the one toast on
// screen opens the latest run's text, whose fix is on the clipboard.
func TestSuppressedRepeatUpdatesTheVisibleToast(t *testing.T) {
	h := newHarness(t)
	release := make(chan struct{}, 2)
	h.setEnhance(func(text string) (string, error) { <-release; return "fix of " + text, nil })
	for _, sel := range []string{"first", "second"} {
		h.clip.mu.Lock()
		h.clip.text = sel
		h.clip.mu.Unlock()
		h.desk.focus(editor)
		h.svc.Trigger()
		h.desk.focus(chat)
		release <- struct{}{}
		h.waitIdle(1)
	}
	n := h.waitNotices(1)
	h.settleQuiet()
	if got := h.shown(); len(got) != 1 {
		t.Fatalf("notices = %+v, want one on screen", got)
	}
	got, ok := h.svc.Notice(n[0].ID)
	if !ok || got.Input != "second" || got.Output != "fix of second" {
		t.Errorf("Notice(%q) = %+v, want the second run's text", n[0].ID, got)
	}
}

// --- the pieces ---

func TestDecidePaste(t *testing.T) {
	tests := []struct {
		name       string
		source     Window
		foreground Window
		exists     bool
		want       pasteDecision
	}{
		{"source still focused", editor, editor, true, pasteNow},
		{"user switched windows", editor, chat, true, skipSourceInBackground},
		{"source closed", editor, chat, false, skipSourceGone},
		{"source closed, handle reused by the focused window", editor, editor, false, skipSourceGone},
		{"source never captured", Window{}, editor, false, skipSourceUnknown},
		{"same handle, other process", editor, Window{Handle: editor.Handle, PID: 99}, true, skipSourceInBackground},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := decidePaste(tc.source, tc.foreground, tc.exists); got != tc.want {
				t.Errorf("decidePaste = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		title  string
		body   string
		target Target
	}{
		{"no key", &enhance.MissingKeyError{Provider: "Anthropic"}, titleNotRun, "No API key for Anthropic.", TargetProviders},
		{"CLI signed out", fmt.Errorf("Claude Code CLI: %w", llm.ErrNotSignedIn), titleNotRun, "Claude Code CLI isn't signed in.", TargetProviders},
		{"CLI missing", fmt.Errorf("Claude Code CLI: %w", llm.ErrCLINotFound), titleNotRun, "Claude Code CLI isn't installed.", TargetProviders},
		{"timeout", &llm.TimeoutError{Provider: "Claude"}, titleTooLong, "The model took too long — try a shorter selection or a faster model.", TargetFix},
		{"cut off", fmt.Errorf("Claude: %w", llm.ErrOutputLimit), titleCutOff, "The answer was cut off — try a shorter selection.", TargetFix},
		{"cut off while reasoning", fmt.Errorf("Claude: %w", llm.ErrOutputLimitThinking), titleCutOff, "The model used its answer on reasoning — pick one that doesn't reason first.", TargetProviders},
		{"key rejected", statusErr(t, http.StatusUnauthorized), titleNotRun, "Anthropic didn't accept the API key.", TargetProviders},
		{"model gone", statusErr(t, http.StatusNotFound), titleNotRun, "The model wasn't found — check it in AI Providers.", TargetProviders},
		{"rate limited", statusErr(t, http.StatusTooManyRequests), titleNotRun, "Rate limited — try again shortly.", TargetFix},
		{"provider down", statusErr(t, http.StatusBadGateway), titleNotRun, "Anthropic isn't available right now.", TargetFix},
		{"provider timed out", statusErr(t, http.StatusRequestTimeout), titleNotRun, "Anthropic timed out — try again.", TargetFix},
		{"too large", statusErr(t, http.StatusRequestEntityTooLarge), titleNotRun, "The text is too long for the provider — try a shorter selection.", TargetFix},
		{"unreachable", fmt.Errorf("Ollama request failed: %w", &url.Error{Op: "Post", URL: "http://localhost:11434", Err: errors.New("connection refused")}), titleNotRun, "Couldn't reach the AI provider.", TargetFix},
		{"anything else", errors.New("Claude Code CLI failed: exited with status 3"), titleNotRun, "Something went wrong — click for details.", TargetFix},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := describe(tc.err)
			if n.Title != tc.title || n.Body != tc.body || n.Target != tc.target {
				t.Errorf("describe = {%q, %q, %q}, want {%q, %q, %q}", n.Title, n.Body, n.Target, tc.title, tc.body, tc.target)
			}
			if n.Detail != tc.err.Error() {
				t.Errorf("Detail = %q, want the full error for the Fix page", n.Detail)
			}
		})
	}
}

// statusErr gets a real StatusError from the llm package, the way a provider
// call would produce one.
func statusErr(t *testing.T, status int) error {
	t.Helper()
	srv := newStatusServer(t, status)
	_, err := mustClient(t, srv).Complete(context.Background(), llm.Request{Model: "claude-sonnet-4-6", User: "x", MaxTokens: 16})
	var se *llm.StatusError
	if !errors.As(err, &se) {
		t.Fatalf("expected a StatusError for %d, got %v", status, err)
	}
	return err
}
