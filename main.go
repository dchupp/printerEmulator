package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist/spa
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	printers, err := QueryInstalledPrinters()
	if err != nil {
		fmt.Errorf("Error querying installed printers: %v", err)
	}
	for _, v := range printers {
		fmt.Println(v)

	}
	configPath, err := getMyAppConfigPath()
	if err != nil {
		log.Fatalf("Error getting application config path: %v", err)
	}

	dbFilePath := filepath.Join(configPath, "printEmulator.db")
	db, err := ConnectSQLLite3(dbFilePath)
	if err != nil {
		log.Fatalf("Cannot connect to database: %v", err)
	}
	defer db.SQL.Close()
	// Create an instance of the app structure
	app := NewApp(db.SQL)
	// tcp := app.NewTCPServer()
	// app.tcp = tcp
	// defer app.tcp.Stop()

	// Create application with options
	err = wails.Run(&options.App{
		Title:             "Printer_Emulator",
		Width:             800,
		Height:            600,
		MinWidth:          800,
		MinHeight:         580,
		MaxWidth:          4096,
		MaxHeight:         4096,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Menu:             nil,
		Logger:           nil,
		LogLevel:         logger.DEBUG,
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		WindowStartState: options.Normal,
		Bind: []interface{}{
			app,
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			// DisableFramelessWindowDecorations: false,
			WebviewUserDataPath: "",
			ZoomFactor:          0.90,
			// IsZoomControlEnabled enables the zoom factor to be changed by the user.
			// IsZoomControlEnabled: true,
		},
		// Mac platform specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "Printer_Emulator",
				Message: "",
				Icon:    icon,
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

func getAppDataRoamingPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("error getting user config directory: %w", err)
	}
	return configDir, nil
}
func getMyAppConfigPath() (string, error) {
	appDataPath, err := getAppDataRoamingPath()
	if err != nil {
		return "", err
	}

	myAppPath := filepath.Join(appDataPath, "DataGenie")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(myAppPath, 0755); err != nil {
		return "", fmt.Errorf("error creating application config directory: %w", err)
	}

	return myAppPath, nil
}
