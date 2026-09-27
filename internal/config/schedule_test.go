package config

import (
	"testing"
	"time"
)

func TestNextDigestDaily(t *testing.T) {
	now := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	d := DigestConfig{DailyAt: "08:30"}
	next, err := d.NextDigest(now)
	if err != nil {
		t.Fatal(err)
	}
	// 08:30 already passed today -> tomorrow 08:30
	if next.Day() != 3 || next.Hour() != 8 || next.Minute() != 30 {
		t.Fatalf("next = %v", next)
	}
}

func TestNextDigestDailyLaterToday(t *testing.T) {
	now := time.Date(2026, 1, 2, 7, 0, 0, 0, time.UTC)
	d := DigestConfig{DailyAt: "08:30"}
	next, _ := d.NextDigest(now)
	if next.Day() != 2 || next.Hour() != 8 {
		t.Fatalf("should be today 08:30, got %v", next)
	}
}

func TestNextDigestEveryHours(t *testing.T) {
	now := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	d := DigestConfig{EveryHours: 8}
	next, _ := d.NextDigest(now)
	if next.Sub(now) != 8*time.Hour {
		t.Fatalf("delta = %v", next.Sub(now))
	}
}

func TestNextDigestBadTime(t *testing.T) {
	d := DigestConfig{DailyAt: "25:99"}
	if _, err := d.NextDigest(time.Now()); err == nil {
		t.Fatal("expected error for bad daily_at")
	}
}
