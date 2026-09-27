// Package forecast turns the recent history of a metric into an early warning:
// it fits a linear trend and estimates how long until the metric reaches a
// ceiling (100%). This is what lets the tool warn before a threshold is
// crossed instead of after — "C: diski ~5 saat içinde dolabilir".
//
// All functions are pure and time-argument driven, so they are unit-testable.
package forecast

import (
	"fmt"
	"math"
	"strings"
	"time"

	"srvmon/internal/alert"
	"srvmon/internal/model"
)

// Point is one (time, value) reading.
type Point struct {
	T time.Time
	V float64
}

// Prediction is an estimated time-to-ceiling for one metric.
type Prediction struct {
	Key      string      `json:"key"`
	Subject  string      `json:"subject"`
	Current  float64     `json:"current"`
	RatePerH float64     `json:"rate_per_hour"`
	ETAHours float64     `json:"eta_hours"`
	Level    alert.Level `json:"level"`
	Title    string      `json:"title"`
}

// Tunables.
const (
	minPoints = 5 // need enough history to trust a trend
	minSpan   = 45 * time.Second
	ceiling   = 100.0
)

// slopePerHour fits a least-squares line and returns the slope in units/hour,
// plus the time span covered. ok is false when there is too little data.
func slopePerHour(pts []Point) (slope float64, span time.Duration, ok bool) {
	if len(pts) < minPoints {
		return 0, 0, false
	}
	base := pts[0].T
	var n, sx, sy, sxx, sxy float64
	for _, p := range pts {
		x := p.T.Sub(base).Hours()
		y := p.V
		n++
		sx += x
		sy += y
		sxx += x * x
		sxy += x * y
	}
	den := n*sxx - sx*sx
	if den == 0 {
		return 0, 0, false
	}
	slope = (n*sxy - sx*sy) / den
	span = pts[len(pts)-1].T.Sub(base)
	if span < minSpan {
		return 0, 0, false
	}
	return slope, span, true
}

// ETA estimates hours until the last value reaches the ceiling, given the
// series. ok is false when the metric is flat, falling, or already full.
func ETA(pts []Point) (hours, ratePerHour, current float64, ok bool) {
	slope, _, valid := slopePerHour(pts)
	if !valid {
		return 0, 0, 0, false
	}
	current = pts[len(pts)-1].V
	if current >= ceiling || slope <= 0 {
		return 0, slope, current, false
	}
	hours = (ceiling - current) / slope
	if math.IsInf(hours, 0) || math.IsNaN(hours) || hours <= 0 {
		return 0, slope, current, false
	}
	return hours, slope, current, true
}

// forecastable reports whether a metric key is a "fills toward a ceiling"
// gauge worth predicting (disk, memory, swap, DB connections, spaces).
func forecastable(key string) bool {
	switch {
	case strings.HasPrefix(key, "disk.") && strings.HasSuffix(key, ".percent"):
		return true
	case key == "mem.percent" || key == "swap.percent":
		return true
	case strings.HasSuffix(key, ".session_percent"):
		return true
	case strings.Contains(key, ".space.") && strings.HasSuffix(key, ".percent"):
		return true
	}
	return false
}

func subjectOf(key string) string {
	switch {
	case strings.HasPrefix(key, "disk."):
		return strings.TrimSuffix(strings.TrimPrefix(key, "disk."), ".percent") + " diski"
	case key == "mem.percent":
		return "Bellek"
	case key == "swap.percent":
		return "Swap"
	case strings.HasSuffix(key, ".session_percent"):
		return strings.TrimSuffix(strings.TrimPrefix(key, "db."), ".session_percent") + " bağlantıları"
	case strings.Contains(key, ".space."):
		p := strings.TrimPrefix(key, "db.")
		db, rest, _ := strings.Cut(p, ".space.")
		sp := strings.TrimSuffix(rest, ".percent")
		return db + " · " + sp + " alanı"
	}
	return key
}

func humanHours(h float64) string {
	d := time.Duration(h * float64(time.Hour))
	days := int(d.Hours()) / 24
	hrs := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dg %dsa", days, hrs)
	case hrs > 0:
		return fmt.Sprintf("%dsa %ddk", hrs, mins)
	default:
		return fmt.Sprintf("%ddk", mins)
	}
}

// All computes predictions from the sample history for every forecastable
// metric whose estimated time-to-full is within horizon. Results are sorted
// most-urgent first.
func All(history []model.Sample, horizon time.Duration) []Prediction {
	if len(history) < minPoints {
		return nil
	}
	// Collect series per key from in-memory Values.
	series := map[string][]Point{}
	for _, s := range history {
		for k, v := range s.Values {
			if forecastable(k) {
				series[k] = append(series[k], Point{T: s.Time, V: v})
			}
		}
	}

	var out []Prediction
	for key, pts := range series {
		hours, rate, cur, ok := ETA(pts)
		if !ok || hours > horizon.Hours() {
			continue
		}
		lvl := alert.Warn
		if hours <= 6 {
			lvl = alert.Crit
		}
		out = append(out, Prediction{
			Key: key, Subject: subjectOf(key), Current: round1(cur),
			RatePerH: round1(rate), ETAHours: round1(hours), Level: lvl,
			Title: fmt.Sprintf("%s ~%s içinde dolabilir (şu an %%%.0f, +%%%.1f/sa)",
				subjectOf(key), humanHours(hours), cur, rate),
		})
	}
	// Sort by ETA ascending (most urgent first).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].ETAHours < out[j-1].ETAHours; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// MinETAHours returns the soonest predicted time-to-full in hours, or a large
// sentinel when nothing is trending up. Feed this to the alert engine so a
// rule can e-mail before a ceiling is hit.
func MinETAHours(preds []Prediction) float64 {
	min := 1e9
	for _, p := range preds {
		if p.ETAHours < min {
			min = p.ETAHours
		}
	}
	return min
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }
