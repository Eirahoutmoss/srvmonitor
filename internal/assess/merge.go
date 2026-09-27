package assess

import (
	"fmt"

	"srvmon/internal/alert"
	"srvmon/internal/forecast"
)

// Merge folds trend predictions into a health verdict as "öngörü" findings.
// A prediction can raise the overall level even when no threshold is breached
// yet — this is the early-warning path ("disk ~5 saat içinde dolabilir").
func Merge(h Health, preds []forecast.Prediction) Health {
	for _, p := range preds {
		h.Findings = append(h.Findings, Finding{
			Level:  p.Level,
			Source: "öngörü",
			Title:  p.Title,
			Detail: fmt.Sprintf("Tahmini dolma: ~%.1f saat · artış hızı %%%.1f/sa · metrik %s",
				p.ETAHours, p.RatePerH, p.Key),
		})
	}

	// Recompute overall level and summary including predictions.
	var nc, nw, np int
	worst := alert.OK
	for _, f := range h.Findings {
		if f.Level > worst {
			worst = f.Level
		}
		switch {
		case f.Source == "öngörü":
			np++
		case f.Level == alert.Crit:
			nc++
		case f.Level == alert.Warn:
			nw++
		}
	}
	h.Level = worst

	switch {
	case nc > 0:
		h.Summary = fmt.Sprintf("%d kritik, %d uyarı bulgu var — acil müdahale gerekli", nc, nw)
	case np > 0 && nw == 0:
		h.Summary = fmt.Sprintf("Şu an sorun yok ama %d yaklaşan risk öngörülüyor", np)
	case nw > 0:
		h.Summary = fmt.Sprintf("%d uyarı bulgu var — izlenmeli", nw)
	default:
		h.Summary = "Sistem sağlıklı — bilinen sorun yok"
	}
	if np > 0 && nc == 0 && nw > 0 {
		h.Summary += fmt.Sprintf(" · %d yaklaşan risk", np)
	}

	// Re-sort so criticals lead, predictions ranked by their level.
	sortFindings(h.Findings)
	return h
}
