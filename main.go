package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"keylint/internal/app"
	"keylint/internal/cli"
	"keylint/internal/features/enhance"
	featurelogger "keylint/internal/features/logger"
	"keylint/internal/features/pyramidize"
	"keylint/internal/features/settings"
	"keylint/internal/features/shortcut"
	"keylint/internal/features/updater"
	"keylint/internal/logger"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// AppVersion is injected at build time via -ldflags "-X main.AppVersion=x.y.z".
var AppVersion = "dev"

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func init() {
	application.RegisterEvent[string]("shortcut:fix")
	application.RegisterEvent[string]("shortcut:pyramidize")
	application.RegisterEvent[string]("settings:changed")
}

func main() {
	// CLI dispatch — runs headlessly, no Wails/GUI.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-fix", "-pyramidize":
			if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	simulateShortcut := flag.Bool("simulate-shortcut", false, "Fire a synthetic shortcut event on startup (Linux dev mode)")
	flag.Parse()

	wailsApp := application.New(application.Options{
		Name:        "KeyLint",
		Description: "AI-powered text enhancement",
		Icon:        appIcon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	services, err := app.InitializeApp(wailsApp, app.AppIcon(appIcon))
	if err != nil {
		log.Fatalf("failed to initialise app: %v", err)
	}

	// Initialize structured logger based on the saved settings.
	cfg := services.Settings.Get()
	logger.Init(cfg.LogLevel, cfg.SensitiveLogging)
	logger.Info("app initializing", "version", AppVersion)
	// The settings service read its file before logging was up, so its own
	// lines went nowhere; this is the record of what it found.
	if loaded := settings.LoadOutcome(services.Settings); loaded.Found {
		logger.Info("settings: loaded at start-up", "path", loaded.Path, "completed_setup", loaded.CompletedSetup)
	} else {
		logger.Info("settings: no file at start-up, using defaults", "path", loaded.Path)
	}

	// Register backend services so the frontend can call their methods.
	wailsApp.RegisterService(application.NewService(services.Settings))
	wailsApp.RegisterService(application.NewService(services.Welcome))
	wailsApp.RegisterService(application.NewService(services.Clipboard))
	wailsApp.RegisterService(application.NewService(enhance.NewService(services.Settings)))

	// Pyramidize service — captures source app on hotkey and exposes RPC methods.
	pyramidizeSvc := pyramidize.NewService(services.Settings, services.Clipboard)
	wailsApp.RegisterService(application.NewService(pyramidizeSvc))

	// Log service — forwards frontend log messages into debug.log.
	wailsApp.RegisterService(application.NewService(featurelogger.NewService()))

	// Updater service — AppVersion injected at build time via ldflags.
	updaterSvc := updater.NewService(AppVersion, services.Settings)
	updaterSvc.SetQuitFunc(func() {
		// Brief delay so the frontend can display the "closing" message.
		time.Sleep(2 * time.Second)
		wailsApp.Quit()
	})
	wailsApp.RegisterService(application.NewService(updaterSvc))

	// Dev-tools shortcut simulation service.
	sim := &simulateService{shortcut: services.Shortcut}
	wailsApp.RegisterService(application.NewService(sim))

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "KeyLint",
		Width:            1280,
		Height:           800,
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})

	// Hide to tray on close instead of quitting.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})

	logger.Info("window created")

	// Start the system tray.
	services.Tray.Setup(window)

	// Register the global shortcut (no-op on Linux).
	// Unregister on shutdown so dev-mode restarts don't leave a stale registration.
	// hookActive is false while no hook is installed, so a save that fixes the
	// shortcuts after a failed registration installs it instead of updating a
	// hook that does not exist.
	var hookActive atomic.Bool
	if err := services.Shortcut.Register(shortcutConfigOf(cfg)); err != nil {
		log.Printf("warn: shortcut registration failed: %v", err)
		logger.Warn("shortcut: registration failed", "err", err)
	} else {
		hookActive.Store(true)
		logger.Info("shortcut: registered", "mode", cfg.ShortcutMode, "fix", cfg.ShortcutFix)
	}
	wailsApp.OnShutdown(func() { services.Shortcut.Unregister() })

	// Apply saved shortcuts straight away. Every write goes through the
	// settings service, so this sees Save and Reset to defaults alike; writes
	// that leave the shortcuts alone (a provider switch, say) are skipped, so
	// they do not reset a double tap in progress.
	settings.OnSaved(services.Settings, func(old, updated settings.Settings) {
		next := shortcutConfigOf(updated)
		if hookActive.Load() {
			if next == shortcutConfigOf(old) {
				return
			}
			if err := services.Shortcut.UpdateConfig(next); err != nil {
				logger.Warn("shortcut: applying saved shortcuts failed", "err", err)
			}
			return
		}
		if err := services.Shortcut.Register(next); err != nil {
			logger.Warn("shortcut: registration failed", "err", err)
			return
		}
		hookActive.Store(true)
		logger.Info("shortcut: registered", "mode", updated.ShortcutMode, "fix", updated.ShortcutFix)
	})

	// Forward classified shortcut events to the frontend.
	go func() {
		for event := range services.Shortcut.Triggered() {
			logger.Info("shortcut: action", "action", event.Action, "source", event.Source)
			pyramidizeSvc.CaptureSourceApp()
			if err := services.Clipboard.CopyFromForeground(); err != nil {
				logger.Warn("shortcut: CopyFromForeground failed", "err", err)
			}
			switch event.Action {
			case "fix":
				wailsApp.Event.Emit("shortcut:fix", event.Source)
			case "pyramidize":
				window.Show().Focus()
				wailsApp.Event.Emit("shortcut:pyramidize", event.Source)
			}
		}
	}()

	// Simulate shortcut on startup when --simulate-shortcut flag is set.
	if *simulateShortcut {
		if s, ok := services.Shortcut.(interface{ Simulate() }); ok {
			go s.Simulate()
		}
	}

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// simulateService exposes SimulateShortcut to the frontend (used by dev-tools button).
type simulateService struct {
	shortcut shortcut.Service
}

func (s *simulateService) SimulateShortcut() {
	if sim, ok := s.shortcut.(interface{ Simulate() }); ok {
		sim.Simulate()
	}
}

// SetShortcutPaused temporarily disables shortcut detection (e.g. while recording a new shortcut in settings).
func (s *simulateService) SetShortcutPaused(paused bool) {
	s.shortcut.SetPaused(paused)
}

// shortcutConfigOf is the part of the settings the shortcut hook runs on.
func shortcutConfigOf(s settings.Settings) shortcut.ShortcutConfig {
	return shortcut.ShortcutConfig{
		Mode:            s.ShortcutMode,
		FixCombo:        s.ShortcutFix,
		PyramidizeCombo: s.ShortcutPyramidize,
		DoubleTapDelay:  time.Duration(s.ShortcutDoubleTapDelay) * time.Millisecond,
	}
}
