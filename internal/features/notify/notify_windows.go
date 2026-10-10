//go:build windows

package notify

import (
	"bytes"
	"errors"
	"syscall"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"git.sr.ht/~jackmordaunt/go-toast/v2/tmpl"
	"git.sr.ht/~jackmordaunt/go-toast/v2/wintoast"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"keylint/internal/logger"
)

// activatorGUID is the COM class Windows calls back when a KeyLint toast is
// clicked. It is KeyLint's own: go-toast's default GUID is shared by every app
// that leaves it alone, and two apps on one CLSID would get each other's
// clicks. Generated once for KeyLint; never change it, or toasts already in
// the Action Center stop reaching the app.
const activatorGUID = "{CEF3EFCF-75E1-4709-9868-A73D680E3BEE}"

// queueSize bounds toasts waiting to be pushed. The caller already allows at
// most one per hotkey press; anything beyond this is dropped, not queued.
const queueSize = 4

type toastJob struct{ id, title, body string }

type windowsToasts struct {
	appName string
	jobs    chan toastJob
}

func newPlatform(appName string, icon []byte, activated func(string)) platform {
	w := &windowsToasts{appName: appName, jobs: make(chan toastJob, queueSize)}
	ready := make(chan bool, 1)
	go w.run(icon, activated, ready)
	// Registration is a few registry writes; waiting for it means the first
	// toast cannot race it. A failure is logged inside and turns toasts off.
	if !<-ready {
		w.jobs = nil
	}
	return w
}

func (w *windowsToasts) show(id, title, body string) {
	if w.jobs == nil {
		return
	}
	select {
	case w.jobs <- toastJob{id, title, body}:
	default:
		logger.Warn("notify: queue full, notification dropped", "id", id)
	}
}

// run owns one locked OS thread for the life of the app. COM state belongs to
// a thread, and goroutines move between threads, so every push happens here.
func (w *windowsToasts) run(icon []byte, activated func(string), ready chan<- bool) {
	runtime.LockOSThread()

	if err := w.register(icon, activated); err != nil {
		logger.Warn("notify: notifications are off, registration failed", "err", err)
		ready <- false
		return
	}
	ready <- true

	for job := range w.jobs {
		err := w.push(job)
		if err != nil && strings.Contains(err.Error(), "RoInitialize") {
			// go-toast initialises once per process, on the first push, and
			// treats "this thread was already initialised" as a failure. It
			// does not try again, so neither does the retry: this one goes.
			err = w.push(job)
		}
		if err != nil {
			logger.Warn("notify: toast failed", "id", job.id, "err", err)
		}
	}
}

