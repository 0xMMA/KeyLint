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
	// busyMu keeps one SetBusy's icon and tooltip from interleaving with
	// another's, so the icon always ends in the state of the last call.
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
func (s *Service) SetBusy(busy bool) {
	s.mu.Lock()
	tray := s.tray
	s.mu.Unlock()
	if tray == nil {
		return
	}
	s.busyMu.Lock()
	defer s.busyMu.Unlock()
	if busy {
		tray.SetTooltip(busyTooltip)
		if icon := s.busyIcon(); icon != nil {
			tray.SetIcon(icon)
		}
		return
	}
	tray.SetTooltip(tooltip)
	if len(s.icon) > 0 {
		tray.SetIcon(s.icon)
	}
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
