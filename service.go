package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AppVersion is shown in the UI and written into the build metadata.
const AppVersion = "3.0.1"

// autostartID is the registry value name 2.x used, so an existing
// "start at login" entry keeps working after the upgrade.
const autostartID = "ZPLPrinterEmulator"

// Events pushed to the frontend.
const (
	EventLabelsAdded   = "labels:added"
	EventLabelsCleared = "labels:cleared"
	EventJobDone       = "job:done"
	EventServerStatus  = "server:status"
	EventSettings      = "settings:changed"
)

func init() {
	application.RegisterEvent[[]Label](EventLabelsAdded)
	application.RegisterEvent[Job](EventJobDone)
	application.RegisterEvent[ServerStatus](EventServerStatus)
	application.RegisterEvent[Settings](EventSettings)
	application.RegisterEvent[application.Void](EventLabelsCleared)
}

const (
	sourceEditor  = "ZPL editor"
	sourcePreview = "ZPL editor (preview)"
)

type queuedJob struct {
	data   []byte
	source string
	done   chan Job // nil for jobs from the network
}

// EmulatorService is the backend the UI talks to. Every exported method is
// callable from the frontend through the generated bindings.
type EmulatorService struct {
	store    *Store
	renderer Renderer
	history  *History
	queue    chan queuedJob

	// emit sends an event to the UI; replaced in tests.
	emit func(name string, data any)
	// onStatus lets the tray menu follow the server state.
	onStatus func(ServerStatus)

	mu       sync.Mutex
	settings Settings
	server   *PrintServer
	status   ServerStatus

	nextJob   atomic.Int64
	nextLabel atomic.Int64
	stats     struct {
		sync.Mutex
		Stats
		renderTotal float64
		rendered    int64
	}
}

func NewEmulatorService(store *Store) *EmulatorService {
	st := store.LoadSettings()
	s := &EmulatorService{
		store:    store,
		history:  NewHistory(st.HistoryLimit),
		queue:    make(chan queuedJob, 256),
		settings: st,
		emit: func(name string, data any) {
			if app := application.Get(); app != nil {
				app.Event.Emit(name, data)
			}
		},
	}
	go s.worker()
	return s
}

// ServiceStartup starts the printer if the user asked for that.
func (s *EmulatorService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if s.Settings().AutoStartServer {
		s.Start()
	}
	return nil
}

func (s *EmulatorService) ServiceShutdown() error {
	s.Stop()
	return nil
}

// ---- settings ----------------------------------------------------------

func (s *EmulatorService) Version() string { return AppVersion }

func (s *EmulatorService) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// UpdateSettings saves new settings and applies them. If the port or
// network access changed while the printer runs, it is restarted.
func (s *EmulatorService) UpdateSettings(next Settings) (Settings, error) {
	next = next.Normalize()
	s.mu.Lock()
	prev := s.settings
	s.settings = next
	running := s.server != nil
	s.mu.Unlock()
	if err := s.store.SaveSettings(next); err != nil {
		return next, err
	}
	s.history.SetLimit(next.HistoryLimit)
	s.emit(EventSettings, next)
	if running && (prev.Port != next.Port || prev.ListenAll != next.ListenAll) {
		s.Stop()
		if _, err := s.Start(); err != nil {
			return next, err
		}
	}
	return next, nil
}

// ChooseSaveDirectory asks for a folder to save labels to. An empty result
// means the user cancelled.
func (s *EmulatorService) ChooseSaveDirectory() (string, error) {
	app := application.Get()
	dlg := app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		SetTitle("Save labels to")
	if dir := s.Settings().SaveDir; dir != "" {
		dlg.SetDirectory(dir)
	}
	path, err := dlg.PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	st := s.Settings()
	st.SaveDir = path
	_, err = s.UpdateSettings(st)
	return path, err
}

// OpenSaveDirectory shows the label folder in the file manager.
func (s *EmulatorService) OpenSaveDirectory() error {
	dir := s.Settings().SaveDir
	if dir == "" {
		return errors.New("no save folder is set")
	}
	return application.Get().Browser.OpenFile(dir)
}

