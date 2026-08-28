package cron

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/naueramant/munin/internal/config"
)

const (
	BeginMarker = "# --- BEGIN MUNIN MANAGED JOBS [DO NOT EDIT] ---"
	EndMarker   = "# --- END MUNIN MANAGED JOBS ---"

	legacyBeginMarker = "# --- BEGIN MIMIR MANAGED JOBS [DO NOT EDIT] ---"
	legacyEndMarker   = "# --- END MIMIR MANAGED JOBS ---"
)

// UpdateCrontab updates the native user crontab with CEC power schedule and custom jobs.
func UpdateCrontab(power config.PowerConfig, jobs []config.Job) error {
	currentCrontab, err := readCurrentCrontab()
	if err != nil {
		return fmt.Errorf("failed to read current crontab: %w", err)
	}

	newCrontab := GenerateUpdatedCrontab(currentCrontab, power, jobs)

	// If unchanged, don't execute crontab command or log at Info level
	if strings.TrimSpace(currentCrontab) == strings.TrimSpace(newCrontab) {
		slog.Debug("Crontab is already up to date")
		return nil
	}

	if err := installCrontab(newCrontab); err != nil {
		return fmt.Errorf("failed to install updated crontab: %w", err)
	}

	slog.Info("Updated native user crontab with display power schedule and jobs")
	return nil
}

// GenerateUpdatedCrontab merges the munin managed block into an existing crontab string.
func GenerateUpdatedCrontab(existing string, power config.PowerConfig, jobs []config.Job) string {
	cleaned := RemoveManagedBlock(existing)
	managedBlock := GenerateManagedBlock(power, jobs)

	if strings.TrimSpace(managedBlock) == "" {
		return strings.TrimRight(cleaned, "\n") + "\n"
	}

	trimmed := strings.TrimRight(cleaned, "\n")
	if trimmed == "" {
		return managedBlock + "\n"
	}

	return trimmed + "\n\n" + managedBlock + "\n"
}

// GenerateManagedBlock builds the crontab entries inside the marker comments.
func GenerateManagedBlock(power config.PowerConfig, jobs []config.Job) string {
	var lines []string

	dev := power.GetCecDevice()

	if strings.TrimSpace(power.TurnOn) != "" {
		lines = append(lines, fmt.Sprintf("# Display Power: Turn On & set active source (device %d)", dev))
		lines = append(lines, fmt.Sprintf("%s echo 'on %d' | cec-client -s -d 1 && echo 'as' | cec-client -s -d 1 > /dev/null 2>&1", strings.TrimSpace(power.TurnOn), dev))
	}

	if strings.TrimSpace(power.TurnOff) != "" {
		lines = append(lines, fmt.Sprintf("# Display Power: Turn Off / Standby (device %d)", dev))
		lines = append(lines, fmt.Sprintf("%s echo 'standby %d' | cec-client -s -d 1 > /dev/null 2>&1", strings.TrimSpace(power.TurnOff), dev))
	}

	for _, job := range jobs {
		cmdLine := job.GetCommandLine()
		when := strings.TrimSpace(job.When)
		if when != "" && cmdLine != "" {
			lines = append(lines, fmt.Sprintf("%s %s", when, cmdLine))
		}
	}

	if len(lines) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString(BeginMarker + "\n")
	for _, l := range lines {
		buf.WriteString(l + "\n")
	}
	buf.WriteString(EndMarker)

	return buf.String()
}

// RemoveManagedBlock strips out any existing munin (and legacy mimir) managed block from a crontab string.
func RemoveManagedBlock(content string) string {
	content = stripBlock(content, BeginMarker, EndMarker)
	content = stripBlock(content, legacyBeginMarker, legacyEndMarker)
	return content
}

func stripBlock(content, beginMarker, endMarker string) string {
	beginIdx := strings.Index(content, beginMarker)
	if beginIdx == -1 {
		return content
	}

	endIdx := strings.Index(content, endMarker)
	if endIdx == -1 {
		return strings.TrimRight(content[:beginIdx], "\n")
	}

	endIdx += len(endMarker)
	before := content[:beginIdx]
	after := content[endIdx:]

	return strings.TrimRight(strings.TrimRight(before, "\n")+"\n"+strings.TrimLeft(after, "\n"), "\n")
}

func readCurrentCrontab() (string, error) {
	cmd := exec.Command("crontab", "-l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		// "no crontab for <user>" is normal when empty
		if strings.Contains(strings.ToLower(outStr), "no crontab") {
			return "", nil
		}
		// If crontab command is not found or other error
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(outStr), err)
	}

	return string(out), nil
}

func installCrontab(content string) error {
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}
