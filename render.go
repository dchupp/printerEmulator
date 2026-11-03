package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"sync"

	zpl "github.com/dchupp/DataMagik-ZPL-To-Image"
)

// maxLabelsPerJob stops a runaway ^PQ from flooding the preview.
const maxLabelsPerJob = 500

var errTooManyLabels = fmt.Errorf("job produced more than %d labels; the rest were skipped", maxLabelsPerJob)

// RenderedLabel is one label image of a job.
type RenderedLabel struct {
	Image image.Image
	PNG   []byte
}

// Renderer turns ZPL into label images locally, with no network service.
//
// It keeps one virtual printer, so graphics, formats and fonts downloaded
// by one job (~DG, ^DF, ~DU, …) are available to later jobs, as on a real
// Zebra. The printer is rebuilt when the media settings change.
type Renderer struct {
	mu      sync.Mutex
	printer *zpl.Printer
	key     renderKey
}

type renderKey struct {
	w, h         float64
	dpmm, rot    int
	expandCopies bool
}

func keyOf(s Settings) renderKey {
	return renderKey{s.LabelWidth, s.LabelHeight, s.DPMM, s.Rotation, s.ExpandCopies}
}

func (r *Renderer) printerFor(s Settings) (*zpl.Printer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := keyOf(s)
	if r.printer != nil && r.key == k {
		return r.printer, nil
	}
	p, err := zpl.NewPrinter(zpl.Options{
		DPMM:         s.DPMM,
		LabelWidth:   s.LabelWidth,
		LabelHeight:  s.LabelHeight,
		Rotation:     s.Rotation,
		ExpandCopies: s.ExpandCopies,
	})
	if err != nil {
		return nil, err
	}
	r.printer, r.key = p, k
	return p, nil
}

// Reset forgets everything stored in the virtual printer's memory.
func (r *Renderer) Reset() {
	r.mu.Lock()
	r.printer = nil
	r.mu.Unlock()
}

// Render renders a job and returns its labels as PNGs plus any warnings.
// Labels rendered before an error are still returned.
func (r *Renderer) Render(data []byte, s Settings) ([]RenderedLabel, []string, error) {
	p, err := r.printerFor(s)
	if err != nil {
		return nil, nil, err
	}
	var out []RenderedLabel
	warns, err := p.RenderEach(bytes.NewReader(data), func(img image.Image) error {
		if len(out) >= maxLabelsPerJob {
			return errTooManyLabels
		}
		png, err := zpl.EncodePNG(img)
		if err != nil {
			return err
		}
		out = append(out, RenderedLabel{Image: img, PNG: png})
		return nil
	})
	msgs := make([]string, 0, len(warns))
	for _, w := range warns {
		msgs = append(msgs, w.String())
	}
	if errors.Is(err, errTooManyLabels) {
		msgs = append(msgs, err.Error())
		err = nil
	}
	return out, msgs, err
}