func (s *EmulatorService) AutoStartEnabled() bool {
	ok, err := application.Get().Autostart.IsEnabled()
	return err == nil && ok
}

func (s *EmulatorService) SetAutoStart(enabled bool) error {
	a := application.Get().Autostart
	if enabled {
		return a.EnableWithOptions(application.AutostartOptions{Identifier: autostartID})
	}
	return a.Disable()
}

// ---- printer server ----------------------------------------------------

func (s *EmulatorService) Status() ServerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func listenAddr(st Settings) string {
	host := "127.0.0.1"
	if st.ListenAll {
		host = "0.0.0.0"
	}
	return net.JoinHostPort(host, strconv.Itoa(st.Port))
}

// Start opens the printer port.
func (s *EmulatorService) Start() (ServerStatus, error) {
	s.mu.Lock()
	if s.server != nil {
		st := s.status
		s.mu.Unlock()
		return st, nil
	}
	srv, err := StartPrintServer(listenAddr(s.settings), jobSink{s})
	if err != nil {
		s.status = ServerStatus{Error: friendlyListenError(err, s.settings.Port)}
	} else {
		s.server = srv
		s.status = ServerStatus{Running: true, Address: srv.Addr()}
	}
	st := s.status
	s.mu.Unlock()
	s.publishStatus(st)
	if err != nil {
		return st, errors.New(st.Error)
	}
	return st, nil
}

// Stop closes the printer port. Jobs already received still finish.
func (s *EmulatorService) Stop() ServerStatus {
	s.mu.Lock()
	srv := s.server
	s.server = nil
	s.status = ServerStatus{}
	s.mu.Unlock()
	if srv != nil {
		srv.Stop()
	}
	s.publishStatus(ServerStatus{})
	return ServerStatus{}
}

func (s *EmulatorService) publishStatus(st ServerStatus) {
	s.emit(EventServerStatus, st)
	if s.onStatus != nil {
		s.onStatus(st)
	}
}

func friendlyListenError(err error, port int) string {
	if msg := err.Error(); strings.Contains(msg, "address already in use") || strings.Contains(msg, "Only one usage of each socket address") {
		return fmt.Sprintf("Port %d is already in use. Close the other program using it or choose another port.", port)
	}
	return err.Error()
}

// ---- jobs --------------------------------------------------------------

// handleJob queues a job read from the printer port. Jobs are processed in
// the order they arrive, one at a time, like a real printer.
func (s *EmulatorService) handleJob(data []byte, source string) {
	s.queue <- queuedJob{data: append([]byte(nil), data...), source: source}
}

// hostResponse answers ~HS and ~HI so drivers that poll the printer see a
// ready printer instead of timing out.
func (s *EmulatorService) hostResponse(q string) []byte {
	st := s.Settings()
	switch q {
	case "~HS":
		length := int(st.LabelHeight*25.4*float64(st.DPMM) + 0.5)
		return fmt.Appendf(nil, "\x02030,0,0,%04d,000,0,0,0,000,0,0,0\x03\r\n\x02001,0,0,0,1,2,6,0,00000000,1,000\x03\r\n\x021234,0\x03\r\n", min(length, 9999))
	case "~HI":
		return fmt.Appendf(nil, "\x02ZPL Emulator,V%s,%d,8192KB\x03\r\n", AppVersion, st.DPMM)
	}
	return nil
}

func (s *EmulatorService) worker() {
	for q := range s.queue {
		j := s.process(q.data, q.source)
		if q.done != nil {
			q.done <- j
		}
	}
}

// PrintZPL runs ZPL typed in the app through the same pipeline as a job
// from the network. With previewOnly it is shown but never forwarded.
func (s *EmulatorService) PrintZPL(zpl string, previewOnly bool) Job {
	done := make(chan Job, 1)
	src := sourceEditor
	if previewOnly {
		src = sourcePreview
	}
	s.queue <- queuedJob{data: []byte(zpl), source: src, done: done}
	return <-done
}

