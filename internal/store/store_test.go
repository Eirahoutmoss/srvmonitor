package store

import (
	"testing"
	"time"

	"srvmon/internal/model"
)

func sample(cpu float64) model.Sample {
	return model.Sample{Time: time.Now(), System: model.SystemMetrics{CPUPercent: cpu}}
}

func TestEmpty(t *testing.T) {
	r := New(4)
	if _, ok := r.Latest(); ok {
		t.Fatal("empty ring should report no data")
	}
	if len(r.History()) != 0 {
		t.Fatal("empty history expected")
	}
}

func TestLatestAndOrder(t *testing.T) {
	r := New(3)
	for i := 1; i <= 2; i++ {
		r.Add(sample(float64(i)))
	}
	l, ok := r.Latest()
	if !ok || l.System.CPUPercent != 2 {
		t.Fatalf("latest = %+v ok=%v", l, ok)
	}
	h := r.History()
	if len(h) != 2 || h[0].System.CPUPercent != 1 || h[1].System.CPUPercent != 2 {
		t.Fatalf("history order wrong: %+v", h)
	}
}

func TestEviction(t *testing.T) {
	r := New(3)
	for i := 1; i <= 5; i++ {
		r.Add(sample(float64(i)))
	}
	h := r.History()
	if len(h) != 3 {
		t.Fatalf("expected 3 kept, got %d", len(h))
	}
	if h[0].System.CPUPercent != 3 || h[2].System.CPUPercent != 5 {
		t.Fatalf("wrong window kept: %v..%v", h[0].System.CPUPercent, h[2].System.CPUPercent)
	}
	if l, _ := r.Latest(); l.System.CPUPercent != 5 {
		t.Fatalf("latest after eviction = %v", l.System.CPUPercent)
	}
}
