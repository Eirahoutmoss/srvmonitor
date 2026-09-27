// Package collect gathers system and Oracle metrics into model.Sample values.
package collect

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"

	"srvmon/internal/model"
)

// System collects host metrics. It is stateful: network throughput is derived
// from the delta between successive calls.
type System struct {
	prevNet  map[string]net.IOCountersStat
	prevTime time.Time
}

// NewSystem returns a system collector.
func NewSystem() *System { return &System{prevNet: map[string]net.IOCountersStat{}} }

// Collect reads a full set of host metrics.
func (s *System) Collect() model.SystemMetrics {
	var m model.SystemMetrics

	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		m.CPUPercent = round1(pcts[0])
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		m.MemUsed, m.MemTotal, m.MemPercent = vm.Used, vm.Total, round1(vm.UsedPercent)
	}
	if sw, err := mem.SwapMemory(); err == nil {
		m.SwapUsed, m.SwapTotal, m.SwapPercent = sw.Used, sw.Total, round1(sw.UsedPercent)
	}
	if up, err := host.Uptime(); err == nil {
		m.UptimeSec = up
	}
	if info, err := host.Info(); err == nil {
		m.Procs = info.Procs
	}

	if parts, err := disk.Partitions(false); err == nil {
		for _, p := range parts {
			u, err := disk.Usage(p.Mountpoint)
			if err != nil || u.Total == 0 {
				continue
			}
			m.Disks = append(m.Disks, model.DiskUsage{
				Path: p.Mountpoint, Total: u.Total, Used: u.Used,
				Free: u.Free, UsedPercent: round1(u.UsedPercent),
			})
		}
	}

	now := time.Now()
	if counters, err := net.IOCounters(true); err == nil {
		elapsed := now.Sub(s.prevTime).Seconds()
		for _, c := range counters {
			prev, ok := s.prevNet[c.Name]
			if ok && elapsed > 0 {
				m.Nets = append(m.Nets, model.NetThroughput{
					Name:       c.Name,
					RxBytesSec: round1(float64(c.BytesRecv-prev.BytesRecv) / elapsed),
					TxBytesSec: round1(float64(c.BytesSent-prev.BytesSent) / elapsed),
				})
			}
			s.prevNet[c.Name] = c
		}
	}
	s.prevTime = now
	return m
}

// Flatten builds the metric map the alert engine evaluates from a sample.
func Flatten(sample model.Sample) map[string]float64 {
	v := map[string]float64{
		"cpu.percent":  sample.System.CPUPercent,
		"mem.percent":  sample.System.MemPercent,
		"swap.percent": sample.System.SwapPercent,
	}
	for _, d := range sample.System.Disks {
		v[fmt.Sprintf("disk.%s.percent", d.Path)] = d.UsedPercent
	}
	for _, n := range sample.System.Nets {
		v[fmt.Sprintf("net.%s.rx_bps", n.Name)] = n.RxBytesSec
		v[fmt.Sprintf("net.%s.tx_bps", n.Name)] = n.TxBytesSec
	}
	for _, o := range sample.Databases {
		up := 0.0
		if o.Up {
			up = 1
		}
		v[fmt.Sprintf("db.%s.up", o.Name)] = up
		v[fmt.Sprintf("db.%s.session_percent", o.Name)] = o.SessionPercent
		v[fmt.Sprintf("db.%s.blocking", o.Name)] = float64(o.BlockingSessions)
		v[fmt.Sprintf("db.%s.buffer_hit_ratio", o.Name)] = o.BufferHitRatio
		for _, sp := range o.Spaces {
			v[fmt.Sprintf("db.%s.space.%s.percent", o.Name, sp.Name)] = sp.UsedPercent
		}
	}

	// Deep-scan counts, so a rule can raise e-mail/SMS on service or event
	// problems just like any metric threshold.
	v["services.problem"] = float64(len(sample.Services))
	var crit, errs float64
	for _, e := range sample.Events {
		switch e.Level {
		case "Kritik":
			crit++
		case "Hata":
			errs++
		}
	}
	v["events.critical"] = crit
	v["events.error"] = errs

	// Watched items that are definitively down (skip undetermined states).
	var down float64
	for _, wch := range sample.Watch {
		if !wch.Up && wch.State != "Bilinmiyor" && wch.State != "Tanımsız (url veya tcp gerekli)" {
			down++
		}
	}
	v["watch.down"] = down
	return v
}

func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}