func (s *EmulatorService) process(data []byte, source string) Job {
	st := s.Settings()
	previewOnly := source == sourcePreview
	mode := st.Mode
	if previewOnly {
		mode = ModePreview
	}
	j := Job{
		ID:         s.nextJob.Add(1),
		Received:   time.Now().UnixMilli(),
		Source:     source,
		Bytes:      len(data),
		Mode:       mode,
		Deliveries: []Delivery{},
	}

	targets, terr := s.targets(mode, st)
	showPreview := mode == ModePreview || st.PreviewWhileForwarding
	needRender := showPreview
	for _, p := range targets {
		needRender = needRender || p.PrinterType == PrinterTypeIPP
	}

	var labels []RenderedLabel
	var warns []string
	var rerr error
	if needRender {
		t0 := time.Now()
		labels, warns, rerr = s.renderer.Render(data, st)
		j.RenderMs = float64(time.Since(t0).Microseconds()) / 1000
		j.Labels = len(labels)
	}
	s.history.AddJob(j, data)

	if showPreview && len(labels) > 0 {
		s.publishLabels(j, labels, warns, st)
	}

	j.Deliveries = s.deliver(targets, data, labels, rerr, st)

	switch {
	case terr != nil:
		j.Error = terr.Error()
	case rerr != nil:
		j.Error = "Render failed: " + rerr.Error()
	}
	s.history.UpdateJob(j)
	s.recordStats(j)
	s.emit(EventJobDone, j)
	return j
}

func (s *EmulatorService) targets(mode Mode, st Settings) ([]Printer, error) {
	switch mode {
	case ModeForward:
		if st.ForwardPrinterID == 0 {
			return nil, errors.New("Forward mode is on but no printer is selected.")
		}
		p, err := s.store.Printer(st.ForwardPrinterID)
		if err != nil {
			return nil, err
		}
		if isSelf(p, st) {
			return nil, fmt.Errorf("%s is this emulator (port %d); forwarding to it would loop forever.", p.PrinterName, st.Port)
		}
		return []Printer{p}, nil
	case ModeRelay:
		if st.RelayGroupID == 0 {
			return nil, errors.New("Relay mode is on but no relay group is selected.")
		}
		g, err := s.store.RelayGroup(st.RelayGroupID)
		if err != nil {
			return nil, err
		}
		var out []Printer
		for _, id := range g.PrinterIDs {
			if p, err := s.store.Printer(id); err == nil {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("Relay group %q has no printers.", g.Name)
		}
		return out, nil
	}
	return nil, nil
}

// isSelf reports whether a printer is the emulator's own port, which would
// send every job back to itself.
func isSelf(p Printer, st Settings) bool {
	if p.PrinterType != PrinterTypeZebra || p.PrinterPort != st.Port {
		return false
	}
	switch strings.ToLower(p.IPAddress) {
	case "localhost", "0.0.0.0", "::1", "[::1]":
		return true
	}
	if ip := net.ParseIP(p.IPAddress); ip != nil && ip.IsLoopback() {
		return true
	}
	if st.ListenAll {
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if n, ok := a.(*net.IPNet); ok && n.IP.String() == p.IPAddress {
					return true
				}
			}
		}
	}
	return false
}

// deliver sends a job to every target in parallel.
func (s *EmulatorService) deliver(targets []Printer, data []byte, labels []RenderedLabel, rerr error, st Settings) []Delivery {
	out := make([]Delivery, len(targets))
	var wg sync.WaitGroup
	for i, p := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t0 := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 2*sendTimeout)
			defer cancel()
			var err error
			switch {
			case isSelf(p, st):
				err = errors.New("this is the emulator's own port; skipped so the job does not loop")
			case p.PrinterType == PrinterTypeZebra:
				err = SendRaw(ctx, p, data)
			case rerr != nil:
				err = fmt.Errorf("could not render the label: %w", rerr)
			case len(labels) == 0:
				err = errors.New("nothing to print")
			default:
				err = SendIPP(ctx, p, labels, st.DPMM)
			}
			d := Delivery{PrinterID: p.PrinterID, PrinterName: p.PrinterName, OK: err == nil, Millis: time.Since(t0).Milliseconds()}
			if err != nil {
				d.Error = err.Error()
			}
			out[i] = d
		}()
	}
	wg.Wait()
	return out
}

