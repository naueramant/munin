package utils

import (
	"testing"
	"time"
)

func TestComputeNextCronDelay(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)

	// 1. Every minute "* * * * *" -> should be 60 seconds (12:01:00)
	delay := ComputeNextCronDelay("* * * * *", "* * * * *", now)
	if delay != time.Minute {
		t.Errorf("expected 1m, got %v", delay)
	}

	// 2. Daily at 13:00 -> "0 13 * * *" -> should be 1 hour
	delay = ComputeNextCronDelay("0 13 * * *", "0 4 * * *", now)
	if delay != time.Hour {
		t.Errorf("expected 1h, got %v", delay)
	}

	// 3. Past time on same day: 11:00 -> should be next day at 11:00 (23 hours)
	delay = ComputeNextCronDelay("0 11 * * *", "0 4 * * *", now)
	if delay != 23*time.Hour {
		t.Errorf("expected 23h, got %v", delay)
	}

	// 4. Time format "14:00" compatibility
	delay = ComputeNextCronDelay("14:00", "0 4 * * *", now)
	if delay != 2*time.Hour {
		t.Errorf("expected 2h, got %v", delay)
	}

	// 5. Duration format "5m" compatibility
	delay = ComputeNextCronDelay("5m", "* * * * *", now)
	if delay != 5*time.Minute {
		t.Errorf("expected 5m, got %v", delay)
	}

	// 6. Empty string uses defaultExpr
	delay = ComputeNextCronDelay("", "0 13 * * *", now)
	if delay != time.Hour {
		t.Errorf("expected default 1h, got %v", delay)
	}
}

func TestActiveWindowRemaining(t *testing.T) {
	// Window: daily 21:19 for 7 minutes (ends 21:26).
	dur := 7 * time.Minute

	// Inside the window at 21:23 -> ~3 minutes remaining.
	now := time.Date(2026, 8, 28, 21, 23, 0, 0, time.UTC)
	remaining, active := ActiveWindowRemaining("19 21 * * *", dur, now)
	if !active || remaining != 3*time.Minute {
		t.Errorf("expected active with 3m remaining, got active=%v remaining=%v", active, remaining)
	}

	// After the window at 21:30 -> not active.
	after := time.Date(2026, 8, 28, 21, 30, 0, 0, time.UTC)
	if _, active := ActiveWindowRemaining("19 21 * * *", dur, after); active {
		t.Errorf("expected not active after window")
	}

	// "HH:MM" form inside the window.
	if remaining, active := ActiveWindowRemaining("21:19", dur, now); !active || remaining != 3*time.Minute {
		t.Errorf("expected HH:MM active with 3m remaining, got active=%v remaining=%v", active, remaining)
	}

	// Duration-style expressions have no absolute anchor.
	if _, active := ActiveWindowRemaining("5m", dur, now); active {
		t.Errorf("expected duration-style expression to be inactive")
	}
}
