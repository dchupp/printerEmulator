package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const (
	// readIdle is how long a connection may be silent before buffered data
	// that does not end in ^XZ is printed anyway.
	readIdle = time.Second
	// connIdle closes connections that send nothing for this long.
	connIdle = 2 * time.Minute
	// maxJobBytes caps one buffered job; anything larger is printed in parts.
	maxJobBytes = 64 << 20
)

// JobHandler receives every job read from the printer port.
type JobHandler interface {
	// HandleJob is called with a complete job. It must not keep data.
	HandleJob(data []byte, source string)
	// HostResponse returns the reply to a host query such as ~HS, or nil.
	HostResponse(query string) []byte
}

// PrintServer is a raw TCP print server, like port 9100 on a Zebra printer.
type PrintServer struct {
	ln      net.Listener
	handler JobHandler
	quit    chan struct{}
	wg      sync.WaitGroup

	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

func StartPrintServer(addr string, h JobHandler) (*PrintServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &PrintServer{ln: ln, handler: h, quit: make(chan struct{}), conns: map[net.Conn]struct{}{}}
	s.wg.Add(1)
	go s.accept()
	return s, nil
}

func (s *PrintServer) Addr() string { return s.ln.Addr().String() }

// Stop closes the listener and every open connection, then waits briefly
// for in-flight reads to finish.
func (s *PrintServer) Stop() {
	close(s.quit)
	s.ln.Close()
	s.mu.Lock()
	for c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func (s *PrintServer) accept() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		s.mu.Lock()
		s.conns[c] = struct{}{}
		s.mu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.serve(c)
			s.mu.Lock()
			delete(s.conns, c)
			s.mu.Unlock()
			c.Close()
		}()
	}
}

// serve reads one connection. Each complete format (up to the last ^XZ) is
// printed as soon as it arrives, so a sender that keeps the connection open
// still sees its labels immediately; the remainder is printed when the
// sender closes the connection or goes quiet.
func (s *PrintServer) serve(c net.Conn) {
	source := c.RemoteAddr().String()
	var buf []byte
	chunk := make([]byte, 64<<10)
	var silent time.Duration
	flush := func(n int) {
		if n > 0 && len(bytes.TrimSpace(buf[:n])) > 0 {
			s.handler.HandleJob(buf[:n], source)
		}
		buf = append(buf[:0], buf[n:]...)
	}
	for {
		c.SetReadDeadline(time.Now().Add(readIdle))
		n, err := c.Read(chunk)
		if n > 0 {
			silent = 0
			scanFrom := max(0, len(buf)-2) // a query split across reads
			buf = append(buf, chunk[:n]...)
			for _, q := range hostQueries(buf[scanFrom:]) {
				if resp := s.handler.HostResponse(q); resp != nil {
					c.SetWriteDeadline(time.Now().Add(5 * time.Second))
					c.Write(resp)
				}
			}
			if end := lastFormatEnd(buf); end > 0 {
				flush(end)
			} else if len(buf) >= maxJobBytes {
				flush(len(buf))
			}
		}
		if err == nil {
			continue
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			flush(len(buf))
			if silent += readIdle; silent >= connIdle {
				return
			}
			select {
			case <-s.quit:
				return
			default:
			}
			continue
		}
		if !errors.Is(err, io.EOF) {
			select {
			case <-s.quit:
				return // stopping: drop partial data
			default:
			}
		}
		flush(len(buf))
		return
	}
}

// lastFormatEnd returns the offset just past the last ^XZ in b, or 0.
func lastFormatEnd(b []byte) int {
	for i := len(b) - 3; i >= 0; i-- {
		if b[i] == '^' && (b[i+1]|0x20) == 'x' && (b[i+2]|0x20) == 'z' {
			return i + 3
		}
	}
	return 0
}

// hostQueries finds host status commands a driver may wait on.
func hostQueries(b []byte) []string {
	var out []string
	for i := 0; i+2 < len(b); i++ {
		if b[i] != '~' || (b[i+1]|0x20) != 'h' {
			continue
		}
		switch b[i+2] | 0x20 {
		case 's':
			out = append(out, "~HS")
		case 'i':
			out = append(out, "~HI")
		}
	}
	return out
}