func (s *EmulatorService) publishLabels(j Job, labels []RenderedLabel, warns []string, st Settings) {
	metas := make([]Label, len(labels))
	stored := make([]storedLabel, len(labels))
	for i, l := range labels {
		b := l.Image.Bounds()
		m := Label{
			ID:       s.nextLabel.Add(1),
			JobID:    j.ID,
			Index:    i,
			Received: j.Received,
			Width:    b.Dx(),
			Height:   b.Dy(),
			DPMM:     st.DPMM,
			Source:   j.Source,
			Bytes:    len(l.PNG),
			Warnings: warns,
		}
		m.URL = labelURL(m.ID)
		metas[i] = m
		stored[i] = storedLabel{meta: m, png: l.PNG}
	}
	s.history.AddLabels(stored)
	s.emit(EventLabelsAdded, metas)
	if st.SaveLabels && st.SaveDir != "" {
		go saveToDisk(st, time.UnixMilli(j.Received), stored, j.ID, s.history)
	}
}

func saveToDisk(st Settings, at time.Time, labels []storedLabel, jobID int64, h *History) {
	stamp := at.Format("2006-01-02_15-04-05.000")
	for _, l := range labels {
		name := fmt.Sprintf("label_%s_%d-%d.png", stamp, jobID, l.meta.Index+1)
		os.WriteFile(filepath.Join(st.SaveDir, name), l.png, 0o644)
	}
	if st.SaveZPL {
		if z, ok := h.ZPL(jobID); ok {
			os.WriteFile(filepath.Join(st.SaveDir, fmt.Sprintf("label_%s_%d.zpl", stamp, jobID)), z, 0o644)
		}
	}
}

func (s *EmulatorService) recordStats(j Job) {
	s.stats.Lock()
	defer s.stats.Unlock()
	s.stats.Jobs++
	s.stats.Labels += int64(j.Labels)
	failed := j.Error != ""
	for _, d := range j.Deliveries {
		failed = failed || !d.OK
	}
	if failed {
		s.stats.Failures++
	}
	if j.RenderMs > 0 {
		s.stats.renderTotal += j.RenderMs
		s.stats.rendered++
		s.stats.AvgRenderMs = s.stats.renderTotal / float64(s.stats.rendered)
	}
}

func (s *EmulatorService) Stats() Stats {
	s.stats.Lock()
	defer s.stats.Unlock()
	return s.stats.Stats
}

// ---- label history -----------------------------------------------------

func (s *EmulatorService) Labels() []Label { return s.history.Labels() }
func (s *EmulatorService) Jobs() []Job     { return s.history.Jobs() }

func (s *EmulatorService) RemoveLabel(id int64) { s.history.Remove(id) }

func (s *EmulatorService) ClearLabels() {
	s.history.Clear()
	s.emit(EventLabelsCleared, nil)
}

// JobZPL returns the raw data of a job, if it is still in the history.
func (s *EmulatorService) JobZPL(jobID int64) (string, error) {
	b, ok := s.history.ZPL(jobID)
	if !ok {
		return "", errors.New("this job is no longer in the history")
	}
	return string(b), nil
}

// SaveLabelAs asks where to save one label as a PNG.
func (s *EmulatorService) SaveLabelAs(id int64) (string, error) {
	png, ok := s.history.PNG(id)
	if !ok {
		return "", errors.New("this label is no longer in the history")
	}
	dlg := application.Get().Dialog.SaveFile().
		SetFilename(fmt.Sprintf("label-%d.png", id)).
		CanCreateDirectories(true)
	if dir := s.Settings().SaveDir; dir != "" {
		dlg.SetDirectory(dir)
	}
	path, err := dlg.PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	if filepath.Ext(path) == "" {
		path += ".png"
	}
	return path, os.WriteFile(path, png, 0o644)
}

