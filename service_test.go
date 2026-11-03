package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const sampleLabel = "^XA^FO50,50^A0N,50^FDHello^FS^FO50,120^BY2^BCN,80,Y,N,N^FD12345^FS^XZ"

type eventLog struct {
	mu   sync.Mutex
	got  map[string][]any
	cond *sync.Cond
}

func newTestService(t *testing.T) (*EmulatorService, *eventLog) {
	t.Helper()
	st, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := NewEmulatorService(st)
	ev := &eventLog{got: map[string][]any{}}
	ev.cond = sync.NewCond(&ev.mu)
	s.emit = func(name string, data any) {
		ev.mu.Lock()
		ev.got[name] = append(ev.got[name], data)
		ev.cond.Broadcast()
		ev.mu.Unlock()
	}
	cfg := s.Settings()
	cfg.Port = 0 // any free port
	s.mu.Lock()
	s.settings = cfg
	s.mu.Unlock()
	t.Cleanup(func() { s.Stop() })
	return s, ev
}

// wait blocks until n events of the given name arrived.
func (e *eventLog) wait(t *testing.T, name string, n int) []any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	go func() { time.Sleep(5 * time.Second); e.cond.Broadcast() }()
	e.mu.Lock()
	defer e.mu.Unlock()
	for len(e.got[name]) < n {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d %q events, got %d", n, name, len(e.got[name]))
		}
		e.cond.Wait()
	}
	return append([]any(nil), e.got[name]...)
}

func send(t *testing.T, addr, data string, closeAfter bool) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write([]byte(data)); err != nil {
		t.Fatal(err)
	}
	if closeAfter {
		c.Close()
	}
	return c
}

func TestPreviewOverTCP(t *testing.T) {
	s, ev := newTestService(t)
	st, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Running || !strings.HasPrefix(st.Address, "127.0.0.1:") {
		t.Fatalf("status = %+v", st)
	}
	send(t, st.Address, sampleLabel+sampleLabel, true)

	added := ev.wait(t, EventLabelsAdded, 1)[0].([]Label)
	if len(added) != 2 {
		t.Fatalf("got %d labels, want 2", len(added))
	}
	l := added[0]
	if l.Width != 813 || l.Height != 1219 { // 4x6 in at 8 dpmm, rounded
		t.Errorf("label size %dx%d", l.Width, l.Height)
	}
	// The PNG is served by the asset middleware.
	rec := httptest.NewRecorder()
	s.history.Middleware(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", l.URL, nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("GET %s = %d", l.URL, rec.Code)
	}
	if _, err := png.Decode(rec.Body); err != nil {
		t.Fatal(err)
	}
	job := ev.wait(t, EventJobDone, 1)[0].(Job)
	if job.Labels != 2 || job.Error != "" {
		t.Errorf("job = %+v", job)
	}
	if z, _ := s.JobZPL(job.ID); z != sampleLabel+sampleLabel {
		t.Errorf("stored ZPL = %q", z)
	}
}

// A sender that keeps the connection open sees each label as soon as its
// ^XZ arrives, without waiting for the connection to close.
func TestOpenConnectionPrintsEachFormat(t *testing.T) {
	s, ev := newTestService(t)
	st, _ := s.Start()
	c := send(t, st.Address, sampleLabel, false)
	defer c.Close()
	start := time.Now()
	ev.wait(t, EventLabelsAdded, 1)
	if d := time.Since(start); d > readIdle {
		t.Errorf("first label took %v", d)
	}
	c.Write([]byte(sampleLabel[:20]))
	time.Sleep(50 * time.Millisecond)
	c.Write([]byte(sampleLabel[20:]))
	ev.wait(t, EventLabelsAdded, 2)
}

func TestHostStatus(t *testing.T) {
	s, _ := newTestService(t)
	st, _ := s.Start()
	c := send(t, st.Address, "~H", false)
	defer c.Close()
	time.Sleep(20 * time.Millisecond)
	c.Write([]byte("S"))
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 256)
	n, err := io.ReadAtLeast(c, buf, 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(buf[:n]); !strings.HasPrefix(got, "\x02030,0,0,1219,") || strings.Count(got, "\x03") != 3 {
		t.Errorf("~HS reply = %q", got)
	}
}

func TestPortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	s, _ := newTestService(t)
	s.settings.Port = ln.Addr().(*net.TCPAddr).Port
	if _, err := s.Start(); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("err = %v", err)
	}
	if s.Status().Running {
		t.Fatal("running after failed start")
	}
}

// fakeZebra records everything sent to it.
func fakeZebra(t *testing.T) (Printer, func() string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var mu sync.Mutex
	var got bytes.Buffer
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			b, _ := io.ReadAll(c)
			mu.Lock()
			got.Write(b)
			mu.Unlock()
			c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	return Printer{PrinterName: "Fake Zebra", IPAddress: "127.0.0.1", PrinterPort: port, PrinterType: PrinterTypeZebra},
		func() string {
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				mu.Lock()
				s := got.String()
				mu.Unlock()
				if s != "" {
					return s
				}
				time.Sleep(10 * time.Millisecond)
			}
			return ""
		}
}

// fakeIPP accepts IPP Print-Job requests and records the document formats.
func fakeIPP(t *testing.T) (Printer, func() []string) {
	var mu sync.Mutex
	var formats []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		format := "?"
		if i := bytes.Index(body, []byte("document-format")); i >= 0 {
			n := int(binary.BigEndian.Uint16(body[i+15:]))
			format = string(body[i+17 : i+17+n])
			doc := body[bytes.IndexByte(body[i:], ippTagEnd)+i+1:]
			if format == "application/pdf" && !bytes.HasPrefix(doc, []byte("%PDF-1.4")) {
				format = "bad pdf"
			}
		}
		mu.Lock()
		formats = append(formats, format)
		mu.Unlock()
		w.Header().Set("Content-Type", ippContentType)
		w.Write([]byte{2, 0, 0, 0, 0, 0, 0, 1, ippTagEnd})
	}))
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	var p int
	for _, ch := range port {
		p = p*10 + int(ch-'0')
	}
	return Printer{PrinterName: "Fake IPP", IPAddress: host, PrinterPort: p, PrinterType: PrinterTypeIPP},
		func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), formats...) }
}

func TestRelayToZebraAndIPP(t *testing.T) {
	s, _ := newTestService(t)
	z, zebraGot := fakeZebra(t)
	ipp, ippGot := fakeIPP(t)
	z, err := s.SavePrinter(z)
	if err != nil {
		t.Fatal(err)
	}
	ipp, err = s.SavePrinter(ipp)
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.SaveRelayGroup(RelayGroup{Name: "Both", PrinterIDs: []int{z.PrinterID, ipp.PrinterID}})
	if err != nil {
		t.Fatal(err)
	}
	cfg := s.Settings()
	cfg.Mode, cfg.RelayGroupID = ModeRelay, g.GroupID
	if _, err := s.UpdateSettings(cfg); err != nil {
		t.Fatal(err)
	}

	job := s.PrintZPL(sampleLabel, false)
	if job.Error != "" || len(job.Deliveries) != 2 {
		t.Fatalf("job = %+v", job)
	}
	for _, d := range job.Deliveries {
		if !d.OK {
			t.Errorf("delivery to %s failed: %s", d.PrinterName, d.Error)
		}
	}
	if got := zebraGot(); got != sampleLabel {
		t.Errorf("Zebra got %q", got)
	}
	if got := ippGot(); len(got) != 1 || got[0] != "application/pdf" {
		t.Errorf("IPP got %v", got)
	}
	if n := len(s.Labels()); n != 0 {
		t.Errorf("relay without preview stored %d labels", n)
	}

	// Deleting a printer removes it from the group.
	if err := s.DeletePrinter(ipp.PrinterID); err != nil {
		t.Fatal(err)
	}
	g, _ = s.store.RelayGroup(g.GroupID)
	if len(g.PrinterIDs) != 1 || g.PrinterIDs[0] != z.PrinterID {
		t.Errorf("group after delete = %+v", g)
	}
}

