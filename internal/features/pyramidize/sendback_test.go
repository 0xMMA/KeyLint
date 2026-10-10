package pyramidize

import (
	"errors"
	"strings"
	"testing"
	"time"

	"keylint/internal/features/silentfix"
)

// --- fakes ---

type fakeClip struct {
	written []string
	err     error
}

func (c *fakeClip) Write(text string) error {
	if c.err != nil {
		return c.err
	}
	c.written = append(c.written, text)
	return nil
}

// fakeSendBackDesktop is a desktop whose source window takes the focus after
// a number of Active polls (-1: never), and can close or lose it on cue.
type fakeSendBackDesktop struct {
	exists      bool
	activeAfter int // Active returns true from this poll on; -1 never
	loseFocusAt int // Active returns false from this poll on; 0 never
	polls       int
	activations int
	pastes      int
	pasteErr    error
}

func (d *fakeSendBackDesktop) Foreground() silentfix.Window  { return silentfix.Window{} }
func (d *fakeSendBackDesktop) Title(silentfix.Window) string { return "" }
func (d *fakeSendBackDesktop) Exists(silentfix.Window) bool  { return d.exists }
func (d *fakeSendBackDesktop) Activate(silentfix.Window)     { d.activations++ }
func (d *fakeSendBackDesktop) Active(silentfix.Window) bool {
	d.polls++
	if d.loseFocusAt > 0 && d.polls >= d.loseFocusAt {
		return false
	}
	return d.activeAfter >= 0 && d.polls > d.activeAfter
}
func (d *fakeSendBackDesktop) SendPaste() error {
	d.pastes++
	return d.pasteErr
}

var notepad = silentfix.Window{Handle: 0x1234, PID: 42}

// newSendBackService wires a service to fakes and a fake clock that only
// moves when the service sleeps.
func newSendBackService(d *fakeSendBackDesktop, c *fakeClip) (*Service, *time.Duration) {
	slept := new(time.Duration)
	t0 := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	svc := &Service{
		clipboard:     c,
		desktop:       d,
		sleep:         func(d time.Duration) { *slept += d },
		now:           func() time.Time { return t0.Add(*slept) },
		sourceAppName: "Untitled - Notepad",
		sourceWindow:  notepad,
	}
	return svc, slept
}

func TestSendBackPastesOnceIntoTheSourceWindow(t *testing.T) {
	d := &fakeSendBackDesktop{exists: true, activeAfter: 0}
	c := &fakeClip{}
	svc, _ := newSendBackService(d, c)

	if err := svc.SendBack("hello"); err != nil {
		t.Fatalf("SendBack: %v", err)
	}
	if len(c.written) != 1 || c.written[0] != "hello" {
		t.Errorf("clipboard writes = %q, want [hello]", c.written)
	}
	if d.activations != 1 {
		t.Errorf("activations = %d, want 1", d.activations)
	}
	if d.pastes != 1 {
		t.Errorf("pastes = %d, want 1", d.pastes)
	}
}

func TestSendBackWaitsForTheWindowToComeForward(t *testing.T) {
	d := &fakeSendBackDesktop{exists: true, activeAfter: 4}
	svc, slept := newSendBackService(d, &fakeClip{})

	if err := svc.SendBack("hello"); err != nil {
		t.Fatalf("SendBack: %v", err)
	}
	if d.pastes != 1 {
		t.Errorf("pastes = %d, want 1", d.pastes)
	}
	if *slept >= focusTimeout+time.Second {
		t.Errorf("slept %v, expected a short wait", *slept)
	}
}

func TestSendBackToAClosedWindowPastesNothing(t *testing.T) {
	d := &fakeSendBackDesktop{exists: false, activeAfter: 0}
	c := &fakeClip{}
	svc, _ := newSendBackService(d, c)

	err := svc.SendBack("hello")
	if !errors.Is(err, errSourceClosed) {
		t.Fatalf("err = %v, want errSourceClosed", err)
	}
	if !strings.Contains(err.Error(), "on the clipboard") {
		t.Errorf("message %q does not say where the text is", err)
	}
	if d.activations != 0 || d.pastes != 0 {
		t.Errorf("activations = %d, pastes = %d, want 0 and 0", d.activations, d.pastes)
	}
	if len(c.written) != 1 {
		t.Errorf("the text must still be on the clipboard, writes = %d", len(c.written))
	}
}