// ResendJob sends a job from the history to a network printer.
func (s *EmulatorService) ResendJob(jobID int64, printerID int) (Delivery, error) {
	data, ok := s.history.ZPL(jobID)
	if !ok {
		return Delivery{}, errors.New("this job is no longer in the history")
	}
	p, err := s.store.Printer(printerID)
	if err != nil {
		return Delivery{}, err
	}
	st := s.Settings()
	var labels []RenderedLabel
	var rerr error
	if p.PrinterType == PrinterTypeIPP {
		labels, _, rerr = s.renderer.Render(data, st)
	}
	d := s.deliver([]Printer{p}, data, labels, rerr, st)[0]
	if !d.OK {
		return d, errors.New(d.Error)
	}
	return d, nil
}

// ResetPrinterMemory clears graphics, formats and fonts stored by earlier
// jobs, like power-cycling a printer.
func (s *EmulatorService) ResetPrinterMemory() { s.renderer.Reset() }

// ---- network printers & relay groups -----------------------------------

func (s *EmulatorService) Printers() ([]Printer, error) { return s.store.Printers() }

func (s *EmulatorService) SavePrinter(p Printer) (Printer, error) { return s.store.SavePrinter(p) }

func (s *EmulatorService) DeletePrinter(id int) error {
	if err := s.store.DeletePrinter(id); err != nil {
		return err
	}
	st := s.Settings()
	if st.ForwardPrinterID == id {
		st.ForwardPrinterID = 0
		_, err := s.UpdateSettings(st)
		return err
	}
	return nil
}

// TestPrinter checks that a printer's port is reachable.
func (s *EmulatorService) TestPrinter(p Printer) (int64, error) {
	p, err := p.Normalize()
	if err != nil {
		return 0, err
	}
	t0 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	if err := CheckPrinter(ctx, p); err != nil {
		return 0, err
	}
	return time.Since(t0).Milliseconds(), nil
}

func (s *EmulatorService) RelayGroups() ([]RelayGroup, error) { return s.store.RelayGroups() }

func (s *EmulatorService) SaveRelayGroup(g RelayGroup) (RelayGroup, error) {
	if len(g.PrinterIDs) == 0 {
		return g, errors.New("pick at least one printer")
	}
	return s.store.SaveRelayGroup(g)
}

func (s *EmulatorService) DeleteRelayGroup(id int) error {
	if err := s.store.DeleteRelayGroup(id); err != nil {
		return err
	}
	st := s.Settings()
	if st.RelayGroupID == id {
		st.RelayGroupID = 0
		_, err := s.UpdateSettings(st)
		return err
	}
	return nil
}

// jobSink connects the print server to the service without exposing these
// methods to the frontend bindings.
type jobSink struct{ s *EmulatorService }

func (j jobSink) HandleJob(data []byte, source string) { j.s.handleJob(data, source) }
func (j jobSink) HostResponse(q string) []byte         { return j.s.hostResponse(q) }

// EditorPreview is the result of rendering ZPL typed in the editor.
type EditorPreview struct {
	Images   []string `json:"images"` // PNG data URLs
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Warnings []string `json:"warnings"`
	Millis   float64  `json:"millis"`
}

// PreviewZPL renders ZPL for the editor's live preview with a fresh
// printer, so it neither enters the label history nor changes the
// emulator's printer memory.
func (s *EmulatorService) PreviewZPL(zpl string) (EditorPreview, error) {
	var r Renderer
	t0 := time.Now()
	labels, warns, err := r.Render([]byte(zpl), s.Settings())
	out := EditorPreview{Images: []string{}, Warnings: warns, Millis: float64(time.Since(t0).Microseconds()) / 1000}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	for i, l := range labels {
		if i == 20 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("Showing the first 20 of %d labels.", len(labels)))
			break
		}
		out.Images = append(out.Images, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(l.PNG))
		b := l.Image.Bounds()
		out.Width, out.Height = b.Dx(), b.Dy()
	}
	return out, err
}
