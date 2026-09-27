//go:build !windows

package collect

import (
	"time"

	"srvmon/internal/model"
)

// CollectServices is a no-op off Windows; service inspection is Windows-only.
func CollectServices() []model.ServiceStatus { return nil }

// CollectWatchedServices off Windows marks each named service as unknown.
func CollectWatchedServices(names []string) []model.WatchStatus {
	out := make([]model.WatchStatus, 0, len(names))
	for _, n := range names {
		out = append(out, model.WatchStatus{Name: n, Kind: "servis", State: "Bilinmiyor"})
	}
	return out
}

// CollectEvents is a no-op off Windows; the Event Log is Windows-only.
func CollectEvents(window time.Duration, max int) []model.EventEntry { return nil }