func TestForwardFailureIsReported(t *testing.T) {
	s, _ := newTestService(t)
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // nothing listens here now
	p, _ := s.SavePrinter(Printer{PrinterName: "Gone", IPAddress: "127.0.0.1", PrinterPort: port, PrinterType: PrinterTypeZebra})
	cfg := s.Settings()
	cfg.Mode, cfg.ForwardPrinterID, cfg.PreviewWhileForwarding = ModeForward, p.PrinterID, true
	s.UpdateSettings(cfg)

	job := s.PrintZPL(sampleLabel, false)
	if len(job.Deliveries) != 1 || job.Deliveries[0].OK {
		t.Fatalf("job = %+v", job)
	}
	if len(s.Labels()) != 1 {
		t.Error("preview while forwarding stored no label")
	}
	if s.Stats().Failures != 1 {
		t.Errorf("stats = %+v", s.Stats())
	}
	// The editor's preview button never forwards.
	if job := s.PrintZPL(sampleLabel, true); len(job.Deliveries) != 0 {
		t.Errorf("preview job delivered: %+v", job)
	}
}

func TestPrinterMemoryPersistsAcrossJobs(t *testing.T) {
	s, _ := newTestService(t)
	// Store a format in one job and recall it in the next, as WMS systems do.
	s.PrintZPL("^XA^DFR:SHIP.ZPL^FS^FO50,50^A0N,40^FN1^FS^XZ", true)
	job := s.PrintZPL("^XA^XFR:SHIP.ZPL^FS^FN1^FDRecalled^FS^XZ", true)
	if job.Labels != 1 {
		t.Fatalf("recalled format printed %d labels", job.Labels)
	}
}

func TestHistoryLimit(t *testing.T) {
	s, _ := newTestService(t)
	cfg := s.Settings()
	cfg.HistoryLimit = 3
	s.UpdateSettings(cfg)
	for i := 0; i < 5; i++ {
		s.PrintZPL(sampleLabel, true)
	}
	ls := s.Labels()
	if len(ls) != 3 || ls[0].ID != 5 || ls[2].ID != 3 {
		t.Fatalf("labels = %+v", ls)
	}
	if _, ok := s.history.PNG(1); ok {
		t.Error("trimmed label still served")
	}
	s.RemoveLabel(4)
	if ls := s.Labels(); len(ls) != 2 || ls[1].ID != 3 {
		t.Fatalf("after remove = %+v", ls)
	}
	s.ClearLabels()
	if len(s.Labels()) != 0 {
		t.Fatal("clear left labels")
	}
}

func TestSaveToDisk(t *testing.T) {
	s, _ := newTestService(t)
	dir := t.TempDir()
	cfg := s.Settings()
	cfg.SaveLabels, cfg.SaveZPL, cfg.SaveDir = true, true, dir
	s.UpdateSettings(cfg)
	s.PrintZPL(sampleLabel, true)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		png, _ := filepath.Glob(filepath.Join(dir, "*.png"))
		zpl, _ := filepath.Glob(filepath.Join(dir, "*.zpl"))
		if len(png) == 1 && len(zpl) == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("label files were not written")
}

func TestLegacySettingsImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE settings (settingID INTEGER PRIMARY KEY, printWidth REAL, printHeight REAL, printRotation REAL, printerPort REAL, printPath TEXT, printerDPI_value INTEGER, printerDPI_desc TEXT, defaultPrinter INTEGER, autoStartServer INTEGER DEFAULT 0)`,
		`INSERT INTO settings VALUES (1, 2, 3, 90, 9101, '/tmp/labels', 12, '12 dpmm', 0, 1)`,
		`CREATE TABLE relay_groups (groupID INTEGER PRIMARY KEY AUTOINCREMENT, printerIDs TEXT NOT NULL)`,
		`INSERT INTO relay_groups (printerIDs) VALUES ('[1,2]')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	got := st.LoadSettings()
	if got.LabelWidth != 2 || got.LabelHeight != 3 || got.Rotation != 90 || got.Port != 9101 ||
		got.SaveDir != "/tmp/labels" || got.DPMM != 12 || !got.AutoStartServer || got.Mode != ModePreview {
		t.Errorf("imported %+v", got)
	}
	groups, err := st.RelayGroups()
	if err != nil || len(groups) != 1 || groups[0].Name != "Group 1" || len(groups[0].PrinterIDs) != 2 {
		t.Errorf("groups = %+v, %v", groups, err)
	}
}

