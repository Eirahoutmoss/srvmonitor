package forecast

import (
	"testing"
	"time"

	"srvmon/internal/alert"
	"srvmon/internal/model"
)

// series builds evenly-spaced points rising from start by step each minute.
func series(start, step float64, n int) []Point {
	base := time.Now().Add(-time.Duration(n) * time.Minute)
	pts := make([]Point, n)
	for i := 0; i < n; i++ {
		pts[i] = Point{T: base.Add(time.Duration(i) * time.Minute), V: start + step*float64(i)}
	}
	return pts
}

func TestETARising(t *testing.T) {
	// 50% rising 1%/minute => 60%/hour; from 50 to 100 is ~50 min.
	pts := series(50, 1, 11) // last value 60
	hours, rate, cur, ok := ETA(pts)
	if !ok {
		t.Fatal("expected a prediction")
	}
	if cur != 60 {
		t.Fatalf("current = %v", cur)
	}
	if rate < 59 || rate > 61 {
		t.Fatalf("rate/hour = %v want ~60", rate)
	}
	// (100-60)/60 h = 0.666h ≈ 40 min
	if hours < 0.6 || hours > 0.75 {
		t.Fatalf("eta hours = %v want ~0.67", hours)
	}
}

func TestETAFlatNoPrediction(t *testing.T) {
	if _, _, _, ok := ETA(series(70, 0, 10)); ok {
		t.Fatal("flat series should not predict")
	}
}

func TestETAFallingNoPrediction(t *testing.T) {
	if _, _, _, ok := ETA(series(90, -1, 10)); ok {
		t.Fatal("falling series should not predict")
	}
}

func TestETATooFewPoints(t *testing.T) {
	if _, _, _, ok := ETA(series(50, 1, 3)); ok {
		t.Fatal("too few points should not predict")
	}
}

func TestETAAlreadyFull(t *testing.T) {
	if _, _, _, ok := ETA(series(100, 1, 10)); ok {
		t.Fatal("already at ceiling should not predict")
	}
}

func histFromKey(key string, start, step float64, n int) []model.Sample {
	base := time.Now().Add(-time.Duration(n) * time.Minute)
	out := make([]model.Sample, n)
	for i := 0; i < n; i++ {
		out[i] = model.Sample{
			Time:   base.Add(time.Duration(i) * time.Minute),
			Values: map[string]float64{key: start + step*float64(i)},
		}
	}
	return out
}

func TestAllProducesDiskPrediction(t *testing.T) {
	hist := histFromKey("disk.C:.percent", 80, 0.5, 12) // rising to 85.5, ~29%/h
	preds := All(hist, 48*time.Hour)
	if len(preds) != 1 {
		t.Fatalf("expected 1 prediction, got %d: %+v", len(preds), preds)
	}
	p := preds[0]
	if p.Subject != "C: diski" {
		t.Errorf("subject = %q", p.Subject)
	}
	if p.ETAHours <= 0 {
		t.Errorf("eta = %v", p.ETAHours)
	}
	if p.Level != alert.Crit && p.Level != alert.Warn {
		t.Errorf("level = %v", p.Level)
	}
}

func TestAllIgnoresCPU(t *testing.T) {
	hist := histFromKey("cpu.percent", 50, 2, 12) // rising CPU must be ignored
	if preds := All(hist, 48*time.Hour); len(preds) != 0 {
		t.Fatalf("cpu should not be forecast, got %+v", preds)
	}
}

func TestAllHorizonFilter(t *testing.T) {
	// Very slow rise: from 80 at 0.01%/min = 0.6%/h -> to 100 ~33h; horizon 6h filters it out.
	hist := histFromKey("disk.C:.percent", 80, 0.01, 12)
	if preds := All(hist, 6*time.Hour); len(preds) != 0 {
		t.Fatalf("slow trend should be filtered by horizon, got %+v", preds)
	}
	if preds := All(hist, 48*time.Hour); len(preds) != 1 {
		t.Fatalf("should appear within 48h horizon, got %+v", preds)
	}
}

func TestMinETAHours(t *testing.T) {
	if got := MinETAHours(nil); got < 1e8 {
		t.Fatalf("empty should be sentinel, got %v", got)
	}
	preds := []Prediction{{ETAHours: 10}, {ETAHours: 3}, {ETAHours: 7}}
	if got := MinETAHours(preds); got != 3 {
		t.Fatalf("min = %v want 3", got)
	}
}