func TestSendBackWhenTheFocusIsRefusedPastesNothing(t *testing.T) {
	d := &fakeSendBackDesktop{exists: true, activeAfter: -1}
	svc, slept := newSendBackService(d, &fakeClip{})

	err := svc.SendBack("hello")
	if err == nil {
		t.Fatal("SendBack succeeded without the focus")
	}
	if want := "Couldn't switch back to Untitled - Notepad"; !strings.Contains(err.Error(), want) {
		t.Errorf("message %q does not contain %q", err, want)
	}
	if !strings.Contains(err.Error(), "Ctrl+V") {
		t.Errorf("message %q does not say how to paste by hand", err)
	}
	if d.pastes != 0 {
		t.Errorf("pastes = %d, want 0", d.pastes)
	}
	if *slept < focusTimeout || *slept > focusTimeout+focusPoll {
		t.Errorf("waited %v, want about %v", *slept, focusTimeout)
	}
}

func TestSendBackWhenTheFocusMovesBeforeThePastePastesNothing(t *testing.T) {
	// Active on the first poll, gone by the check right before the keystroke.
	d := &fakeSendBackDesktop{exists: true, activeAfter: 0, loseFocusAt: 2}
	svc, _ := newSendBackService(d, &fakeClip{})

	if err := svc.SendBack("hello"); err == nil {
		t.Fatal("SendBack succeeded after the focus moved")
	}
	if d.pastes != 0 {
		t.Errorf("pastes = %d, want 0", d.pastes)
	}
}

func TestSendBackWithoutASourceWindowPastesNothing(t *testing.T) {
	d := &fakeSendBackDesktop{exists: true, activeAfter: 0}
	c := &fakeClip{}
	svc, _ := newSendBackService(d, c)
	svc.sourceWindow = silentfix.Window{}

	if err := svc.SendBack("hello"); !errors.Is(err, errNoSourceWindow) {
		t.Fatalf("err = %v, want errNoSourceWindow", err)
	}
	if d.activations != 0 || d.pastes != 0 {
		t.Errorf("activations = %d, pastes = %d, want 0 and 0", d.activations, d.pastes)
	}
	if len(c.written) != 1 {
		t.Errorf("the text must still be on the clipboard, writes = %d", len(c.written))
	}
}

func TestSendBackReportsAFailedPaste(t *testing.T) {
	d := &fakeSendBackDesktop{exists: true, activeAfter: 0, pasteErr: errors.New("SendInput failed")}
	svc, _ := newSendBackService(d, &fakeClip{})

	err := svc.SendBack("hello")
	if err == nil || !strings.Contains(err.Error(), "Couldn't paste into Untitled - Notepad") {
		t.Fatalf("err = %v, want a paste failure naming the app", err)
	}
	if d.pastes != 1 {
		t.Errorf("pastes = %d, want 1", d.pastes)
	}
}

func TestSendBackStopsWhenTheClipboardWriteFails(t *testing.T) {
	d := &fakeSendBackDesktop{exists: true, activeAfter: 0}
	svc, _ := newSendBackService(d, &fakeClip{err: errors.New("OpenClipboard: access denied")})

	if err := svc.SendBack("hello"); err == nil {
		t.Fatal("SendBack succeeded with a failed clipboard write")
	}
	if d.activations != 0 || d.pastes != 0 {
		t.Errorf("activations = %d, pastes = %d, want 0 and 0", d.activations, d.pastes)
	}
}

func TestSendBackIsNotAvailableInCLIMode(t *testing.T) {
	svc := NewService(nil, nil)
	if err := svc.SendBack("hello"); err == nil {
		t.Fatal("SendBack succeeded without a clipboard")
	}
}

func TestSendBackIgnoresASecondClickWhileRunning(t *testing.T) {
	d := &fakeSendBackDesktop{exists: true, activeAfter: 0}
	svc, _ := newSendBackService(d, &fakeClip{})
	svc.sendingBack.Lock()
	defer svc.sendingBack.Unlock()

	if err := svc.SendBack("hello"); err != nil {
		t.Fatalf("SendBack: %v", err)
	}
	if d.pastes != 0 {
		t.Errorf("pastes = %d, want 0", d.pastes)
	}
}
