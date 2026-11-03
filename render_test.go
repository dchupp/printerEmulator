package main

import (
	"os"
	"testing"
)

// Zebra drivers wrap a label in setup-only formats (^EG, ^MC, ^CI, ^PO,
// ^LH). A printer feeds nothing for those, so only the real label shows.
func TestSetupOnlyFormatsPrintNothing(t *testing.T) {
	data, err := os.ReadFile("testdata/driver_setup_formats.zpl")
	if err != nil {
		t.Fatal(err)
	}
	var r Renderer
	labels, _, err := r.Render(data, DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 1 {
		t.Fatalf("got %d labels, want 1 (the three setup formats must not print)", len(labels))
	}
	if isBlank(labels[0].Image) {
		t.Fatal("the remaining label is blank")
	}
}

func TestEmptyFormatPrintsNothing(t *testing.T) {
	var r Renderer
	labels, _, err := r.Render([]byte("^XA^XZ^XA^FO10,10^GB50,50,50^FS^XZ^XA^LH0,0^XZ"), DefaultSettings())
	if err != nil || len(labels) != 1 {
		t.Fatalf("got %d labels, %v; want 1", len(labels), err)
	}
}
