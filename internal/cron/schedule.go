package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseSchedule parses a human-readable schedule string into a next-run time.
// Supported formats:
//
//	""           → one-shot (no next run)
//	"@once"      → one-shot (same as empty)
//	"@every 30m" → every 30 minutes
//	"@every 2h"  → every 2 hours
//	"@every 1d"  → every 1 day
//	"@hourly"    → every 1 hour
//	"@daily"     → every 24 hours (midnight)
//	"@weekly"    → every 7 days
//	"@monthly"   → 1st of next month
func ParseSchedule(schedule string, from time.Time) (next time.Time, isOneShot bool, err error) {
	schedule = strings.TrimSpace(schedule)

	// Empty or @once → one-shot
	if schedule == "" || schedule == "@once" {
		return time.Time{}, true, nil
	}

	// @every Xm / Xh / Xd
	if strings.HasPrefix(schedule, "@every ") {
		dur, err := parseDuration(strings.TrimPrefix(schedule, "@every "))
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid @every duration: %w", err)
		}
		return from.Add(dur), false, nil
	}

	// Named schedules
	switch strings.ToLower(schedule) {
	case "@hourly":
		return from.Add(time.Hour), false, nil
	case "@daily":
		// Next midnight
		y, m, d := from.Date()
		next = time.Date(y, m, d+1, 0, 0, 0, 0, from.Location())
		return next, false, nil
	case "@weekly":
		// Next Monday midnight
		y, m, d := from.Date()
		daysUntilMon := (8 - int(from.Weekday())) % 7
		if daysUntilMon == 0 {
			daysUntilMon = 7
		}
		next = time.Date(y, m, d+daysUntilMon, 0, 0, 0, 0, from.Location())
		return next, false, nil
	case "@monthly":
		// Next 1st of month
		y, m, _ := from.Date()
		next = time.Date(y, m+1, 1, 0, 0, 0, 0, from.Location())
		return next, false, nil
	}

	// Try standard 5-field cron: min hour day month weekday
	// Simplified: only support "*/N" in one field for now
	parts := strings.Fields(schedule)
	if len(parts) == 5 {
		return parseCronExpr(parts, from)
	}

	return time.Time{}, false, fmt.Errorf("unsupported schedule format: %q (use @every Xm, @hourly, @daily, @weekly, @monthly, or 5-field cron)", schedule)
}

// parseDuration parses "30m", "2h", "1d" into time.Duration.
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// parseCronExpr handles basic 5-field cron expressions.
// Supports: exact values, */N (every N), and * (any).
func parseCronExpr(fields []string, from time.Time) (time.Time, bool, error) {
	minField := fields[0]
	hourField := fields[1]

	// Parse minute
	minStep := 0
	if strings.HasPrefix(minField, "*/") {
		n, err := strconv.Atoi(strings.TrimPrefix(minField, "*/"))
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid cron minute: %s", minField)
		}
		minStep = n
	} else if minField != "*" {
		n, err := strconv.Atoi(minField)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid cron minute: %s", minField)
		}
		// Exact minute: next occurrence today or tomorrow
		next := time.Date(from.Year(), from.Month(), from.Day(), from.Hour(), n, 0, 0, from.Location())
		if hourField != "*" {
			h, err := strconv.Atoi(hourField)
			if err == nil {
				next = time.Date(from.Year(), from.Month(), from.Day(), h, n, 0, 0, from.Location())
			}
		}
		if !next.After(from) {
			next = next.Add(24 * time.Hour)
		}
		return next, false, nil
	}

	// */N minute step
	if minStep > 0 {
		currentMin := from.Minute()
		nextMin := ((currentMin / minStep) + 1) * minStep
		next := from.Truncate(time.Minute).Add(time.Duration(nextMin-currentMin) * time.Minute)
		if !next.After(from) {
			next = next.Add(time.Duration(minStep) * time.Minute)
		}
		return next, false, nil
	}

	// Wildcard: default to hourly
	next := from.Truncate(time.Minute).Add(time.Minute)
	if !next.After(from) {
		next = next.Add(time.Minute)
	}
	return next, false, nil
}
