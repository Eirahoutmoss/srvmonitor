package assess

import (
	"strings"
	"testing"

	"srvmon/internal/alert"
	"srvmon/internal/forecast"
	"srvmon/internal/model"
)

func TestMergePredictionRaisesHealthyToWarning(t *testing.T) {
	h := Evaluate(model.Sample{Values: map[string]float64{}}, map[string]alert.Level{})
	if h.Level != alert.OK {
		t.Fatalf("baseline should be OK, got %v", h.Level)
	}
	preds := []forecast.Prediction{{
		Key: "disk.C:.percent", Subject: "C: diski", ETAHours: 9, RatePerH: 2,
		Level: alert.Warn, Title: "C: diski ~9sa içinde dolabilir",
	}}
	h = Merge(h, preds)
	if h.Level != alert.Warn {
		t.Fatalf("prediction should raise to Warn, got %v", h.Level)
	}
	if !strings.Contains(h.Summary, "yaklaşan risk") {
		t.Fatalf("summary should mention upcoming risk: %q", h.Summary)
	}
	if len(h.Findings) != 1 || h.Findings[0].Source != "öngörü" {
		t.Fatalf("expected one öngörü finding, got %+v", h.Findings)
	}
}

func TestMergeCriticalPredictionLeads(t *testing.T) {
	h := Evaluate(model.Sample{Values: map[string]float64{}}, map[string]alert.Level{})
	preds := []forecast.Prediction{{
		Key: "disk.C:.percent", Subject: "C: diski", ETAHours: 3, RatePerH: 8,
		Level: alert.Crit, Title: "C: diski ~3sa içinde dolabilir",
	}}
	h = Merge(h, preds)
	if h.Level != alert.Crit {
		t.Fatalf("critical prediction should make health Crit, got %v", h.Level)
	}
	if h.Findings[0].Level != alert.Crit {
		t.Fatalf("critical finding should lead, got %+v", h.Findings[0])
	}
}

func TestMergeNoPredictionsUnchanged(t *testing.T) {
	base := Evaluate(model.Sample{Values: map[string]float64{}}, map[string]alert.Level{})
	h := Merge(base, nil)
	if h.Level != alert.OK || !strings.Contains(h.Summary, "sağlıklı") {
		t.Fatalf("no predictions should stay healthy: %+v", h)
	}
}
