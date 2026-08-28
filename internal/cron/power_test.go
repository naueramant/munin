package cron

import (
	"testing"
	"time"

	"github.com/naueramant/munin/internal/config"
)

func TestNormalizeSchedule(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"10:00", "0 10 * * *"},
		{"07:30", "30 7 * * *"},
		{"00:00", "0 0 * * *"},
		{"23:59", "59 23 * * *"},
		{"0 7 * * 1-5", "0 7 * * 1-5"},
		{"@daily", "@daily"},
		{"", ""},
		{"invalid", "invalid"},
	}

	for _, tc := range tests {
		got := NormalizeSchedule(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeSchedule(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestShouldScreenBeOff_UserScenario(t *testing.T) {
	// The user's specific scenario:
	// Screen turned off at 10:00, reboot at 11:00, screen on at 18:00.
	power := config.PowerConfig{
		ScreenOff: "10:00",
		Reboot:    "11:00",
		ScreenOn:  "18:00",
	}

	loc := time.UTC
	baseDate := time.Date(2026, 8, 28, 0, 0, 0, 0, loc)

	// 1. At 11:05 (right after 11:00 reboot)
	// 10:00 screen_off has run. 18:00 screen_on hasn't run yet.
	// Screen should be OFF!
	now1 := baseDate.Add(11*time.Hour + 5*time.Minute)
	if !ShouldScreenBeOff(power, now1) {
		t.Errorf("at 11:05, expected ShouldScreenBeOff to be true (screen off ran at 10:00, screen on hasn't run yet)")
	}

	// 2. At 18:05 (after screen_on at 18:00)
	// 18:00 screen_on has run! Screen should be ON.
	now2 := baseDate.Add(18*time.Hour + 5*time.Minute)
	if ShouldScreenBeOff(power, now2) {
		t.Errorf("at 18:05, expected ShouldScreenBeOff to be false (screen on already ran at 18:00)")
	}

	// 3. At 09:30 (before screen_off at 10:00)
	// Last event was yesterday's screen_on at 18:00. Screen should be ON.
	now3 := baseDate.Add(9*time.Hour + 30*time.Minute)
	if ShouldScreenBeOff(power, now3) {
		t.Errorf("at 09:30, expected ShouldScreenBeOff to be false (screen off hasn't run yet today)")
	}
}

func TestShouldScreenBeOff_WeekdaySchedule(t *testing.T) {
	// Mon-Fri screen: On at 07:00, Off at 19:00.
	power := config.PowerConfig{
		ScreenOn:  "0 7 * * 1-5",
		ScreenOff: "0 19 * * 1-5",
		Reboot:    "0 3 * * *",
	}

	loc := time.UTC

	// 2026-08-28 is a Friday
	// At 20:00 Friday: screen was turned off at 19:00 Friday.
	friEvening := time.Date(2026, 8, 28, 20, 0, 0, 0, loc)
	if !ShouldScreenBeOff(power, friEvening) {
		t.Errorf("expected Friday evening to be OFF")
	}

	// Saturday 2026-08-29 at 03:05 AM (after nightly 03:00 reboot):
	// Last event was Friday 19:00 (off). Screen should remain OFF throughout the weekend!
	satNight := time.Date(2026, 8, 29, 3, 5, 0, 0, loc)
	if !ShouldScreenBeOff(power, satNight) {
		t.Errorf("expected Saturday night to be OFF after reboot")
	}

	// Sunday 2026-08-30 at 14:00:
	// Screen should still be OFF!
	sunAfternoon := time.Date(2026, 8, 30, 14, 0, 0, 0, loc)
	if !ShouldScreenBeOff(power, sunAfternoon) {
		t.Errorf("expected Sunday afternoon to be OFF")
	}

	// Monday 2026-08-31 at 06:30 AM (before 07:00 on):
	// Screen should still be OFF!
	monMorningPre := time.Date(2026, 8, 31, 6, 30, 0, 0, loc)
	if !ShouldScreenBeOff(power, monMorningPre) {
		t.Errorf("expected Monday 06:30 to be OFF")
	}

	// Monday 2026-08-31 at 07:15 AM (after 07:00 on):
	// Screen should now be ON!
	monMorningPost := time.Date(2026, 8, 31, 7, 15, 0, 0, loc)
	if ShouldScreenBeOff(power, monMorningPost) {
		t.Errorf("expected Monday 07:15 to be ON")
	}
}

func TestShouldScreenBeOff_OnlyOffConfigured(t *testing.T) {
	power := config.PowerConfig{
		ScreenOff: "10:00",
	}

	loc := time.UTC
	baseDate := time.Date(2026, 8, 28, 0, 0, 0, 0, loc)

	// At 11:00 (after 10:00 off)
	now := baseDate.Add(11 * time.Hour)
	if !ShouldScreenBeOff(power, now) {
		t.Errorf("expected ShouldScreenBeOff to be true when only screen_off is configured and has passed")
	}
}

func TestShouldScreenBeOff_OnlyOnConfigured(t *testing.T) {
	power := config.PowerConfig{
		ScreenOn: "07:00",
	}

	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	if ShouldScreenBeOff(power, now) {
		t.Errorf("expected ShouldScreenBeOff to be false when only screen_on is configured")
	}
}

func TestShouldScreenBeOff_Empty(t *testing.T) {
	power := config.PowerConfig{}
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	if ShouldScreenBeOff(power, now) {
		t.Errorf("expected ShouldScreenBeOff to be false when power is empty")
	}
}
