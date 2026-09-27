package digest

import (
	"strings"
	"testing"
	"time"

	"srvmon/internal/alert"
	"srvmon/internal/assess"
	"srvmon/internal/forecast"
	"srvmon/internal/model"
)

func TestBuildContainsSections(t *testing.T) {
	s := model.Sample{
		Server: "PROD-01",
		System: model.SystemMetrics{CPUPercent: 12, MemPercent: 40, Disks: []model.DiskUsage{
			{Path: "C:", UsedPercent: 93, Free: 5 << 30},
		}},
		Watch: []model.WatchStatus{
			{Name: "MyApp", Kind: "servis", Up: false, State: "Durdu"},
		},
		Databases: []model.DBMetrics{{Name: "PROD", Type: "oracle", Up: true, Status: "OPEN", Sessions: 10, MaxSessions: 100}},
		Values:    map[string]float64{},
	}
	active := map[string]alert.Level{"disk.C:.percent": alert.Crit}
	health := assess.Evaluate(s, active)
	preds := []forecast.Prediction{{Subject: "C: diski", Title: "C: diski ~4sa içinde dolabilir", Level: alert.Crit, ETAHours: 4}}
	subj, html := Build(s, health, preds, time.Date(2026, 1, 2, 8, 30, 0, 0, time.UTC))

	if !strings.Contains(subj, "PROD-01") || !strings.Contains(subj, "KRİTİK") {
		t.Fatalf("subject = %q", subj)
	}
	for _, want := range []string{"Bulgular", "Öngörüler", "İzlenen", "MyApp", "Diskler", "C:", "ORACLE"} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q", want)
		}
	}
	// user-supplied text must be HTML-escaped (no raw injection)
	if strings.Contains(html, "<script>") {
		t.Error("unescaped content leaked")
	}
}

func TestBuildHealthy(t *testing.T) {
	s := model.Sample{Server: "OK-01", Values: map[string]float64{}}
	h := assess.Evaluate(s, map[string]alert.Level{})
	subj, html := Build(s, h, nil, time.Now())
	if !strings.Contains(subj, "NORMAL") {
		t.Errorf("subject = %q", subj)
	}
	if !strings.Contains(html, "sağlıklı") {
		t.Errorf("healthy body should say so")
	}
}
