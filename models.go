package main

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
)

// Mode decides what happens to a print job received on the printer port.
type Mode string

const (
	// ModePreview renders labels on screen (and optionally to disk).
	ModePreview Mode = "preview"
	// ModeForward sends each job to one network printer.
	ModeForward Mode = "forward"
	// ModeRelay sends each job to every printer in a relay group.
	ModeRelay Mode = "relay"
)

// Settings are the user's preferences. Everything is saved on change.
type Settings struct {
	// LabelWidth and LabelHeight are the media size in inches.
	LabelWidth  float64 `json:"labelWidth"`
	LabelHeight float64 `json:"labelHeight"`
	// DPMM is the print density in dots per millimetre: 6, 8, 12 or 24.
	DPMM int `json:"dpmm"`
	// Rotation turns the rendered image clockwise: 0, 90, 180 or 270.
	Rotation int `json:"rotation"`
	// ExpandCopies renders every ^PQ copy instead of one image per format.
	ExpandCopies bool `json:"expandCopies"`

	// Port is the raw TCP port the emulator listens on (9100 like a Zebra).
	Port int `json:"port"`
	// ListenAll accepts jobs from other computers instead of only localhost.
	ListenAll bool `json:"listenAll"`
	// AutoStartServer starts the printer when the app opens.
	AutoStartServer bool `json:"autoStartServer"`

	Mode                   Mode `json:"mode"`
	ForwardPrinterID       int  `json:"forwardPrinterID"`
	RelayGroupID           int  `json:"relayGroupID"`
	PreviewWhileForwarding bool `json:"previewWhileForwarding"`

	// SaveLabels writes every rendered label to SaveDir as a PNG.
	SaveLabels bool   `json:"saveLabels"`
	SaveDir    string `json:"saveDir"`
	// SaveZPL also writes the raw job next to the PNGs.
	SaveZPL bool `json:"saveZPL"`

	// HistoryLimit is how many labels the preview keeps in memory.
	HistoryLimit int `json:"historyLimit"`
	// CloseToTray hides the window on close; the printer keeps running.
	CloseToTray bool `json:"closeToTray"`
	// Theme is "auto", "light" or "dark".
	Theme string `json:"theme"`
}

func DefaultSettings() Settings {
	return Settings{
		LabelWidth:   4,
		LabelHeight:  6,
		DPMM:         8,
		Port:         9100,
		Mode:         ModePreview,
		HistoryLimit: 200,
		Theme:        "auto",
	}
}

// Normalize clamps every field into its valid range.
func (s Settings) Normalize() Settings {
	d := DefaultSettings()
	if !(s.LabelWidth > 0) || s.LabelWidth > 30 || math.IsNaN(s.LabelWidth) {
		s.LabelWidth = d.LabelWidth
	}
	if !(s.LabelHeight > 0) || s.LabelHeight > 60 || math.IsNaN(s.LabelHeight) {
		s.LabelHeight = d.LabelHeight
	}
	switch s.DPMM {
	case 6, 8, 12, 24:
	default:
		s.DPMM = d.DPMM
	}
	switch s.Rotation {
	case 0, 90, 180, 270:
	default:
		s.Rotation = 0
	}
	if s.Port <= 0 || s.Port > 65535 {
		s.Port = d.Port
	}
	switch s.Mode {
	case ModePreview, ModeForward, ModeRelay:
	default:
		s.Mode = ModePreview
	}
	if s.HistoryLimit <= 0 {
		s.HistoryLimit = d.HistoryLimit
	}
	if s.HistoryLimit > 2000 {
		s.HistoryLimit = 2000
	}
	switch s.Theme {
	case "auto", "light", "dark":
	default:
		s.Theme = "auto"
	}
	return s
}

// Printer types.
const (
	PrinterTypeZebra = "Zebra" // raw ZPL over TCP (port 9100)
	PrinterTypeIPP   = "IPP"   // rendered locally, sent as PDF over IPP
)

