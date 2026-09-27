package collect

import (
	"time"

	"srvmon/internal/model"
)

// DemoServices returns representative problem services, for previewing the
// dashboard with the -demo flag on any platform.
func DemoServices() []model.ServiceStatus {
	return []model.ServiceStatus{
		{Name: "SQLSERVERAGENT", Display: "SQL Server Agent", State: "Durdu", StartType: "Otomatik", Problem: true},
		{Name: "W3SVC", Display: "World Wide Web Publishing", State: "Durdu", StartType: "Otomatik", Problem: true},
	}
}

// DemoEvents returns representative Event Log records for the -demo flag.
func DemoEvents(now time.Time) []model.EventEntry {
	return []model.EventEntry{
		{Time: now.Add(-3 * time.Minute), Level: "Kritik", Log: "System", Provider: "disk", ID: 7,
			Message: "The device, \\Device\\Harddisk1\\DR1, has a bad block."},
		{Time: now.Add(-8 * time.Minute), Level: "Hata", Log: "Application", Provider: "MSSQLSERVER", ID: 18456,
			Message: "Login failed for user 'app'. Reason: Password did not match."},
		{Time: now.Add(-21 * time.Minute), Level: "Hata", Log: "System", Provider: "Service Control Manager", ID: 7034,
			Message: "The SQL Server Agent service terminated unexpectedly."},
	}
}

// DemoWatch returns representative watched-item statuses for the -demo flag,
// including a user application watched by service, process and endpoint.
func DemoWatch() []model.WatchStatus {
	return []model.WatchStatus{
		{Name: "MSSQLSERVER", Kind: "servis", Up: true, State: "Çalışıyor"},
		{Name: "SiparişUygulaması", Kind: "servis", Up: false, State: "Durdu"},
		{Name: "MyApp.exe", Kind: "süreç", Up: true, State: "Çalışıyor"},
		{Name: "Sipariş API", Kind: "uç nokta", Up: false, State: "HTTP 503"},
		{Name: "Ödeme Servisi", Kind: "uç nokta", Up: true, State: "Yanıt veriyor (HTTP 200)"},
	}
}

// DemoDatabases returns representative readings for the -demo flag, covering
// all three supported engines.
func DemoDatabases() []model.DBMetrics {
	return []model.DBMetrics{
		{
			Name: "PROD", Type: "oracle", Up: true, Status: "OPEN", UptimeSec: 812340,
			Sessions: 148, MaxSessions: 200, SessionPercent: 74, BlockingSessions: 2,
			BufferHitRatio: 98.7, LatencyMS: 3,
			Spaces: []model.DBSpace{
				{Name: "USERS", UsedPercent: 91.4},
				{Name: "SYSAUX", UsedPercent: 68.2},
				{Name: "SYSTEM", UsedPercent: 55.0},
			},
		},
		{
			Name: "raporlar", Type: "postgres", Up: true, Status: "OPEN", UptimeSec: 442100,
			Sessions: 63, MaxSessions: 100, SessionPercent: 63, BlockingSessions: 0,
			BufferHitRatio: 99.2, LatencyMS: 1,
		},
		{
			Name: "web", Type: "mysql", Up: false, Error: "dial tcp 10.0.0.12:3306: connect: connection refused",
		},
	}
}
