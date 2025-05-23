package main

import (
	"context"
	"database/sql"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
// Add db and settings fields to App
type App struct {
	ctx      context.Context
	tcp      *TCPServer
	db       *sql.DB
	Settings *Settings
}

// NewApp creates a new App application struct
func NewApp(db *sql.DB) *App {
	err := InitSettingsTable(db)
	if err != nil {
		panic(err)
	}
	// Initialize printers table at startup
	err = InitPrintersTable(db)
	if err != nil {
		panic(err)
	}
	// Initialize relay_groups table at startup
	err = InitRelayGroupsTable(db)
	if err != nil {
		panic(err)
	}
	settings, err := LoadSettingsFromDB(db)
	if err != nil {
		// If no settings exist, create default
		settings = &Settings{
			SettingID:      1,
			PrintWidth:     4,
			PrintHeight:    6,
			PrintRotation:  0,
			PrinterPort:    9100,
			PrintPath:      "",
			PrinterDPI:     PrinterDPI{Dpi: 8, Description: "8 dpmm (203 dpi)"},
			DefaultPrinter: 0,
		}
		_ = settings.SaveToDB(db)
	}
	return &App{db: db, Settings: settings}
}

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	// Perform your setup here
	a.ctx = ctx

}

// domReady is called after front-end resources have been loaded
func (a App) domReady(ctx context.Context) {
	// Add your action here
	// a.tcp = a.NewTCPServer()
	// a.serve()
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	// Perform your teardown here
}

func (a *App) StartPrinterServer() {
	// active := a.GetPrinterRunStatus()
	if !Running {
		a.tcp = a.NewTCPServer()
		if a.tcp == nil {
			Running = false
			return
		}
		a.tcp.wg.Add(1)
	}
	// a.serve()
}

func (a *App) UpdateSave(fileSave bool) {
	SaveToFile = fileSave
}

func (a *App) UpdateWidth(width int) {
	a.Settings.PrintWidth = float64(width)
	a.Settings.SaveToDB(a.db)
}

func (a *App) GetWidth() int {
	return int(a.Settings.PrintWidth)
}

func (a *App) UpdateHeight(height int) {
	a.Settings.PrintHeight = float64(height)
	a.Settings.SaveToDB(a.db)
}

func (a *App) GetHeight() int {

	return int(a.Settings.PrintHeight)
}

func (a *App) StopPrintServer() {
	a.tcp.Stop()
	runtime.EventsEmit(a.ctx, "Unblock")
}
func (a *App) GetPrinterRunStatus() bool {
	// active := a.tcp.GetStatus()
	return Running
}
func (a *App) UpdatePrinterDPI(dpi PrinterDPI) {
	a.Settings.PrinterDPI = dpi
	a.Settings.SaveToDB(a.db)
}

func (a *App) GetPrinterRotation() int {
	return int(a.Settings.PrintRotation)
}

func (a *App) SetPrinterRotation(rotation int) {
	a.Settings.PrintRotation = float64(rotation)
	a.Settings.SaveToDB(a.db)
}

func (a *App) GetPrinterDPI() PrinterDPI {
	return a.Settings.PrinterDPI
}
func (a *App) UpdatePrinterPort(port int) {
	a.Settings.PrinterPort = float64(port)
	a.Settings.SaveToDB(a.db)
	if Running {
		a.StopPrintServer()
		a.StartPrinterServer()
	}
}

func (a *App) GetPrinterPort() int {
	return int(a.Settings.PrinterPort)
}

func (a *App) SetPrintDirectory() string {
	var dialog runtime.OpenDialogOptions
	dialog.CanCreateDirectories = true
	dialog.Title = "Save Print Location"

	path, _ := runtime.OpenDirectoryDialog(a.ctx, dialog)
	a.Settings.PrintPath = path
	a.Settings.SaveToDB(a.db)

	return path
}
func (a *App) ClearPrintDirectory() {
	a.Settings.PrintPath = ""
	a.Settings.SaveToDB(a.db)
}
func (a *App) GetPrintDirectory() string {
	return a.Settings.PrintPath
}

func (a *App) AddPrinter(printer Printer) error {
	return AddPrinter(a.db, &printer)
}

func (a *App) GetPrinters() ([]Printer, error) {
	return GetPrinters(a.db)
}

func (a *App) UpdatePrinter(printer Printer) error {
	return UpdatePrinter(a.db, &printer)
}

func (a *App) DeletePrinter(printerID int) error {
	return DeletePrinter(a.db, printerID)
}

// Relay group methods for Wails frontend
func (a *App) AddRelayGroup(printerIDs []int) error {
	return AddRelayGroup(a.db, printerIDs)
}

func (a *App) GetRelayGroups() ([]RelayGroup, error) {
	return GetRelayGroups(a.db)
}

func (a *App) DeleteRelayGroup(groupID int) error {
	return DeleteRelayGroup(a.db, groupID)
}

func (a *App) SetPrinterEmulatorMode() {
	PrintMode = 0
}
func (a *App) SetPrinterRelayMode() {
	PrintMode = 2
}
func (a *App) SetPrinterZPLToPrinterMode() {
	PrintMode = 1
}
func (a *App) SelectPrinter(printer Printer) {
	SelectedPrinter = printer
}
func (a *App) SelectRelayGroup(relayGroup RelayGroup) {
	LabelRelayGroup = relayGroup
}
