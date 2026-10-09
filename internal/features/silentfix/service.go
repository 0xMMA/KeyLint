// Package silentfix is the hotkey fix: the user selects text anywhere, presses
// the fix shortcut, and the corrected text replaces the selection — with
// KeyLint's window staying hidden throughout.
//
// It runs entirely in Go, not in the (hidden) webview: the window the text came
// from has to be captured at the key press and held for the whole run, the
// reason a run failed is a Go error type, and the notification that reports it
// is an OS call. The frontend only takes over when the user clicks that
// notification.
//
// How it is meant to feel (#93, decided with the owner): success is silent;
// while it runs, a small dot on the tray icon is the only sign; a failure is
// one small notification with the reason in plain words, and a click on it
// opens the place where it can be fixed. Never a dialog, never a stolen focus,
// never more than one notification per key press, and the same notification is
// not repeated while the last one is still fresh.
package silentfix

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"keylint/internal/logger"
)

// Window identifies a top-level window, so the fix can be pasted back into
// the one the text came from. The zero value is "unknown".
type Window struct {
	Handle uintptr
	// PID is the window's process. A closed window's handle can be reused by
	// another window; the process is what tells the two apart.
	PID uint32
}

func (w Window) known() bool { return w.Handle != 0 }

// Desktop is what the run needs from the operating system.
type Desktop interface {
	// Foreground is the window that has the keyboard focus.
	Foreground() Window
	// Exists reports whether w is still open, and still the same window.
	Exists(w Window) bool
	// Copy sends the copy shortcut to the foreground window.
	Copy() error
	// Paste sends the paste shortcut to the foreground window.
	Paste() error
}

// Clipboard is the system clipboard.
type Clipboard interface {
	Read() (string, error)
	Write(text string) error
}

// Deps are the service's collaborators.
type Deps struct {
	Desktop   Desktop
	Clipboard Clipboard
	// Enhance runs the fix. It must return within its own deadline; the
	// safety timer is only a backstop.
	Enhance func(text string) (string, error)
	// Notify shows a notification. It must not block for long.
	Notify func(Notice)
	// Busy marks the tray icon while a fix runs. Called with the service's
	// lock held, so it must not call back into the service.
	Busy func(busy bool)
}

const (
	// safetyTimeout releases a run that never came back. The enhance call
	// has its own 120 s deadline and the copy at most a second, so this only
	// fires if something below them hangs.
	safetyTimeout = 135 * time.Second
	// quietPeriod is how long an identical notification is not shown again.
	// A user pressing the hotkey a few times to see whether it works, with the
	// key still missing, gets one note, not a stack of them.
	quietPeriod = 30 * time.Second
	// keptNotices is how many recent notices a click can still open.
	keptNotices = 8
)

// Service runs hotkey fixes, one at a time.
type Service struct {
	deps   Deps
	safety time.Duration
	quiet  time.Duration
	now    func() time.Time

	mu sync.Mutex
	// current is the token of the run in flight, 0 when none is. Only the
	// run holding it may finish it: a run released by the safety timer that
	// comes back later finds a different token (or none) and does nothing —
	// no write, no paste, no notification, and no release of a newer run.
	current uint64
	runs    uint64

	lastShown map[string]time.Time
	notices   []Notice
	noticeSeq uint64
}

// New creates the service.
func New(deps Deps) *Service {
	return &Service{
		deps:      deps,
		safety:    safetyTimeout,
		quiet:     quietPeriod,
		now:       time.Now,
		lastShown: map[string]time.Time{},
	}
}

// Trigger starts a fix for the key press that just happened, and reports
// whether it did. A press while a fix is running is dropped: it neither starts
// a second fix nor copies, so nothing can paste twice.
func (s *Service) Trigger() bool {
	// Captured first, before anything else can move the focus: this is the
	// window the copy goes to and the only one the result may be pasted into.
	source := s.deps.Desktop.Foreground()
	token, ok := s.begin()
	if !ok {
		logger.Info("silentfix: a fix is already running, key press ignored")
		return false
	}
	logger.Info("silentfix: started", "run", token, "source", source.Handle)
	go s.run(token, source)
	return true
}

func (s *Service) begin() (uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != 0 {
		return 0, false
	}
	s.runs++
	token := s.runs
	s.current = token
	s.setBusyLocked()
	time.AfterFunc(s.safety, func() {
		if s.finish(token, func() *Notice { n := noticeStuck(); return &n }) {
			logger.Warn("silentfix: run did not come back, released", "run", token)
		}
	})
	return token, true
}

