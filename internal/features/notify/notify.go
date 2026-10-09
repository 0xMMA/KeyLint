// Package notify shows the one kind of OS notification KeyLint has: a small,
// silent, non-modal note from the tray that something the user started in the
// background did not work out. It never takes focus and never opens a dialog;
// clicking it hands its ID back through OnActivate.
//
// Windows shows it as a toast, via go-toast and the AppUserModelID registry
// method for unpackaged apps, which works for the NSIS install, a dev-channel
// build and an exe run from anywhere alike. Other platforms only log it: Linux
// is a development platform for KeyLint, not a target.
package notify

import (
	"sync"

	"keylint/internal/logger"
)

// Toasts shows notifications. The zero value is not usable; use New.
type Toasts struct {
	mu         sync.Mutex
	onActivate func(id string)
	platform   platform
}

// platform is the per-OS half.
type platform interface {
	// show displays a notification. It must not block the caller for long and
	// must never take focus.
	show(id, title, body string)
}

// New prepares notifications for an app with this name — the name Windows
// shows above them — and icon (PNG). Setup failures are logged and leave
// notifications switched off; they never stop the app.
func New(appName string, icon []byte) *Toasts {
	t := &Toasts{}
	t.platform = newPlatform(appName, icon, t.activated)
	return t
}

// OnActivate sets what happens when the user clicks a notification. id is the
// one the notification was shown with. It runs on a COM thread, not the UI
// thread.
func (t *Toasts) OnActivate(fn func(id string)) {
	t.mu.Lock()
	t.onActivate = fn
	t.mu.Unlock()
}

// Show displays a notification. It returns at once; a failure is logged.
// title and body must not carry user text: Windows keeps notifications in
// the Action Center's database on disk.
func (t *Toasts) Show(id, title, body string) {
	logger.Info("notify: show", "id", id, "title", title, "body", body)
	t.platform.show(id, title, body)
}

func (t *Toasts) activated(id string) {
	t.mu.Lock()
	fn := t.onActivate
	t.mu.Unlock()
	logger.Info("notify: activated", "id", id)
	if fn != nil {
		fn(id)
	}
}
