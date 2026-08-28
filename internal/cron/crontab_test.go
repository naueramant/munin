package cron

import (
	"strings"
	"testing"

	"github.com/naueramant/munin/internal/config"
)

func TestGenerateManagedBlock(t *testing.T) {
	power := config.PowerConfig{
		TurnOn:  "0 7 * * 1-5",
		TurnOff: "0 19 * * 1-5",
	}

	jobs := []config.Job{
		{
			When:    "0 3 * * *",
			Command: "sudo reboot",
		},
	}

	block := GenerateManagedBlock(power, jobs)

	if !strings.Contains(block, BeginMarker) {
		t.Errorf("missing begin marker")
	}
	if !strings.Contains(block, EndMarker) {
		t.Errorf("missing end marker")
	}
	if !strings.Contains(block, "0 7 * * 1-5 echo 'on 0' | cec-client") {
		t.Errorf("missing turn on CEC command: %s", block)
	}
	if !strings.Contains(block, "0 19 * * 1-5 echo 'standby 0' | cec-client") {
		t.Errorf("missing turn off CEC command: %s", block)
	}
	if !strings.Contains(block, "0 3 * * * sudo reboot") {
		t.Errorf("missing custom job command: %s", block)
	}
}

func TestGenerateManagedBlock_PowerOptions(t *testing.T) {
	dev := 1
	power := config.PowerConfig{
		ScreenOn:  "07:00",
		ScreenOff: "10:00",
		Reboot:    "11:00",
		PowerOff:  "0 23 * * 5",
		CecDevice: &dev,
	}

	block := GenerateManagedBlock(power, nil)

	if !strings.Contains(block, "0 7 * * * echo 'on 1' | cec-client") {
		t.Errorf("expected normalized screen_on command, got: %s", block)
	}
	if !strings.Contains(block, "0 10 * * * echo 'standby 1' | cec-client") {
		t.Errorf("expected normalized screen_off command, got: %s", block)
	}
	if !strings.Contains(block, "0 11 * * * sudo reboot") {
		t.Errorf("expected normalized reboot command, got: %s", block)
	}
	if !strings.Contains(block, "0 23 * * 5 sudo poweroff") {
		t.Errorf("expected power_off command, got: %s", block)
	}
}

func TestGenerateUpdatedCrontab(t *testing.T) {
	existing := `# Existing user job
0 12 * * * /home/pi/backup.sh

` + BeginMarker + `
0 6 * * * old_job
` + EndMarker + `

# Another existing user job
0 1 * * * /home/pi/cleanup.sh
`

	power := config.PowerConfig{
		TurnOn: "0 8 * * *",
	}

	jobs := []config.Job{
		{
			When:    "0 2 * * *",
			Command: "echo test",
		},
	}

	updated := GenerateUpdatedCrontab(existing, power, jobs)

	if strings.Contains(updated, "old_job") {
		t.Errorf("expected old_job to be removed, got:\n%s", updated)
	}
	if !strings.Contains(updated, "/home/pi/backup.sh") {
		t.Errorf("expected existing backup.sh to be preserved, got:\n%s", updated)
	}
	if !strings.Contains(updated, "/home/pi/cleanup.sh") {
		t.Errorf("expected existing cleanup.sh to be preserved, got:\n%s", updated)
	}
	if !strings.Contains(updated, "0 8 * * * echo 'on 0' | cec-client") {
		t.Errorf("expected new CEC power job, got:\n%s", updated)
	}
	if !strings.Contains(updated, "0 2 * * * echo test") {
		t.Errorf("expected new custom job, got:\n%s", updated)
	}
}

func TestRemoveManagedBlock(t *testing.T) {
	existing := `# Existing
1 2 3 4 5 test
` + BeginMarker + `
foo bar
` + EndMarker + `
# Keep this
5 4 3 2 1 test2`

	cleaned := RemoveManagedBlock(existing)

	if strings.Contains(cleaned, BeginMarker) || strings.Contains(cleaned, "foo bar") {
		t.Errorf("managed block not removed: %s", cleaned)
	}
	if !strings.Contains(cleaned, "1 2 3 4 5 test") || !strings.Contains(cleaned, "5 4 3 2 1 test2") {
		t.Errorf("existing jobs removed: %s", cleaned)
	}
}

func TestRemoveLegacyMimirManagedBlock(t *testing.T) {
	existing := `# Existing
1 2 3 4 5 test
` + legacyBeginMarker + `
legacy mimir job
` + legacyEndMarker + `
# Keep this
5 4 3 2 1 test2`

	cleaned := RemoveManagedBlock(existing)

	if strings.Contains(cleaned, legacyBeginMarker) || strings.Contains(cleaned, "legacy mimir job") {
		t.Errorf("legacy mimir block not removed: %s", cleaned)
	}
	if !strings.Contains(cleaned, "1 2 3 4 5 test") || !strings.Contains(cleaned, "5 4 3 2 1 test2") {
		t.Errorf("existing jobs removed: %s", cleaned)
	}
}

func TestRemoveManagedBlock_OnlyManagedBlock(t *testing.T) {
	existing := BeginMarker + `
0 7 * * 1-5 echo 'on 0' | cec-client
` + EndMarker + "\n"

	cleaned := RemoveManagedBlock(existing)
	if strings.TrimSpace(cleaned) != "" {
		t.Errorf("expected empty crontab, got: %q", cleaned)
	}
}

