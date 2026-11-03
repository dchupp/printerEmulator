package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	dir, err := configDir()
	if err != nil {
		log.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(dir, "printEmulator.db"))
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	svc := NewEmulatorService(store)

	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "ZPL Printer Emulator",
		Description: "Emulates a Zebra ZPL printer: preview, forward and relay labels",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler:        application.AssetFileServerFS(assets),
			Middleware:     svc.history.Middleware,
			DisableLogging: true,
		},
		SingleInstance: singleInstance(func() {
			if window != nil {
				window.Show()
				window.Restore()
				window.Focus()
			}
		}),
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "ZPL Printer Emulator",
		Width:            1240,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(26, 22, 37),
		URL:              "/",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 0,
			Backdrop:                application.MacBackdropNormal,
		},
	})

	// Closing the window either quits or, if the user chose it, hides the
	// window and keeps the printer running in the tray.
	var quitting atomic.Bool
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if quitting.Load() || !svc.Settings().CloseToTray {
			quitting.Store(true)
			app.Quit()
			return
		}
		e.Cancel()
		window.Hide()
	})

	setupTray(app, window, svc, &quitting)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func setupTray(app *application.App, window *application.WebviewWindow, svc *EmulatorService, quitting *atomic.Bool) {
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetTooltip("ZPL Printer Emulator")

	menu := app.NewMenu()
	menu.Add("Show Printer Emulator").OnClick(func(*application.Context) {
		window.Show()
		window.Restore()
		window.Focus()
	})
	menu.AddSeparator()
	status := menu.Add("Printer stopped").SetEnabled(false)
	toggle := menu.Add("Start printer")
	toggle.OnClick(func(*application.Context) {
		if svc.Status().Running {
			svc.Stop()
		} else {
			svc.Start()
		}
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		quitting.Store(true)
		app.Quit()
	})
	tray.SetMenu(menu)
	tray.OnClick(func() {
		window.Show()
		window.Restore()
		window.Focus()
	})

	svc.onStatus = func(st ServerStatus) {
		if st.Running {
			status.SetLabel("Listening on " + st.Address)
			toggle.SetLabel("Stop printer")
			tray.SetTooltip("ZPL Printer Emulator — " + st.Address)
		} else {
			status.SetLabel("Printer stopped")
			toggle.SetLabel("Start printer")
			tray.SetTooltip("ZPL Printer Emulator — stopped")
		}
		menu.Update()
	}
}

// singleInstance focuses the running window when the app is started a
// second time, so two emulators never fight over the port. On Linux it
// needs a D-Bus session; without one it is skipped rather than failing.
func singleInstance(focus func()) *application.SingleInstanceOptions {
	if runtime.GOOS == "linux" && os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return nil
	}
	return &application.SingleInstanceOptions{
		UniqueID:               "com.datamagik.zplprinteremulator",
		OnSecondInstanceLaunch: func(application.SecondInstanceData) { focus() },
	}
}

// configDir is the folder holding the database. It is the same folder 2.x
// used, so existing settings and printers carry over.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find config folder: %w", err)
	}
	dir := filepath.Join(base, "DataGenie")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config folder: %w", err)
	}
	return dir, nil
}
