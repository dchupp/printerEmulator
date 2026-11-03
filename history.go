package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// labelPathPrefix is where the asset server serves label PNGs.
const labelPathPrefix = "/labels/"

type storedLabel struct {
	meta Label
	png  []byte
}

// History keeps the most recent labels and jobs in memory.
//
// The UI receives only metadata through events and loads each PNG from
// /labels/{id}.png, so large images never pass through the JS bridge.
type History struct {
	mu     sync.RWMutex
	limit  int
	labels []storedLabel // oldest first
	byID   map[int64]int // id -> index in labels, rebuilt on trim
	jobs   []Job         // oldest first
	zpl    map[int64][]byte
}

const (
	jobHistory     = 200
	maxStoredZPL   = 4 << 20
	pngCacheHeader = "private, max-age=31536000, immutable"
)

func NewHistory(limit int) *History {
	return &History{limit: limit, byID: map[int64]int{}, zpl: map[int64][]byte{}}
}

func (h *History) SetLimit(n int) {
	h.mu.Lock()
	h.limit = n
	h.trim()
	h.mu.Unlock()
}

// AddJob records a job and the ZPL it carried.
func (h *History) AddJob(j Job, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.jobs = append(h.jobs, j)
	if len(data) <= maxStoredZPL {
		h.zpl[j.ID] = data
	}
	if n := len(h.jobs) - jobHistory; n > 0 {
		for _, old := range h.jobs[:n] {
			delete(h.zpl, old.ID)
		}
		h.jobs = append(h.jobs[:0:0], h.jobs[n:]...)
	}
}

// UpdateJob replaces a recorded job with the same ID.
func (h *History) UpdateJob(j Job) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.jobs) - 1; i >= 0; i-- {
		if h.jobs[i].ID == j.ID {
			h.jobs[i] = j
			return
		}
	}
}

func (h *History) AddLabels(ls []storedLabel) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, l := range ls {
		h.byID[l.meta.ID] = len(h.labels)
		h.labels = append(h.labels, l)
	}
	h.trim()
}

func (h *History) trim() {
	n := len(h.labels) - h.limit
	if n <= 0 {
		return
	}
	h.labels = append(h.labels[:0:0], h.labels[n:]...)
	clear(h.byID)
	for i, l := range h.labels {
		h.byID[l.meta.ID] = i
	}
}

// Labels returns label metadata, newest first.
func (h *History) Labels() []Label {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Label, len(h.labels))
	for i, l := range h.labels {
		out[len(h.labels)-1-i] = l.meta
	}
	return out
}

// Jobs returns recent jobs, newest first.
func (h *History) Jobs() []Job {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Job, len(h.jobs))
	for i, j := range h.jobs {
		out[len(h.jobs)-1-i] = j
	}
	return out
}

func (h *History) PNG(id int64) ([]byte, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	i, ok := h.byID[id]
	if !ok {
		return nil, false
	}
	return h.labels[i].png, true
}

func (h *History) Label(id int64) (Label, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	i, ok := h.byID[id]
	if !ok {
		return Label{}, false
	}
	return h.labels[i].meta, true
}

func (h *History) ZPL(jobID int64) ([]byte, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	b, ok := h.zpl[jobID]
	return b, ok
}

func (h *History) Remove(id int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	i, ok := h.byID[id]
	if !ok {
		return
	}
	h.labels = append(h.labels[:i], h.labels[i+1:]...)
	clear(h.byID)
	for i, l := range h.labels {
		h.byID[l.meta.ID] = i
	}
}

func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.labels = nil
	clear(h.byID)
	h.jobs = nil
	clear(h.zpl)
}

// Middleware serves /labels/{id}.png from the history and passes every
// other request to the frontend assets.
func (h *History) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, labelPathPrefix)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		id, err := strconv.ParseInt(strings.TrimSuffix(rest, ".png"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		png, ok := h.PNG(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(png)))
		w.Header().Set("Cache-Control", pngCacheHeader)
		w.Write(png)
	})
}

func labelURL(id int64) string { return labelPathPrefix + strconv.FormatInt(id, 10) + ".png" }
