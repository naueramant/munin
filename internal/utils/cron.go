package utils

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	cronparser "github.com/robfig/cron/v3"
)

var defaultCronParser = cronparser.NewParser(
	cronparser.Minute | cronparser.Hour | cronparser.Dom | cronparser.Month | cronparser.Dow | cronparser.Descriptor,
)

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