// Printer is a network printer jobs can be forwarded or relayed to.
type Printer struct {
	PrinterID   int    `json:"printerID"`
	PrinterName string `json:"printerName"`
	IPAddress   string `json:"ipAddress"`
	PrinterPort int    `json:"printerPort"`
	PrinterType string `json:"printerType"`
	IPPEndpoint string `json:"ippEndpoint"`
	UseTLS      bool   `json:"useTLS"`
}

func (p Printer) Normalize() (Printer, error) {
	p.PrinterName = strings.TrimSpace(p.PrinterName)
	p.IPAddress = strings.TrimSpace(p.IPAddress)
	p.IPPEndpoint = strings.TrimSpace(p.IPPEndpoint)
	if p.PrinterName == "" {
		return p, errors.New("printer name is required")
	}
	if p.IPAddress == "" {
		return p, errors.New("IP address or host name is required")
	}
	if strings.ContainsAny(p.IPAddress, "/ ") {
		return p, fmt.Errorf("%q is not a valid host", p.IPAddress)
	}
	switch p.PrinterType {
	case PrinterTypeZebra:
		if p.PrinterPort == 0 {
			p.PrinterPort = 9100
		}
	case PrinterTypeIPP:
		if p.PrinterPort == 0 {
			p.PrinterPort = 631
		}
		if p.IPPEndpoint == "" {
			p.IPPEndpoint = "/ipp/print"
		}
		if !strings.HasPrefix(p.IPPEndpoint, "/") {
			p.IPPEndpoint = "/" + p.IPPEndpoint
		}
	default:
		return p, fmt.Errorf("unknown printer type %q", p.PrinterType)
	}
	if p.PrinterPort < 1 || p.PrinterPort > 65535 {
		return p, fmt.Errorf("port %d is out of range", p.PrinterPort)
	}
	return p, nil
}

func (p Printer) Address() string {
	return net.JoinHostPort(p.IPAddress, fmt.Sprint(p.PrinterPort))
}

// RelayGroup is a named set of printers that each receive every job.
type RelayGroup struct {
	GroupID    int    `json:"groupID"`
	Name       string `json:"name"`
	PrinterIDs []int  `json:"printerIDs"`
}

// Label describes one rendered label. The PNG itself is served from URL so
// images never travel through the event bridge.
type Label struct {
	ID       int64    `json:"id"`
	JobID    int64    `json:"jobID"`
	Index    int      `json:"index"`
	Received int64    `json:"received"` // Unix milliseconds
	Width    int      `json:"width"`    // pixels
	Height   int      `json:"height"`   // pixels
	DPMM     int      `json:"dpmm"`
	Source   string   `json:"source"`
	Bytes    int      `json:"bytes"`
	URL      string   `json:"url"`
	Warnings []string `json:"warnings"`
}

// Delivery is the outcome of sending a job to one printer.
type Delivery struct {
	PrinterID   int    `json:"printerID"`
	PrinterName string `json:"printerName"`
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	Millis      int64  `json:"millis"`
}

// Job is one print job received on the port (or submitted from the UI).
type Job struct {
	ID         int64      `json:"id"`
	Received   int64      `json:"received"` // Unix milliseconds
	Source     string     `json:"source"`
	Bytes      int        `json:"bytes"`
	Mode       Mode       `json:"mode"`
	Labels     int        `json:"labels"`
	RenderMs   float64    `json:"renderMs"`
	Deliveries []Delivery `json:"deliveries"`
	Error      string     `json:"error"`
}

// ServerStatus is pushed to the UI whenever the printer starts or stops.
type ServerStatus struct {
	Running bool   `json:"running"`
	Address string `json:"address"`
	Error   string `json:"error"`
}

// Stats summarises activity since the app started.
type Stats struct {
	Jobs        int64   `json:"jobs"`
	Labels      int64   `json:"labels"`
	Failures    int64   `json:"failures"`
	AvgRenderMs float64 `json:"avgRenderMs"`
}
