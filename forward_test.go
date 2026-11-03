package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"testing"
)

func TestLabelPDF(t *testing.T) {
	var r Renderer
	labels, _, err := r.Render([]byte("^XA^FO0,0^GB100,100,100^FS^XZ"), Settings{LabelWidth: 2, LabelHeight: 1, DPMM: 8})
	if err != nil || len(labels) != 1 {
		t.Fatal(labels, err)
	}
	pdf := LabelPDF(labels[0].Image, 8)

	// 2x1 in = 144x72 pt (406x203 dots at 8 dpmm).
	if !regexp.MustCompile(`/MediaBox \[0 0 143\.\d+ 71\.\d+\]`).Match(pdf) {
		t.Errorf("MediaBox wrong:\n%s", pdf[:300])
	}
	// xref offsets point at their objects.
	xref := bytes.LastIndex(pdf, []byte("\nxref\n")) + 1
	m := regexp.MustCompile(`(\d{10}) 00000 n`).FindAllSubmatch(pdf[xref:], -1)
	if len(m) != 5 {
		t.Fatalf("%d xref entries", len(m))
	}
	for i, e := range m {
		off, _ := strconv.Atoi(string(e[1]))
		if want := fmt.Sprintf("%d 0 obj", i+1); !bytes.HasPrefix(pdf[off:], []byte(want)) {
			t.Errorf("xref %d points at %q", i+1, pdf[off:off+10])
		}
	}
	if !bytes.Contains(pdf, []byte(fmt.Sprintf("startxref\n%d\n", xref))) {
		t.Error("startxref wrong")
	}
	// The image stream decodes to the packed bitmap: the box is black (0
	// bits) in the top-left corner and the rest is white.
	i := bytes.Index(pdf, []byte("/Subtype /Image"))
	s := bytes.Index(pdf[i:], []byte("stream\n")) + i + 7
	zr, err := zlib.NewReader(bytes.NewReader(pdf[s:]))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(zr)
	rowLen := (406 + 7) / 8
	if len(raw) != rowLen*203 {
		t.Fatalf("bitmap %d bytes", len(raw))
	}
	if raw[0] != 0x00 || raw[rowLen-2] != 0xFF || raw[len(raw)-2] != 0xFF {
		t.Errorf("bitmap pixels wrong: %x %x", raw[0], raw[rowLen-2])
	}
}
