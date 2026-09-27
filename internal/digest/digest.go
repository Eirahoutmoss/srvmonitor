// Package digest renders a full status snapshot as an HTML e-mail body, for the
// periodic "durum bilgisi" mail. It is pure/testable: given a sample, health
// verdict and predictions, it returns the subject and HTML.
package digest

import (
	"fmt"
	"html"
	"strings"
	"time"

	"srvmon/internal/alert"
	"srvmon/internal/assess"
	"srvmon/internal/forecast"
	"srvmon/internal/format"
	"srvmon/internal/model"
)

func color(l alert.Level) string {
	switch l {
	case alert.Crit:
		return "#dc2626"
	case alert.Warn:
		return "#d97706"
	default:
		return "#16a34a"
	}
}

// Build renders the digest subject and HTML body.
func Build(s model.Sample, h assess.Health, preds []forecast.Prediction, now time.Time) (subject, body string) {
	icon := "✓"
	if h.Level == alert.Crit {
		icon = "⛔"
	} else if h.Level == alert.Warn {
		icon = "⚠"
	}
	subject = fmt.Sprintf("[%s] %s durum raporu · %s", h.Level.String(), s.Server, now.Format("2006-01-02 15:04"))

	var b strings.Builder
	b.WriteString(`<div style="font-family:Segoe UI,Arial,sans-serif;max-width:680px;margin:auto;color:#1a1d23">`)
	fmt.Fprintf(&b, `<div style="background:%s;color:#fff;padding:16px 20px;border-radius:10px">
<div style="font-size:20px;font-weight:700">%s %s</div>
<div style="opacity:.9">%s · %s</div></div>`,
		color(h.Level), icon, html.EscapeString(h.Summary), html.EscapeString(s.Server), now.Format("02.01.2006 15:04"))

	// Findings
	if len(h.Findings) > 0 {
		b.WriteString(`<h3 style="margin:18px 0 8px">Bulgular</h3>`)
		for _, f := range h.Findings {
			fmt.Fprintf(&b, `<div style="border-left:4px solid %s;background:#f7f7f9;padding:8px 12px;margin-bottom:6px;border-radius:6px">
<b>%s</b> <span style="color:%s;font-size:12px">[%s · %s]</span><br><span style="color:#6b7280;font-size:12px">%s</span></div>`,
				color(f.Level), html.EscapeString(f.Title), color(f.Level), f.Level.String(),
				html.EscapeString(f.Source), html.EscapeString(f.Detail))
		}
	} else {
		b.WriteString(`<p style="color:#16a34a">Aktif bulgu yok — sistem sağlıklı.</p>`)
	}

	// Predictions
	if len(preds) > 0 {
		b.WriteString(`<h3 style="margin:18px 0 8px">Öngörüler · Erken Uyarı</h3><ul style="margin:0;padding-left:18px">`)
		for _, p := range preds {
			fmt.Fprintf(&b, `<li>%s</li>`, html.EscapeString(p.Title))
		}
		b.WriteString(`</ul>`)
	}

	// Watched items
	if len(s.Watch) > 0 {
		b.WriteString(`<h3 style="margin:18px 0 8px">İzlenen Uygulama/Servisler</h3>`)
		b.WriteString(tableOpen("Ad", "Tür", "Durum"))
		for _, w := range s.Watch {
			c := "#16a34a"
			if !w.Up {
				c = "#dc2626"
			}
			fmt.Fprintf(&b, `<tr><td style="%s">%s</td><td style="%s">%s</td><td style="%s;color:%s">%s</td></tr>`,
				td, html.EscapeString(w.Name), td, html.EscapeString(w.Kind), td, c, html.EscapeString(w.State))
		}
		b.WriteString(`</table>`)
	}

	// Disks
	if len(s.System.Disks) > 0 {
		b.WriteString(`<h3 style="margin:18px 0 8px">Diskler</h3>`)
		b.WriteString(tableOpen("Bölüm", "Doluluk", "Boş"))
		for _, d := range s.System.Disks {
			c := "#1a1d23"
			if d.UsedPercent >= 90 {
				c = "#dc2626"
			} else if d.UsedPercent >= 80 {
				c = "#d97706"
			}
			fmt.Fprintf(&b, `<tr><td style="%s">%s</td><td style="%s;color:%s">%s</td><td style="%s">%s</td></tr>`,
				td, html.EscapeString(d.Path), td, c, format.Percent(d.UsedPercent), td, format.Bytes(d.Free))
		}
		b.WriteString(`</table>`)
	}

	// System summary line
	fmt.Fprintf(&b, `<p style="color:#6b7280;font-size:12px;margin-top:16px">CPU %s · Bellek %s · Swap %s · Çalışma süresi %s</p>`,
		format.Percent(s.System.CPUPercent), format.Percent(s.System.MemPercent),
		format.Percent(s.System.SwapPercent), format.Duration(s.System.UptimeSec))

	// Databases
	for _, o := range s.Databases {
		label := strings.ToUpper(o.Type) + " · " + html.EscapeString(o.Name)
		if !o.Up {
			fmt.Fprintf(&b, `<p style="color:#dc2626">%s: ERİŞİLEMİYOR</p>`, label)
			continue
		}
		fmt.Fprintf(&b, `<p style="font-size:13px"><b>%s</b>: %s · bağlantı %d/%d (%s) · bloklayan %d · buffer hit %s</p>`,
			label, html.EscapeString(o.Status), o.Sessions, o.MaxSessions,
			format.Percent(o.SessionPercent), o.BlockingSessions, format.Percent(o.BufferHitRatio))
	}

	b.WriteString(`<p style="color:#9aa3af;font-size:11px;margin-top:18px">srvmon otomatik durum raporu</p></div>`)
	return subject, b.String()
}

const td = "padding:6px 10px;border-bottom:1px solid #eee;font-size:13px;text-align:left"

func tableOpen(cols ...string) string {
	var b strings.Builder
	b.WriteString(`<table style="width:100%;border-collapse:collapse;border:1px solid #eee;border-radius:8px"><tr>`)
	for _, c := range cols {
		fmt.Fprintf(&b, `<th style="padding:6px 10px;text-align:left;font-size:11px;color:#6b7280;border-bottom:1px solid #eee">%s</th>`, html.EscapeString(c))
	}
	b.WriteString(`</tr>`)
	return b.String()
}
