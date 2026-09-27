// Package format turns raw numbers into strings a person can read at a glance.
package format

import (
	"fmt"
	"time"
)

// Bytes formats a byte count with binary units (KiB, MiB, ...).
func Bytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// BytesPerSec formats a throughput reading.
func BytesPerSec(f float64) string {
	if f < 0 {
		f = 0
	}
	return Bytes(uint64(f)) + "/s"
}

// Percent formats a percentage with one decimal.
func Percent(f float64) string {
	return fmt.Sprintf("%.1f%%", f)
}

// Duration formats a number of seconds as a compact uptime string.
func Duration(sec uint64) string {
	d := time.Duration(sec) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dg %dsa %ddk", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dsa %ddk", hours, mins)
	}
	return fmt.Sprintf("%ddk", mins)
}