// register tells Windows who KeyLint is and where to send clicks: the
// AppUserModelID registry method for unpackaged desktop apps
// (HKCU\Software\Classes\AppUserModelId\<AUMID> with DisplayName, IconUri and
// CustomActivator), plus the CLSID that CustomActivator names. No Start-menu
// shortcut is involved, so it works the same for the NSIS install, a
// dev-channel build, and an exe run from a build folder. Everything is under
// HKCU and written on every start, so the paths follow the exe that last ran.
func (w *windowsToasts) register(icon []byte, activated func(string)) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating the executable: %w", err)
	}
	iconPath, err := writeIcon(icon)
	if err != nil {
		// A toast without an icon still says what it has to.
		logger.Warn("notify: icon not written", "err", err)
	}

	aumidKey := `Software\Classes\AppUserModelId\` + w.appName
	values := map[string]string{"DisplayName": w.appName, "CustomActivator": activatorGUID}
	if iconPath != "" {
		values["IconUri"] = iconPath
	}
	if err := setValues(aumidKey, values); err != nil {
		return err
	}
	// Quoted: an install under Program Files has a space in its path. COM
	// appends -Embedding when it has to start the app for a click.
	serverKey := `Software\Classes\CLSID\` + activatorGUID + `\LocalServer32`
	if err := setValues(serverKey, map[string]string{"": `"` + exe + `"`}); err != nil {
		return err
	}

	// The callback must be in place before the class is registered: go-toast
	// calls it unguarded from a COM thread.
	wintoast.SetActivationCallback(func(_ string, args string, _ []wintoast.UserData) {
		activated(args)
	})
	// Values above are already written, so go-toast's own registry writes
	// find them and leave them alone; this records the AUMID and GUID for Push.
	if err := wintoast.SetAppData(wintoast.AppData{AppID: w.appName, GUID: activatorGUID}); err != nil {
		return fmt.Errorf("setting app data: %w", err)
	}

	// Registering the class up front makes this process the one Windows
	// calls for a click from the start. go-toast only registers when it
	// pushes, so without this a click on a toast left in the Action Center by
	// an earlier session would start a second KeyLint. Not fatal: toasts this
	// session pushes register it anyway.
	registered := make(chan error, 1)
	go func() {
		// A thread of its own, parked for good once registered: it keeps the
		// multithreaded apartment alive that the class lives in, and it keeps
		// CoInitializeEx off the push thread, where go-toast's RoInitialize
		// must be the first COM call — a second one answers S_FALSE, which
		// go-toast reports as a failure.
		runtime.LockOSThread()
		err := registerActivator()
		registered <- err
		if err == nil {
			select {}
		}
	}()
	if err := <-registered; err != nil {
		logger.Warn("notify: clicks on toasts from an earlier session will start KeyLint again", "err", err)
	}
	return nil
}

// registerActivator registers go-toast's activation class under
// activatorGUID, in the multithreaded apartment this thread joins.
func registerActivator() error {
	// S_FALSE: this thread was already in the multithreaded apartment, which
	// is what we want. x/sys reports it as an error.
	if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err != nil && !errors.Is(err, syscall.Errno(1)) {
		return fmt.Errorf("CoInitializeEx: %w", err)
	}
	const (
		clsctxLocalServer = 0x4
		regclsMultipleUse = 1
	)
	var cookie uint32
	hr, _, _ := procCoRegisterClassObject.Call(
		uintptr(unsafe.Pointer(wintoast.GUID_ImplNotificationActivationCallback)),
		uintptr(unsafe.Pointer(wintoast.ClassFactory)),
		clsctxLocalServer,
		regclsMultipleUse,
		uintptr(unsafe.Pointer(&cookie)),
	)
	if hr != 0 {
		return fmt.Errorf("CoRegisterClassObject: HRESULT 0x%08x", uint32(hr))
	}
	return nil
}

var procCoRegisterClassObject = windows.NewLazySystemDLL("ole32.dll").NewProc("CoRegisterClassObject")

// push shows one toast: silent, short, and activating the app in the
// foreground only when clicked. The XML is built from go-toast's template and
// pushed over COM directly — toast.Notification.Push would fall back to
// spawning PowerShell when COM fails, which is not something a grammar tool
// should be seen doing.
func (w *windowsToasts) push(job toastJob) error {
	n := toast.Notification{
		AppID:               w.appName,
		Title:               job.title,
		Body:                job.body,
		ActivationType:      toast.Foreground,
		ActivationArguments: job.id,
		Audio:               toast.Silent,
		Duration:            toast.Short,
	}
	var xml bytes.Buffer
	if err := tmpl.XMLTemplate.Execute(&xml, &n); err != nil {
		return fmt.Errorf("building toast: %w", err)
	}
	return wintoast.Push(w.appName, xml.String())
}

// writeIcon puts the app icon where IconUri can point at it.
func writeIcon(icon []byte) (string, error) {
	if len(icon) == 0 {
		return "", nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "KeyLint")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "notification-icon.png")
	if err := os.WriteFile(path, icon, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func setValues(path string, values map[string]string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer key.Close()
	for name, value := range values {
		if err := key.SetStringValue(name, value); err != nil {
			return fmt.Errorf("writing %s\\%s: %w", path, name, err)
		}
	}
	return nil
}
