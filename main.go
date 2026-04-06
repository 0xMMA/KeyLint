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
	application.RegisterEvent[string]("shortcut:single")
	application.RegisterEvent[string]("shortcut:double")
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
	if err := services.Shortcut.Register(); err != nil {
		log.Printf("warn: shortcut registration failed: %v", err)
		logger.Warn("shortcut: registration failed", "err", err)
	} else {
		logger.Info("shortcut: registered", "key", cfg.ShortcutKey)
	}
	wailsApp.OnShutdown(func() { services.Shortcut.Unregister() })

	// Double-press detection: single press → silent fix, double press → show Pyramidize UI.
	// Clipboard is captured on the first press (while source app still has focus).
	// The detector classifies presses and emits Single/Double results.
	detector := shortcut.NewDetector(200 * time.Millisecond)
	wailsApp.OnShutdown(func() { detector.Stop() })

	// Feed raw hotkey events into the detector; capture clipboard only on the first
	// press of each cycle. CopyFromForeground() sleeps ~150ms (Ctrl+C + wait), so
	// skipping it on the second press keeps the full 200ms window available.
	var clipboardCaptured atomic.Bool
	go func() {
		ch := services.Shortcut.Triggered()
		for event := range ch {
			logger.Info("shortcut: triggered", "source", event.Source)
			if clipboardCaptured.CompareAndSwap(false, true) {
				pyramidizeSvc.CaptureSourceApp()
				if err := services.Clipboard.CopyFromForeground(); err != nil {
					logger.Warn("shortcut: CopyFromForeground failed", "err", err)
				}
			}
			detector.Press()
		}
	}()

	// Consume classified results and emit the appropriate Wails event.
	go func() {
		for result := range detector.Result() {
			clipboardCaptured.Store(false)
			switch result {
			case shortcut.Single:
				logger.Info("shortcut: single press detected")
				wailsApp.Event.Emit("shortcut:single", "hotkey")
			case shortcut.Double:
				logger.Info("shortcut: double press detected")
				window.Show().Focus()
				wailsApp.Event.Emit("shortcut:double", "hotkey")
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
