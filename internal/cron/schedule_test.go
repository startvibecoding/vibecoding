package cron

import (
	"testing"
	"time"
)

func TestParseScheduleEmpty(t *testing.T) {
	next, oneShot, err := ParseSchedule("", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !oneShot {
		t.Error("expected one-shot for empty schedule")
	}
	if !next.IsZero() {
		t.Error("expected zero next run for one-shot")
	}
}

func TestParseScheduleOnce(t *testing.T) {
	next, oneShot, err := ParseSchedule("@once", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !oneShot {
		t.Error("expected one-shot for @once")
	}
	if !next.IsZero() {
		t.Error("expected zero next run for @once")
	}
}

func TestParseScheduleEveryDuration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		schedule string
		wantDur  time.Duration
	}{
		{"@every 30m", 30 * time.Minute},
		{"@every 2h", 2 * time.Hour},
		{"@every 1d", 24 * time.Hour},
	}

	for _, tt := range tests {
		next, oneShot, err := ParseSchedule(tt.schedule, now)
		if err != nil {
			t.Errorf("ParseSchedule(%q): %v", tt.schedule, err)
			continue
		}
		if oneShot {
			t.Errorf("ParseSchedule(%q): unexpected one-shot", tt.schedule)
		}
		got := next.Sub(now).Round(time.Minute)
		if got != tt.wantDur {
			t.Errorf("ParseSchedule(%q): got %v, want %v", tt.schedule, got, tt.wantDur)
		}
	}
}

func TestParseScheduleNamed(t *testing.T) {
	now := time.Date(2026, 5, 29, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		schedule string
		wantNext time.Time
	}{
		{"@hourly", time.Date(2026, 5, 29, 16, 30, 0, 0, time.UTC)},
		{"@daily", time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)},
		{"@monthly", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		next, oneShot, err := ParseSchedule(tt.schedule, now)
		if err != nil {
			t.Errorf("ParseSchedule(%q): %v", tt.schedule, err)
			continue
		}
		if oneShot {
			t.Errorf("ParseSchedule(%q): unexpected one-shot", tt.schedule)
		}
		if !next.Equal(tt.wantNext) {
			t.Errorf("ParseSchedule(%q): got %v, want %v", tt.schedule, next, tt.wantNext)
		}
	}
}

func TestParseScheduleInvalid(t *testing.T) {
	_, _, err := ParseSchedule("invalid", time.Now())
	if err == nil {
		t.Error("expected error for invalid schedule")
	}

	_, _, err = ParseSchedule("@every xyz", time.Now())
	if err == nil {
		t.Error("expected error for invalid @every duration")
	}
}
