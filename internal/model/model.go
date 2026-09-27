// Package model holds the data types shared across the collector, store,
// alert engine and web layers. Keeping them in one dependency-free package
// avoids import cycles.
package model

import "time"

// Sample is one full snapshot of a server at a point in time.
type Sample struct {
	Time      time.Time       `json:"time"`
	Server    string          `json:"server"`
	System    SystemMetrics   `json:"system"`
	Databases []DBMetrics     `json:"databases"`
	Services  []ServiceStatus `json:"services"`
	Events    []EventEntry    `json:"events"`
	Watch     []WatchStatus   `json:"watch"`
	// Values is the flattened metric map the alert engine evaluates.
	// Keys look like "cpu.percent", "disk.C:.percent", "oracle.PROD.up".
	Values map[string]float64 `json:"-"`
}

// ServiceStatus is one Windows service's state.
type ServiceStatus struct {
	Name      string `json:"name"`
	Display   string `json:"display"`
	State     string `json:"state"`      // Çalışıyor, Durdu, ...
	StartType string `json:"start_type"` // Otomatik, Manuel, Devre dışı
	Problem   bool   `json:"problem"`    // otomatik başlatmalı ama çalışmıyor
}

// WatchStatus is the state of a user-named service or process the operator
// explicitly asked to monitor.
type WatchStatus struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"` // "servis" | "süreç"
	Up    bool   `json:"up"`
	State string `json:"state"` // Çalışıyor / Durdu / Bulunamadı / Bilinmiyor
}

// EventEntry is one Windows Event Log record (Critical or Error).
type EventEntry struct {
	Time     time.Time `json:"time"`
	Level    string    `json:"level"` // Kritik, Hata
	Log      string    `json:"log"`   // System, Application
	Provider string    `json:"provider"`
	ID       int64     `json:"id"`
	Message  string    `json:"message"`
}

// SystemMetrics holds host-level readings.
type SystemMetrics struct {
	CPUPercent  float64         `json:"cpu_percent"`
	MemUsed     uint64          `json:"mem_used"`
	MemTotal    uint64          `json:"mem_total"`
	MemPercent  float64         `json:"mem_percent"`
	SwapUsed    uint64          `json:"swap_used"`
	SwapTotal   uint64          `json:"swap_total"`
	SwapPercent float64         `json:"swap_percent"`
	UptimeSec   uint64          `json:"uptime_sec"`
	Procs       uint64          `json:"procs"`
	Disks       []DiskUsage     `json:"disks"`
	Nets        []NetThroughput `json:"nets"`
}

// DiskUsage is one volume's space usage.
type DiskUsage struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// NetThroughput is one interface's throughput since the previous sample.
type NetThroughput struct {
	Name       string  `json:"name"`
	RxBytesSec float64 `json:"rx_bytes_sec"`
	TxBytesSec float64 `json:"tx_bytes_sec"`
}

// DBMetrics holds the health of one database, regardless of engine.
type DBMetrics struct {
	Name             string    `json:"name"`
	Type             string    `json:"type"` // oracle | postgres | mysql
	Up               bool      `json:"up"`
	Error            string    `json:"error,omitempty"`
	Status           string    `json:"status,omitempty"`
	Sessions         int64     `json:"sessions"`
	MaxSessions      int64     `json:"max_sessions"`
	SessionPercent   float64   `json:"session_percent"`
	BlockingSessions int64     `json:"blocking_sessions"`
	BufferHitRatio   float64   `json:"buffer_hit_ratio"`
	UptimeSec        int64     `json:"uptime_sec"`
	Spaces           []DBSpace `json:"spaces"` // Oracle tablespaces; boş olabilir
	LatencyMS        float64   `json:"latency_ms"`
}

// DBSpace is one storage area's fill level (an Oracle tablespace, etc.).
type DBSpace struct {
	Name        string  `json:"name"`
	UsedPercent float64 `json:"used_percent"`
}
