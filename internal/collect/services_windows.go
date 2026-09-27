//go:build windows

package collect

import (
	"sort"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"srvmon/internal/model"
)

// CollectServices returns services that are set to start automatically but are
// not running — the real "problem" signal. Requires administrator / LocalSystem
// rights to see every service; anything it cannot open is skipped.
func CollectServices() []model.ServiceStatus {
	m, err := mgr.Connect()
	if err != nil {
		return nil
	}
	defer m.Disconnect()

	names, err := m.ListServices()
	if err != nil {
		return nil
	}

	var out []model.ServiceStatus
	for _, name := range names {
		s, err := m.OpenService(name)
		if err != nil {
			continue
		}
		cfg, cErr := s.Config()
		st, sErr := s.Query()
		s.Close()
		if cErr != nil || sErr != nil {
			continue
		}
		if cfg.StartType != mgr.StartAutomatic || st.State == svc.Running {
			continue // only surface automatic services that are not running
		}
		out = append(out, model.ServiceStatus{
			Name:      name,
			Display:   cfg.DisplayName,
			State:     stateName(st.State),
			StartType: "Otomatik",
			Problem:   true,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CollectWatchedServices reports the state of each named service the operator
// explicitly asked to monitor (including their own application's service).
func CollectWatchedServices(names []string) []model.WatchStatus {
	if len(names) == 0 {
		return nil
	}
	m, err := mgr.Connect()
	if err != nil {
		out := make([]model.WatchStatus, 0, len(names))
		for _, n := range names {
			out = append(out, model.WatchStatus{Name: n, Kind: "servis", State: "Bilinmiyor"})
		}
		return out
	}
	defer m.Disconnect()

	out := make([]model.WatchStatus, 0, len(names))
	for _, name := range names {
		ws := model.WatchStatus{Name: name, Kind: "servis"}
		s, err := m.OpenService(name)
		if err != nil {
			ws.State = "Bulunamadı"
			out = append(out, ws)
			continue
		}
		st, qErr := s.Query()
		s.Close()
		if qErr != nil {
			ws.State = "Sorgulanamadı"
		} else {
			ws.Up = st.State == svc.Running
			ws.State = stateName(st.State)
		}
		out = append(out, ws)
	}
	return out
}

func stateName(s svc.State) string {
	switch s {
	case svc.Stopped:
		return "Durdu"
	case svc.StartPending:
		return "Başlatılıyor"
	case svc.StopPending:
		return "Durduruluyor"
	case svc.Running:
		return "Çalışıyor"
	case svc.Paused:
		return "Duraklatıldı"
	default:
		return "Bilinmiyor"
	}
}
