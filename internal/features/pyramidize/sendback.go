package pyramidize

import (
	"errors"
	"fmt"
	"time"

	"keylint/internal/features/clipboard"
	"keylint/internal/features/silentfix"
	"keylint/internal/logger"
)

// sendBackDesktop is what Send Back needs from the operating system.
//
// The window checks and the paste keystroke are the silent fix's (its desktop
// checks that a window is still the same one, and its paste releases held
// modifiers and tags the input so the shortcut hook ignores it). Activation is
// Send Back's own: the silent fix never moves the focus, Send Back has to.
type sendBackDesktop interface {
	// Foreground is the window that has the keyboard focus.
	Foreground() silentfix.Window
	// Title is the window's title, "" if it has none.
	Title(w silentfix.Window) string
	// Exists reports whether w is still open, still the same window, and
	// visible (a window hidden to the tray does not count).
	Exists(w silentfix.Window) bool
	// Activate asks for w to become the foreground window, restoring it
	// first if it is minimised. It may be refused; Active says.
	Activate(w silentfix.Window)
	// Active reports whether w is the foreground window and not minimised.
	Active(w silentfix.Window) bool
	// SendPaste sends the paste shortcut to the foreground window, now.
	SendPaste() error
}

// clipboardWriter is the part of the clipboard Send Back uses.
type clipboardWriter interface {
	Write(text string) error
}

// desktop is the real sendBackDesktop: silentfix's desktop plus the
// platform's activation (capture_windows.go, capture_linux.go).
type desktop struct{ silentfix.Desktop }

func newDesktop(clip *clipboard.Service) desktop {
	return desktop{silentfix.NewDesktop(clip)}
}

func (desktop) Title(w silentfix.Window) string { return windowTitle(w.Handle) }
func (d desktop) Exists(w silentfix.Window) bool {
	return d.Desktop.Exists(w) && isVisible(w.Handle)
}
func (desktop) Activate(w silentfix.Window) { activateWindow(w.Handle) }
func (d desktop) Active(w silentfix.Window) bool {
	return d.Foreground().Handle == w.Handle && !isMinimised(w.Handle)
}

const (
	// focusTimeout bounds the wait for the source window to come to the
	// foreground. Activation normally lands within a frame or two; a window
	// that is not there after half a second is not coming.
	focusTimeout = 500 * time.Millisecond
	focusPoll    = 25 * time.Millisecond
)

var (
	errNoSourceWindow = errors.New("KeyLint doesn't know which window the text came from — the text is on the clipboard, paste it with Ctrl+V.")
	errSourceClosed   = errors.New("The original window is closed — the text is on the clipboard.")
	errClipboardWrite = errors.New("Couldn't put the text on the clipboard — nothing was pasted. Copy it from the canvas instead.")
)

func errNoFocus(app string) error {
	return fmt.Errorf("Couldn't switch back to %s — the text is on the clipboard, paste it with Ctrl+V.", appLabel(app))
}

func errPasteFailed(app string) error {
	return fmt.Errorf("Couldn't paste into %s — the text is on the clipboard, paste it with Ctrl+V.", appLabel(app))
}

func appLabel(app string) string {
	if app == "" {
		return "the original window"
	}
	return app
}

// SendBack writes text to the system clipboard and pastes it into the window
// the Pyramidize shortcut was pressed in.
//
// The clipboard is written first, so whatever goes wrong afterwards the text
// is there to paste by hand, and every error says so. The paste goes to the
// source window only: if it is closed, or does not come to the foreground, or
// loses the foreground before the keystroke, nothing is pasted anywhere.
func (svc *Service) SendBack(text string) error {
	if svc.clipboard == nil || svc.desktop == nil {
		return fmt.Errorf("SendBack is not available in CLI mode")
	}
	if !svc.sendingBack.TryLock() {
		// A second click while the first is still switching windows would
		// paste twice.
		logger.Info("pyramidize: send back already running, click ignored")
		return nil
	}
	defer svc.sendingBack.Unlock()

	svc.mu.Lock()
	source, app := svc.sourceWindow, svc.sourceAppName
	svc.mu.Unlock()
	// The title can name a document or an email subject: redacted like user text.
	logger.Info("pyramidize: send back start", "window", source.Handle, "pid", source.PID, "title", logger.Redact(app))

	if err := svc.clipboard.Write(text); err != nil {
		logger.Warn("pyramidize: send back clipboard write failed", "err", err)
		return errClipboardWrite
	}

	if source.Handle == 0 {
		logger.Warn("pyramidize: send back skipped, source window unknown")
		return errNoSourceWindow
	}
	if !svc.desktop.Exists(source) {
		logger.Warn("pyramidize: send back skipped, source window closed", "window", source.Handle)
		return errSourceClosed
	}

	start := svc.now()
	svc.desktop.Activate(source)
	if !svc.waitActive(source) {
		logger.Warn("pyramidize: send back skipped, source window did not take the focus",
			"window", source.Handle, "foreground", svc.desktop.Foreground().Handle,
			"waited_ms", svc.now().Sub(start).Milliseconds())
		return errNoFocus(app)
	}
	logger.Info("pyramidize: send back focus ok", "window", source.Handle,
		"waited_ms", svc.now().Sub(start).Milliseconds())

	// Let the window settle into its own focus (an activated window sets the
	// keyboard focus to its control as it processes the activation), then
	// look again right before the keystroke: a switch in between must not
	// receive the paste.
	svc.sleep(clipboard.PasteSettle)
	if !svc.desktop.Exists(source) || !svc.desktop.Active(source) {
		logger.Warn("pyramidize: send back skipped, source window lost the focus before the paste",
			"window", source.Handle, "foreground", svc.desktop.Foreground().Handle)
		return errNoFocus(app)
	}
	if err := svc.desktop.SendPaste(); err != nil {
		logger.Warn("pyramidize: send back paste failed", "window", source.Handle, "err", err)
		return errPasteFailed(app)
	}
	logger.Info("pyramidize: send back done", "window", source.Handle)
	return nil
}

// waitActive polls until w is the foreground window, for at most focusTimeout.
func (svc *Service) waitActive(w silentfix.Window) bool {
	deadline := svc.now().Add(focusTimeout)
	for {
		if svc.desktop.Active(w) {
			return true
		}
		if !svc.now().Before(deadline) {
			return false
		}
		svc.sleep(focusPoll)
	}
}
