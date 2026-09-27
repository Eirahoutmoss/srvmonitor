package assess

import (
	"strings"
	"testing"
	"time"

	"srvmon/internal/alert"
	"srvmon/internal/model"
)

func TestHealthySystem(t *testing.T) {
	h := Evaluate(model.Sample{Values: map[string]float64{}}, map[string]alert.Level{})
	if h.Level != alert.OK || !strings.Contains(h.Summary, "sağlıklı") {
		t.Fatalf("expected healthy, got %+v", h)
	}
	if len(h.Findings) != 0 {
		t.Fatalf("no findings expected, got %v", h.Findings)
	}
}

func TestCriticalDominates(t *testing.T) {
	s := model.Sample{Values: map[string]float64{"disk.C:.percent": 95, "cpu.percent": 82}}
	active := map[string]alert.Level{"disk.C:.percent": alert.Crit, "cpu.percent": alert.Warn}
	h := Evaluate(s, active)
	if h.Level != alert.Crit {
		t.Fatalf("overall level = %v", h.Level)
	}
	if h.Findings[0].Level != alert.Crit {
		t.Fatalf("critical must sort first, got %+v", h.Findings[0])
	}
	if !strings.Contains(h.Findings[0].Title, "C: diski") {
		t.Fatalf("disk finding text wrong: %q", h.Findings[0].Title)
	}
	if !strings.Contains(h.Summary, "1 kritik, 1 uyarı") {
		t.Fatalf("summary = %q", h.Summary)
	}
}

func TestProblemService(t *testing.T) {
	s := model.Sample{Values: map[string]float64{}, Services: []model.ServiceStatus{
		{Name: "SQLSERVERAGENT", Display: "SQL Server Agent", State: "Durdu", StartType: "Otomatik", Problem: true},
		{Name: "Spooler", Display: "Yazdırma", State: "Çalışıyor", StartType: "Otomatik", Problem: false},
	}}
	h := Evaluate(s, map[string]alert.Level{})
	if h.Level != alert.Crit || len(h.Findings) != 1 {
		t.Fatalf("expected 1 crit finding, got %+v", h)
	}
	if !strings.Contains(h.Findings[0].Title, "SQL Server Agent") {
		t.Fatalf("service finding text wrong: %q", h.Findings[0].Title)
	}
}

func TestEventAggregation(t *testing.T) {
	now := time.Now()
	s := model.Sample{Values: map[string]float64{}, Events: []model.EventEntry{
		{Time: now, Level: "Kritik", Log: "System", Provider: "disk", ID: 7},
		{Time: now, Level: "Kritik", Log: "System", Provider: "disk", ID: 7},
		{Time: now, Level: "Hata", Log: "Application", Provider: "MSSQLSERVER", ID: 18456},
	}}
	h := Evaluate(s, map[string]alert.Level{})
	if h.Level != alert.Crit {
		t.Fatalf("level = %v", h.Level)
	}
	var critFound, warnFound bool
	for _, f := range h.Findings {
		if f.Source == "olay" && f.Level == alert.Crit {
			critFound = true
			if !strings.Contains(f.Detail, "disk (2)") {
				t.Errorf("event summary wrong: %q", f.Detail)
			}
		}
		if f.Source == "olay" && f.Level == alert.Warn {
			warnFound = true
		}
	}
	if !critFound || !warnFound {
		t.Fatalf("expected both crit and warn event findings: %+v", h.Findings)
	}
}

func TestOracleDownFinding(t *testing.T) {
	s := model.Sample{Values: map[string]float64{}, Databases: []model.DBMetrics{
		{Name: "PROD", Type: "oracle", Up: false, Error: "ORA-12541: TNS:no listener\ndetay"},
	}}
	h := Evaluate(s, map[string]alert.Level{})
	if h.Level != alert.Crit {
		t.Fatalf("level = %v", h.Level)
	}
	if !strings.Contains(h.Findings[0].Title, "PROD") || !strings.Contains(h.Findings[0].Detail, "ORA-12541") {
		t.Fatalf("oracle finding wrong: %+v", h.Findings[0])
	}
}

func TestAgeString(t *testing.T) {
	now := time.Now()
	if AgeString(now.Add(-30*time.Second), now) != "az önce" {
		t.Error("30s should be az önce")
	}
	if AgeString(now.Add(-5*time.Minute), now) != "5 dk önce" {
		t.Error("5m wrong")
	}
	if AgeString(now.Add(-2*time.Hour), now) != "2 sa önce" {
		t.Error("2h wrong")
	}
}