func (s *Service) run(token uint64, source Window) {
	if err := s.deps.Desktop.Copy(); err != nil {
		// The clipboard may still hold the selection (the copy can report a
		// failure it recovered from), so carry on and let Read decide.
		logger.Warn("silentfix: copy failed", "run", token, "err", err)
	}
	text, err := s.deps.Clipboard.Read()
	if err != nil {
		s.fail(token, noticeClipboardRead(err))
		return
	}
	if strings.TrimSpace(text) == "" {
		s.fail(token, noticeEmptySelection())
		return
	}
	result, err := s.deps.Enhance(text)
	if err != nil {
		n := describe(err)
		n.Input = text
		s.fail(token, n)
		return
	}

	pasted := false
	finished := s.finish(token, func() *Notice {
		if err := s.deps.Clipboard.Write(result); err != nil {
			n := noticeClipboardWrite(err)
			n.Input, n.Output = text, result
			return &n
		}
		decision := decidePaste(source, s.deps.Desktop.Foreground(), s.deps.Desktop.Exists(source))
		var pasteErr error
		if decision == pasteNow {
			if pasteErr = s.deps.Desktop.Paste(); pasteErr == nil {
				pasted = true
				return nil
			}
			decision = pasteFailed
		}
		logger.Info("silentfix: result left on the clipboard", "run", token, "reason", decision.String(), "err", pasteErr)
		n := noticeNotPasted(decision, pasteErr)
		n.Input, n.Output = text, result
		return &n
	})
	if !finished {
		logger.Warn("silentfix: run came back after it was released, result dropped", "run", token)
		return
	}
	if pasted {
		logger.Info("silentfix: done", "run", token)
	}
}

// fail ends a run with a notification, if the run still holds the guard.
func (s *Service) fail(token uint64, n Notice) {
	logger.Warn("silentfix: failed", "run", token, "title", n.Title, "body", n.Body, "err", n.Detail)
	if !s.finish(token, func() *Notice { return &n }) {
		logger.Warn("silentfix: failure came back after the run was released, not reported", "run", token)
	}
}

// finish runs fn and releases the guard — only if token is still the run in
// flight, and atomically with that check, so the safety timer cannot release a
// run halfway through its paste. fn may return a notice to show. It reports
// whether the run was still current.
func (s *Service) finish(token uint64, fn func() *Notice) bool {
	s.mu.Lock()
	if s.current != token {
		s.mu.Unlock()
		return false
	}
	n := fn()
	s.current = 0
	s.setBusyLocked()
	var show *Notice
	if n != nil {
		show = s.recordLocked(*n)
	}
	s.mu.Unlock()

	if show != nil {
		s.deps.Notify(*show)
	}
	return true
}

func (s *Service) setBusyLocked() {
	if s.deps.Busy != nil {
		s.deps.Busy(s.current != 0)
	}
}

// recordLocked gives n an ID and keeps it for a click, and returns it unless
// an identical one was shown within the quiet period.
func (s *Service) recordLocked(n Notice) *Notice {
	s.noticeSeq++
	n.ID = fmt.Sprintf("silentfix-%d", s.noticeSeq)
	s.notices = append(s.notices, n)
	if len(s.notices) > keptNotices {
		s.notices = s.notices[len(s.notices)-keptNotices:]
	}

	key := n.Title + "\n" + n.Body
	now := s.now()
	if last, ok := s.lastShown[key]; ok && now.Sub(last) < s.quiet {
		logger.Info("silentfix: same notification shown moments ago, not repeated", "title", n.Title)
		return nil
	}
	s.lastShown[key] = now
	return &n
}

// Notice returns a recent notice by ID, for a click on its notification.
func (s *Service) Notice(id string) (Notice, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.notices {
		if n.ID == id {
			return n, true
		}
	}
	return Notice{}, false
}

// pasteDecision is whether a finished fix may be pasted, and if not, why.
type pasteDecision int

const (
	pasteNow pasteDecision = iota
	skipSourceUnknown
	skipSourceGone
	skipSourceInBackground
	pasteFailed
)

func (d pasteDecision) String() string {
	switch d {
	case pasteNow:
		return "paste"
	case skipSourceUnknown:
		return "source window unknown"
	case skipSourceGone:
		return "source window closed"
	case skipSourceInBackground:
		return "user switched windows"
	default:
		return "paste failed"
	}
}

// decidePaste pastes only into the window the text came from, and only while
// it still has the focus.
//
// If the user has moved to another window in the meantime, the fix is not
// pasted there — and KeyLint does not pull them back to the source window
// either: they may be typing, and a focus switch mid-word would send their
// keystrokes somewhere they did not mean. The fix waits on the clipboard
// instead, and the notification says so.
func decidePaste(source, foreground Window, sourceExists bool) pasteDecision {
	switch {
	case !source.known():
		return skipSourceUnknown
	case !sourceExists:
		return skipSourceGone
	case foreground != source:
		return skipSourceInBackground
	default:
		return pasteNow
	}
}
