// Package store keeps a bounded in-memory history of samples so the dashboard
// can draw sparklines without an external database.
package store

import (
	"sync"

	"srvmon/internal/model"
)

// Ring is a fixed-capacity, concurrency-safe ring buffer of samples.
type Ring struct {
	mu   sync.RWMutex
	buf  []model.Sample
	next int
	full bool
}

// New returns a ring holding at most capacity samples (minimum 1).
func New(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{buf: make([]model.Sample, capacity)}
}

// Add stores a sample, evicting the oldest when full.
func (r *Ring) Add(s model.Sample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = s
	r.next = (r.next + 1) % len(r.buf)
	if r.next == 0 {
		r.full = true
	}
}

// Latest returns the most recent sample and whether one exists.
func (r *Ring) Latest() (model.Sample, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.full && r.next == 0 {
		return model.Sample{}, false
	}
	idx := (r.next - 1 + len(r.buf)) % len(r.buf)
	return r.buf[idx], true
}

// History returns all stored samples in chronological order (oldest first).
func (r *Ring) History() []model.Sample {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := r.next
	if r.full {
		out := make([]model.Sample, 0, len(r.buf))
		out = append(out, r.buf[r.next:]...)
		out = append(out, r.buf[:r.next]...)
		return out
	}
	out := make([]model.Sample, n)
	copy(out, r.buf[:n])
	return out
}
