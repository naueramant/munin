package utils

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	cronparser "github.com/robfig/cron/v3"
)

var defaultCronParser = cronparser.NewParser(
	cronparser.Minute | cronparser.Hour | cronparser.Dom | cronparser.Month | cronparser.Dow | cronparser.Descriptor,
)

// normalizeTimeOfDay converts an "HH:MM" expression into a daily cron
// expression. Other inputs are returned unchanged.
func normalizeTimeOfDay(expr string) string {
	if strings.Contains(expr, ":") && len(strings.Fields(expr)) == 1 {
		parts := strings.Split(expr, ":")
		if len(parts) == 2 {
			hour, errH := strconv.Atoi(parts[0])
			min, errM := strconv.Atoi(parts[1])
			if errH == nil && errM == nil && hour >= 0 && hour < 24 && min >= 0 && min < 60 {
				return fmt.Sprintf("%d %d * * *", min, hour)
			}
		}
	}
	return expr
}

// ActiveWindowRemaining reports whether `from` currently falls inside a window
// that opened at the most recent trigger of the cron/"HH:MM" expression and
// lasts for `duration`, returning the time left in that window. Duration-style
// expressions (e.g. "5m") have no absolute anchor and return false.
func ActiveWindowRemaining(expr string, duration time.Duration, from time.Time) (time.Duration, bool) {
	if duration <= 0 {
		return 0, false
	}

	schedule, err := defaultCronParser.Parse(normalizeTimeOfDay(strings.TrimSpace(expr)))
	if err != nil {
		return 0, false
	}

	// Earliest trigger strictly after (from - duration); if it is at or before
	// `from`, that window is still open.
	start := schedule.Next(from.Add(-duration))
	if start.After(from) {
		return 0, false
	}

	remaining := duration - from.Sub(start)
	if remaining <= 0 {
		return 0, false
	}
	return remaining, true
}

// ComputeNextCronDelay calculates the duration from `from` until the next execution time matching the cron expression.
// If expr is empty, it uses defaultExpr.
func ComputeNextCronDelay(expr string, defaultExpr string, from time.Time) time.Duration {
	targetExpr := strings.TrimSpace(expr)
	if targetExpr == "" {
		targetExpr = defaultExpr
	}

	// 1. Try standard cron or descriptor (@daily, @hourly, * * * * *, 0 4 * * *)
	schedule, err := defaultCronParser.Parse(targetExpr)
	if err == nil {
		next := schedule.Next(from)
		if next.After(from) {
			return next.Sub(from)
		}
	}

	// 2. Compatibility: check if format is "HH:MM" (e.g. "04:00")
	if strings.Contains(targetExpr, ":") && len(strings.Fields(targetExpr)) == 1 {
		parts := strings.Split(targetExpr, ":")
		if len(parts) == 2 {
			hour, errH := strconv.Atoi(parts[0])
			min, errM := strconv.Atoi(parts[1])
			if errH == nil && errM == nil && hour >= 0 && hour < 24 && min >= 0 && min < 60 {
				target := time.Date(from.Year(), from.Month(), from.Day(), hour, min, 0, 0, from.Location())
				if !target.After(from) {
					target = target.Add(24 * time.Hour)
				}
				return target.Sub(from)
			}
		}
	}

	// 3. Compatibility: check if format is a time duration (e.g. "60s", "5m", "24h")
	if d, errD := time.ParseDuration(targetExpr); errD == nil && d > 0 {
		return d
	}

	slog.Warn("Invalid cron expression, falling back to default", "expr", expr, "default", defaultExpr)
	if defaultSched, errDef := defaultCronParser.Parse(defaultExpr); errDef == nil {
		next := defaultSched.Next(from)
		if next.After(from) {
			return next.Sub(from)
		}
	}

	return time.Minute
}
