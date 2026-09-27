package collect

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"srvmon/internal/config"
	"srvmon/internal/model"
)

// CollectWatchedProcesses reports, for each named process image, whether at
// least one matching process is running. Matching is case-insensitive and
// tolerant of a trailing ".exe".
func CollectWatchedProcesses(names []string) []model.WatchStatus {
	if len(names) == 0 {
		return nil
	}
	running := map[string]bool{}
	if procs, err := process.Processes(); err == nil {
		for _, p := range procs {
			if n, err := p.Name(); err == nil {
				running[normProc(n)] = true
			}
		}
	}
	out := make([]model.WatchStatus, 0, len(names))
	for _, name := range names {
		up := running[normProc(name)]
		out = append(out, model.WatchStatus{
			Name: name, Kind: "süreç", Up: up, State: boolState(up, "Çalışıyor", "Bulunamadı"),
		})
	}
	return out
}

// CollectWatchedEndpoints probes each health endpoint. An HTTP URL is up when
// it answers with the expected status (or any 2xx when expect is 0); a TCP
// target is up when the connection succeeds.
func CollectWatchedEndpoints(eps []config.EndpointConfig, timeout time.Duration) []model.WatchStatus {
	if len(eps) == 0 {
		return nil
	}
	client := &http.Client{Timeout: timeout}
	out := make([]model.WatchStatus, 0, len(eps))
	for _, ep := range eps {
		ws := model.WatchStatus{Name: ep.Name, Kind: "uç nokta"}
		switch {
		case ep.URL != "":
			resp, err := client.Get(ep.URL)
			if err != nil {
				ws.Up, ws.State = false, "Yanıt yok"
			} else {
				resp.Body.Close()
				ok := (ep.ExpectStatus == 0 && resp.StatusCode < 300) || resp.StatusCode == ep.ExpectStatus
				ws.Up = ok
				ws.State = fmt.Sprintf("HTTP %d", resp.StatusCode)
				if ok {
					ws.State = "Yanıt veriyor (" + ws.State + ")"
				}
			}
		case ep.TCP != "":
			conn, err := net.DialTimeout("tcp", ep.TCP, timeout)
			if err != nil {
				ws.Up, ws.State = false, "Bağlanılamıyor"
			} else {
				conn.Close()
				ws.Up, ws.State = true, "Bağlantı açık"
			}
		default:
			ws.State = "Tanımsız (url veya tcp gerekli)"
		}
		out = append(out, ws)
	}
	return out
}

func normProc(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.TrimSuffix(s, ".exe")
}

func boolState(up bool, yes, no string) string {
	if up {
		return yes
	}
	return no
}