func TestPrinterValidation(t *testing.T) {
	for _, p := range []Printer{
		{PrinterName: "", IPAddress: "10.0.0.1", PrinterType: PrinterTypeZebra},
		{PrinterName: "x", IPAddress: "", PrinterType: PrinterTypeZebra},
		{PrinterName: "x", IPAddress: "10.0.0.1", PrinterType: "Laser"},
		{PrinterName: "x", IPAddress: "10.0.0.1", PrinterType: PrinterTypeZebra, PrinterPort: 70000},
	} {
		if _, err := p.Normalize(); err == nil {
			t.Errorf("%+v accepted", p)
		}
	}
	p, err := Printer{PrinterName: "x", IPAddress: "10.0.0.1", PrinterType: PrinterTypeIPP, IPPEndpoint: "ipp/print"}.Normalize()
	if err != nil || p.PrinterPort != 631 || p.IPPEndpoint != "/ipp/print" {
		t.Errorf("IPP defaults: %+v %v", p, err)
	}
}

func TestLastFormatEnd(t *testing.T) {
	for in, want := range map[string]int{
		"":                 0,
		"^XA^FDx^FS":       0,
		"^XA^XZ":           6,
		"^xa^xz^XA":        6,
		"^XA^XZ\r\n^XA^XZ": 14,
	} {
		if got := lastFormatEnd([]byte(in)); got != want {
			t.Errorf("lastFormatEnd(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestPreviewZPLLeavesNoTrace(t *testing.T) {
	s, _ := newTestService(t)
	p, err := s.PreviewZPL(sampleLabel)
	if err != nil || len(p.Images) != 1 || !strings.HasPrefix(p.Images[0], "data:image/png;base64,") {
		t.Fatalf("preview = %+v, %v", p, err)
	}
	if len(s.Labels()) != 0 || len(s.Jobs()) != 0 {
		t.Error("preview entered the history")
	}
}

func TestForwardToSelfIsRefused(t *testing.T) {
	s, _ := newTestService(t)
	st, _ := s.Start()
	_, port, _ := net.SplitHostPort(st.Address)
	var n int
	for _, c := range port {
		n = n*10 + int(c-'0')
	}
	cfg := s.Settings()
	cfg.Port = n
	s.mu.Lock()
	s.settings = cfg
	s.mu.Unlock()
	p, _ := s.SavePrinter(Printer{PrinterName: "Me", IPAddress: "127.0.0.1", PrinterPort: n, PrinterType: PrinterTypeZebra})
	cfg.Mode, cfg.ForwardPrinterID = ModeForward, p.PrinterID
	s.mu.Lock()
	s.settings = cfg
	s.mu.Unlock()
	job := s.PrintZPL(sampleLabel, false)
	if !strings.Contains(job.Error, "loop") || len(job.Deliveries) != 0 {
		t.Fatalf("job = %+v", job)
	}
	if s.Stats().Jobs != 1 {
		t.Fatalf("stats = %+v (job looped?)", s.Stats())
	}
}

func TestGraphicOnlyJobIsNotAnError(t *testing.T) {
	s, _ := newTestService(t)
	job := s.PrintZPL("~DGR:BOX.GRF,00004,001,FFFFFFFF\n", true)
	if job.Error != "" || job.Labels != 0 {
		t.Fatalf("job = %+v", job)
	}
	if job := s.PrintZPL("^XA^FO10,10^XGR:BOX.GRF,1,1^FS^XZ", true); job.Labels != 1 || job.Error != "" {
		t.Fatalf("recall job = %+v", job)
	}
}

func TestRelaySkipsSelfVisibly(t *testing.T) {
	s, _ := newTestService(t)
	z, _ := fakeZebra(t)
	z, _ = s.SavePrinter(z)
	self, _ := s.SavePrinter(Printer{PrinterName: "Me", IPAddress: "localhost", PrinterPort: 9100, PrinterType: PrinterTypeZebra})
	g, _ := s.SaveRelayGroup(RelayGroup{PrinterIDs: []int{z.PrinterID, self.PrinterID}})
	cfg := s.Settings()
	cfg.Port, cfg.Mode, cfg.RelayGroupID = 9100, ModeRelay, g.GroupID
	s.UpdateSettings(cfg)
	job := s.PrintZPL(sampleLabel, false)
	if len(job.Deliveries) != 2 || !job.Deliveries[0].OK || job.Deliveries[1].OK || !strings.Contains(job.Deliveries[1].Error, "loop") {
		t.Fatalf("deliveries = %+v", job.Deliveries)
	}
}
