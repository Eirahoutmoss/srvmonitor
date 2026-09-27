// Package assess turns a raw sample plus the active alarm levels into a
// human-readable health verdict: a short summary and a ranked list of
// findings written in plain Turkish ("C: diski %92 dolu", "SQLSERVERAGENT
// servisi durmuş"). This is what separates the tool from a task manager.
package assess

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"srvmon/internal/alert"
	"srvmon/internal/format"
	"srvmon/internal/model"
)

// Finding is one diagnosed issue.
type Finding struct {
	Level  alert.Level `json:"level"`
	Title  string      `json:"title"`
	Detail string      `json:"detail"`
	Source string      `json:"source"` // "sistem", "veritabanı", "servis", "olay", "izleme", "öngörü"
}

// Health is the overall verdict for a sample.
type Health struct {
	Level    alert.Level `json:"level"`
	Summary  string      `json:"summary"`
	Findings []Finding   `json:"findings"`
}

// describe maps a flattened metric key to a plain-Turkish problem statement.
func describe(key string, value float64) string {
	switch {
	case key == "cpu.percent":
		return "İşlemci kullanımı yüksek: " + format.Percent(value)
	case key == "mem.percent":
		return "Bellek kullanımı yüksek: " + format.Percent(value)
	case key == "swap.percent":
		return "Swap kullanımı yüksek: " + format.Percent(value)
	case strings.HasPrefix(key, "disk.") && strings.HasSuffix(key, ".percent"):
		vol := strings.TrimSuffix(strings.TrimPrefix(key, "disk."), ".percent")
		return vol + " diski dolmaya yakın: " + format.Percent(value)
	case strings.HasSuffix(key, ".up"):
		db := strings.TrimSuffix(strings.TrimPrefix(key, "db."), ".up")
		return db + " veritabanına erişilemiyor"
	case strings.HasSuffix(key, ".session_percent"):
		db := strings.TrimSuffix(strings.TrimPrefix(key, "db."), ".session_percent")
		return db + " bağlantı sayısı üst sınıra yaklaşıyor: " + format.Percent(value)
	case strings.HasSuffix(key, ".blocking"):
		db := strings.TrimSuffix(strings.TrimPrefix(key, "db."), ".blocking")
		return db + " veritabanında bloklayan/bekleyen oturum var: " + fmt.Sprintf("%.0f adet", value)
	case strings.Contains(key, ".space."):
		return "Depolama alanı dolmaya yakın (" + key + "): " + format.Percent(value)
	default:
		return key + " eşik dışı: " + fmt.Sprintf("%.1f", value)
	}
}

// source classifies a metric key for grouping in the UI.
func sourceOf(key string) string {
	if strings.HasPrefix(key, "db.") {
		return "veritabanı"
	}
	return "sistem"
}

// Evaluate builds the health verdict. active maps a metric key to its level.
func Evaluate(s model.Sample, active map[string]alert.Level) Health {
	var f []Finding

	for key, lvl := range active {
		val := s.Values[key]
		f = append(f, Finding{Level: lvl, Source: sourceOf(key),
			Title:  describe(key, val),
			Detail: "Metrik: " + key})
	}

	// Database explicit down (in case no rule covers it).
	for _, o := range s.Databases {
		if !o.Up {
			f = append(f, Finding{Level: alert.Crit, Source: "veritabanı",
				Title:  o.Type + " · " + o.Name + " veritabanına erişilemiyor",
				Detail: firstLine(o.Error)})
		}
	}

	// Problem services: automatic but not running.
	for _, sv := range s.Services {
		if sv.Problem {
			name := sv.Display
			if name == "" {
				name = sv.Name
			}
			f = append(f, Finding{Level: alert.Crit, Source: "servis",
				Title:  name + " servisi çalışmıyor",
				Detail: fmt.Sprintf("%s · başlatma: %s · durum: %s", sv.Name, sv.StartType, sv.State)})
		}
	}

	// Watched items (services / processes / endpoints) that are down.
	for _, wch := range s.Watch {
		if wch.Up || wch.State == "Bilinmiyor" || strings.HasPrefix(wch.State, "Tanımsız") {
			continue
		}
		f = append(f, Finding{Level: alert.Crit, Source: "izleme",
			Title:  "İzlenen " + wch.Kind + " çalışmıyor: " + wch.Name,
			Detail: "Durum: " + wch.State})
	}

	// Event log: recent Critical -> crit finding, Error -> warn, aggregated.
	crit, errs := countEvents(s.Events)
	if crit > 0 {
		f = append(f, Finding{Level: alert.Crit, Source: "olay",
			Title:  fmt.Sprintf("Olay günlüğünde %d kritik kayıt", crit),
			Detail: eventSummary(s.Events, "Kritik")})
	}
	if errs > 0 {
		f = append(f, Finding{Level: alert.Warn, Source: "olay",
			Title:  fmt.Sprintf("Olay günlüğünde %d hata kaydı", errs),
			Detail: eventSummary(s.Events, "Hata")})
	}

	// Rank: critical first, then warnings; stable by title.
	sort.SliceStable(f, func(i, j int) bool {
		if f[i].Level != f[j].Level {
			return f[i].Level > f[j].Level
		}
		return f[i].Title < f[j].Title
	})

	h := Health{Findings: f, Level: alert.OK}
	var nc, nw int
	for _, x := range f {
		if x.Level == alert.Crit {
			nc++
		} else if x.Level == alert.Warn {
			nw++
		}
	}
	switch {
	case nc > 0:
		h.Level = alert.Crit
		h.Summary = fmt.Sprintf("%d kritik, %d uyarı bulgu var — acil müdahale gerekli", nc, nw)
	case nw > 0:
		h.Level = alert.Warn
		h.Summary = fmt.Sprintf("%d uyarı bulgu var — izlenmeli", nw)
	default:
		h.Summary = "Sistem sağlıklı — bilinen sorun yok"
	}
	return h
}

func countEvents(ev []model.EventEntry) (crit, errs int) {
	for _, e := range ev {
		switch e.Level {
		case "Kritik":
			crit++
		case "Hata":
			errs++
		}
	}
	return
}

func eventSummary(ev []model.EventEntry, level string) string {
	seen := map[string]int{}
	var order []string
	for _, e := range ev {
		if e.Level != level {
			continue
		}
		key := e.Provider
		if key == "" {
			key = e.Log
		}
		if _, ok := seen[key]; !ok {
			order = append(order, key)
		}
		seen[key]++
	}
	var parts []string
	for _, k := range order {
		parts = append(parts, fmt.Sprintf("%s (%d)", k, seen[k]))
	}
	if len(parts) > 4 {
		parts = parts[:4]
	}
	return strings.Join(parts, ", ")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

// AgeString renders how long ago t was, for the UI.
func AgeString(t, now time.Time) string {
	d := now.Sub(t)
	if d < time.Minute {
		return "az önce"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d dk önce", int(d.Minutes()))
	}
	return fmt.Sprintf("%d sa önce", int(d.Hours()))
}
