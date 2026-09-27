package config

import (
	"fmt"
	"time"
)

// NextDigest returns the next time the digest e-mail should be sent after now.
// DailyAt ("HH:MM", local time) takes precedence; otherwise EveryHours is used;
// if neither is set it defaults to every 24 hours.
func (d DigestConfig) NextDigest(now time.Time) (time.Time, error) {
	if d.DailyAt != "" {
		var h, m int
		if _, err := fmt.Sscanf(d.DailyAt, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
			return time.Time{}, fmt.Errorf("digest.daily_at 'SS:DD' biçiminde olmalı, %q verildi", d.DailyAt)
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		return next, nil
	}
	hours := d.EveryHours
	if hours <= 0 {
		hours = 24
	}
	return now.Add(time.Duration(hours) * time.Hour), nil
}
