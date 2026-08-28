package cron

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/naueramant/munin/internal/config"
	cronparser "github.com/robfig/cron/v3"
)

var defaultCronParser = cronparser.NewParser(
	cronparser.Minute | cronparser.Hour | cronparser.Dom | cronparser.Month | cronparser.Dow | cronparser.Descriptor,
)

// NormalizeSchedule standardizes time strings into valid 5-part cron expressions.
// Supports "HH:MM" (e.g. "10:00" -> "0 10 * * *") as well as standard cron expressions and @descriptors.
func NormalizeSchedule(expr string) string {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return ""
	}

	// Compatibility: check if format is "HH:MM" (e.g. "10:00", "07:30")
	if strings.Contains(trimmed, ":") && len(strings.Fields(trimmed)) == 1 {
		parts := strings.Split(trimmed, ":")
		if len(parts) == 2 {
			hour, errH := strconv.Atoi(parts[0])
			min, errM := strconv.Atoi(parts[1])
			if errH == nil && errM == nil && hour >= 0 && hour < 24 && min >= 0 && min < 60 {
				return fmt.Sprintf("%d %d * * *", min, hour)
			}
		}
	}

	return trimmed
}

// ShouldScreenBeOff determines whether the screen should currently be turned off according to the power schedule.
// Returns true if screen_off was scheduled and executed more recently in the past than screen_on (or screen_on hasn't run yet).
func ShouldScreenBeOff(power config.PowerConfig, now time.Time) bool {
	offRaw := power.GetScreenOff()
	if offRaw == "" {
		return false
	}

	offExpr := NormalizeSchedule(offRaw)
	schedOff, err := defaultCronParser.Parse(offExpr)
	if err != nil {
		slog.Warn("Failed to parse screen_off cron schedule", "expr", offRaw, "error", err)
		return false
	}

	lastOff := findLastRun(schedOff, now)
	if lastOff.IsZero() {
		return false
	}

	onRaw := power.GetScreenOn()
	if onRaw == "" {
		// Screen off was scheduled and has executed in the past, and no screen on is scheduled
		return true
	}

	onExpr := NormalizeSchedule(onRaw)
	schedOn, err := defaultCronParser.Parse(onExpr)
	if err != nil {
		slog.Warn("Failed to parse screen_on cron schedule", "expr", onRaw, "error", err)
		return true
	}

	lastOn := findLastRun(schedOn, now)
	if lastOn.IsZero() {
		// Screen off has run, but screen on has never run in the lookback window
		return true
	}

	// If screen_off happened more recently than screen_on, the screen should currently be off
	return lastOff.After(lastOn)
}

// findLastRun finds the most recent execution time of schedule strictly before or equal to now within a 35-day window.
func findLastRun(schedule cronparser.Schedule, now time.Time) time.Time {
	start := now.Add(-35 * 24 * time.Hour)
	var last time.Time
	cur := start
	for {
		next := schedule.Next(cur)
		if next.After(now) {
			break
		}
		// Guard against infinite loop if Next doesn't advance
		if !next.After(cur) {
			break
		}
		last = next
		cur = next
	}
	return last
}

// StandbyScreen sends the CEC standby command to the specified device.
func StandbyScreen(cecDevice int) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo 'standby %d' | cec-client -s -d 1 > /dev/null 2>&1", cecDevice))
	return cmd.Run()
}

// ReconcilePowerState checks if the screen is scheduled to be off. If so, it executes CEC standby
// asynchronously with retries to account for TV boot / HDMI handshake delays after reboot.
func ReconcilePowerState(power config.PowerConfig) {
	if !ShouldScreenBeOff(power, time.Now()) {
		return
	}

	dev := power.GetCecDevice()
	slog.Info("Screen is scheduled to be off; enforcing TV standby after boot", "cec_device", dev)

	go func() {
		// Wait 5 seconds after boot for HDMI/CEC bus to settle
		time.Sleep(5 * time.Second)
		if err := StandbyScreen(dev); err != nil {
			slog.Warn("Failed to send initial CEC standby command", "cec_device", dev, "error", err)
		}

		// Re-send standby at 15 seconds to catch slow-booting TVs
		time.Sleep(10 * time.Second)
		_ = StandbyScreen(dev)
	}()
}

// ReconcilePowerStateSync performs synchronous reconciliation of the screen power state.
// Returns whether the screen was determined to be off, and any execution error.
func ReconcilePowerStateSync(power config.PowerConfig) (bool, error) {
	if !ShouldScreenBeOff(power, time.Now()) {
		return false, nil
	}

	dev := power.GetCecDevice()
	slog.Info("Screen is scheduled to be off; enforcing TV standby", "cec_device", dev)
	err := StandbyScreen(dev)
	return true, err
}
