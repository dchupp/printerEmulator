package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
	tcp *TCPServer
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
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
	if Running == false {
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
	PrintWidth = width
}

func (a *App) GetWidth() int {
	return PrintWidth
}

func (a *App) UpdateHeight(height int) {
	PrintHeight = height
}

func (a *App) GetHeight() int {
	return PrintHeight
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
	DPI = dpi
}

func (a *App) GetPrinterRotation() int {
	return PrintRotation
}

func (a *App) SetPrinterRotation(rotation int) {
	PrintRotation = rotation
}

func (a *App) GetPrinterDPI() PrinterDPI {
	return DPI
}
func (a *App) UpdatePrinterPort(port int) {
	CONN_PORT = port
	if Running == true {
		a.StopPrintServer()
		a.StartPrinterServer()
	}
}

func (a *App) GetPrinterPort() int {
	return CONN_PORT
}

func (a *App) SetPrintDirectory() string {
	var dialog runtime.OpenDialogOptions
	dialog.CanCreateDirectories = true
	dialog.Title = "Save Print Location"

	path, _ := runtime.OpenDirectoryDialog(a.ctx, dialog)
	FilePath = path

	return path
}

func (a *App) GetPrintDirectory() string {
	return FilePath
}
