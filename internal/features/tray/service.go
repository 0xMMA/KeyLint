package tray

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"keylint/internal/logger"
)

// tooltip is what hovering the tray icon says; busyTooltip while a silent fix
// runs.
const (
	tooltip     = "KeyLint"
	busyTooltip = "KeyLint — fixing…"
)

// Service manages the system tray icon and menu.
type Service struct {
	app  *application.App
	icon []byte

	mu   sync.Mutex
	tray *application.SystemTray
	// busyMu keeps SetBusy calls posting to the main thread in the order
	// they were made; the main thread runs them in that order.
	busyMu sync.Mutex
	// busy is the icon with the working dot, built on first use; nil after a
	// failed build, in which case only the tooltip changes.
	busy     []byte
	busyOnce sync.Once
}

// NewService creates a new TrayService.
func NewService(app *application.App, icon []byte) *Service {
	return &Service{app: app, icon: icon}
}

// Setup initialises the system tray with menu items.
// window is used to show/focus the main window from tray interactions.
func (s *Service) Setup(window application.Window) {
	tray := s.app.SystemTray.New()
	tray.SetLabel("KeyLint")
	tray.SetTooltip(tooltip)
	if len(s.icon) > 0 {
		tray.SetIcon(s.icon)
	}

	menu := s.app.NewMenu()
	menu.Add("Open KeyLint").OnClick(func(ctx *application.Context) {
		logger.Info("tray: open clicked")
		window.Show().Focus()
	})
	menu.AddSeparator()
	menu.Add("Exit").OnClick(func(ctx *application.Context) {
		logger.Info("tray: quit clicked")
		s.app.Quit()
	})
	tray.SetMenu(menu)

	tray.OnClick(func() {
		tray.OpenMenu()
	})
	tray.OnDoubleClick(func() {
		window.Show().Focus()
	})

	s.mu.Lock()
	s.tray = tray
	s.mu.Unlock()
	logger.Info("tray: setup complete")
}

// SetBusy marks the tray icon while a silent fix runs: a small amber dot on
// the icon and "fixing…" in its tooltip. It is the only sign of a fix in
// progress — nothing pops up and nothing takes focus. Safe from any goroutine;
// a no-op before Setup.
//
// It never waits for the main thread: the change is posted there and the call
// returns. The silent fix calls this with its own lock held, and a stalled or
// panicking main-thread call must not hold every later key press with it.
func (s *Service) SetBusy(busy bool) {
	s.mu.Lock()
	tray := s.tray
	s.mu.Unlock()
	if tray == nil {
		return
	}
	tip, icon := tooltip, s.icon
	if busy {
		tip = busyTooltip
		if b := s.busyIcon(); b != nil {
			icon = b
		}
	}
	s.busyMu.Lock()
	defer s.busyMu.Unlock()
	application.InvokeAsync(func() {
		tray.SetTooltip(tip)
		if len(icon) > 0 {
			tray.SetIcon(icon)
		}
	})
}

func (s *Service) busyIcon() []byte {
	s.busyOnce.Do(func() {
		if len(s.icon) == 0 {
			return
		}
		icon, err := busyIcon(s.icon)
		if err != nil {
			logger.Warn("tray: no busy icon, the tooltip alone will show a fix in progress", "err", err)
			return
		}
		s.busy = icon
	})
	return s.busy
}
