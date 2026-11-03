package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	dialTimeout = 5 * time.Second
	sendTimeout = 30 * time.Second
)

// SendRaw writes a job unchanged to a printer's raw port (Zebra 9100).
func SendRaw(ctx context.Context, p Printer, data []byte) error {
	d := net.Dialer{Timeout: dialTimeout}
	c, err := d.DialContext(ctx, "tcp", p.Address())
	if err != nil {
		return fmt.Errorf("connect to %s: %w", p.Address(), err)
	}
	defer c.Close()
	c.SetWriteDeadline(time.Now().Add(sendTimeout))
	if _, err := c.Write(data); err != nil {
		return fmt.Errorf("send to %s: %w", p.Address(), err)
	}
	return nil
}

// CheckPrinter reports whether a printer's port accepts connections.
func CheckPrinter(ctx context.Context, p Printer) error {
	d := net.Dialer{Timeout: dialTimeout}
	c, err := d.DialContext(ctx, "tcp", p.Address())
	if err != nil {
		return err
	}
	return c.Close()
}

// --- IPP ------------------------------------------------------------------

const (
	ippPrintJob      = 0x0002
	ippTagOperation  = 0x01
	ippTagJob        = 0x02
	ippTagEnd        = 0x03
	ippTagInteger    = 0x21
	ippTagURI        = 0x45
	ippTagName       = 0x42
	ippTagCharset    = 0x47
	ippTagLanguage   = 0x48
	ippTagMimeType   = 0x49
	ippContentType   = "application/ipp"
	ippUserName      = "ZPLPrinterEmulator"
	ippStatusOKLimit = 0x00FF // successful-ok* codes
)

var ippRequestID atomic.Int32

func ippAttr(b *bytes.Buffer, tag byte, name string, value []byte) {
	b.WriteByte(tag)
	binary.Write(b, binary.BigEndian, uint16(len(name)))
	b.WriteString(name)
	binary.Write(b, binary.BigEndian, uint16(len(value)))
	b.Write(value)
}

func ippPrintJobRequest(p Printer, docFormat, jobName string) []byte {
	var b bytes.Buffer
	b.Write([]byte{2, 0}) // IPP 2.0
	binary.Write(&b, binary.BigEndian, uint16(ippPrintJob))
	binary.Write(&b, binary.BigEndian, ippRequestID.Add(1))
	b.WriteByte(ippTagOperation)
	ippAttr(&b, ippTagCharset, "attributes-charset", []byte("utf-8"))
	ippAttr(&b, ippTagLanguage, "attributes-natural-language", []byte("en-us"))
	ippAttr(&b, ippTagURI, "printer-uri", []byte(fmt.Sprintf("ipp://%s%s", p.Address(), p.IPPEndpoint)))
	ippAttr(&b, ippTagName, "requesting-user-name", []byte(ippUserName))
	ippAttr(&b, ippTagName, "job-name", []byte(jobName))
	ippAttr(&b, ippTagMimeType, "document-format", []byte(docFormat))
	b.WriteByte(ippTagJob)
	ippAttr(&b, ippTagInteger, "copies", []byte{0, 0, 0, 1})
	b.WriteByte(ippTagEnd)
	return b.Bytes()
}

var ippClient = &http.Client{
	Timeout: sendTimeout,
	Transport: &http.Transport{
		// Network printers almost always use self-signed certificates.
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		DialContext:         (&net.Dialer{Timeout: dialTimeout}).DialContext,
		MaxIdleConnsPerHost: 2,
	},
}

func sendIPP(ctx context.Context, p Printer, docFormat, jobName string, doc []byte) error {
	scheme := "http"
	if p.UseTLS {
		scheme = "https"
	}
	body := append(ippPrintJobRequest(p, docFormat, jobName), doc...)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s://%s%s", scheme, p.Address(), p.IPPEndpoint), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ippContentType)
	resp, err := ippClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IPP: HTTP %s", resp.Status)
	}
	head := make([]byte, 8)
	if _, err := io.ReadFull(resp.Body, head); err != nil {
		return fmt.Errorf("IPP: short response: %w", err)
	}
	if status := binary.BigEndian.Uint16(head[2:4]); status > ippStatusOKLimit {
		return fmt.Errorf("IPP: printer returned status 0x%04x", status)
	}
	return nil
}

// SendIPP prints rendered labels on an IPP printer, one job per label, as
// PDF sized to the label, falling back to PNG for printers without PDF.
func SendIPP(ctx context.Context, p Printer, labels []RenderedLabel, dpmm int) error {
	stamp := time.Now().Format("20060102-150405")
	for i, l := range labels {
		name := fmt.Sprintf("ZPL-Label-%s-%d", stamp, i+1)
		err := sendIPP(ctx, p, "application/pdf", name, LabelPDF(l.Image, dpmm))
		if err != nil {
			if perr := sendIPP(ctx, p, "image/png", name, l.PNG); perr != nil {
				return fmt.Errorf("%v (PNG fallback: %v)", err, perr)
			}
		}
	}
	return nil
}

// --- PDF ------------------------------------------------------------------

// LabelPDF wraps a label image in a one-page PDF whose page is exactly the
// physical label size at the given density. The image is stored as a 1-bit
// Flate-compressed DeviceGray bitmap, so the file stays small and sharp.
func LabelPDF(img image.Image, dpmm int) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	rowLen := (w + 7) / 8
	packed := make([]byte, rowLen*h)
	pal, isPal := img.(image.PalettedImage)
	for y := 0; y < h; y++ {
		row := packed[y*rowLen : (y+1)*rowLen]
		for x := 0; x < w; x++ {
			var white bool
			if isPal {
				white = pal.ColorIndexAt(b.Min.X+x, b.Min.Y+y) == 0
			} else {
				white = color.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.Gray).Y >= 128
			}
			if white {
				row[x>>3] |= 0x80 >> (x & 7)
			}
		}
	}
	var z bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&z, zlib.BestSpeed)
	zw.Write(packed)
	zw.Close()

	if dpmm <= 0 {
		dpmm = 8
	}
	ptPerDot := 72 / (25.4 * float64(dpmm))
	pw, ph := float64(w)*ptPerDot, float64(h)*ptPerDot
	content := fmt.Sprintf("q %.3f 0 0 %.3f 0 0 cm /Im0 Do Q", pw, ph)

	var out bytes.Buffer
	var offsets []int
	obj := func(body string, stream []byte) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s", len(offsets), body)
		if stream != nil {
			out.WriteString("\nstream\n")
			out.Write(stream)
			out.WriteString("\nendstream")
		}
		out.WriteString("\nendobj\n")
	}
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	obj("<< /Type /Catalog /Pages 2 0 R >>", nil)
	obj("<< /Type /Pages /Kids [3 0 R] /Count 1 >>", nil)
	obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.3f %.3f] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>", pw, ph), nil)
	obj(fmt.Sprintf("<< /Length %d >>", len(content)), []byte(content))
	obj(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 1 /Filter /FlateDecode /Length %d >>", w, h, z.Len()), z.Bytes())
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return out.Bytes()
}
