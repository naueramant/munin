package doctor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// checkDependencies inspects required and optional external CLI binaries.
func checkDependencies() []CheckResult {
	var results []CheckResult

	// 1. Chromium browser
	results = append(results, checkChromium())

	// 2. cec-client
	results = append(results, checkCecClient())

	// 3. crontab
	results = append(results, checkCrontab())

	// 4. unclutter
	results = append(results, checkUnclutter())

	// 5. ssh
	results = append(results, checkSSH())

	return results
}

func checkChromium() CheckResult {
	candidates := []string{"chromium-browser", "chromium", "google-chrome"}
	for _, bin := range candidates {
		path, err := exec.LookPath(bin)
		if err == nil {
			version := getBinaryVersion(path, "--version")
			return CheckResult{
				Category: CategoryDependencies,
				Name:     "Chromium Browser",
				Status:   StatusOK,
				Message:  fmt.Sprintf("Found %s (%s)", path, version),
			}
		}
	}

	return CheckResult{
		Category: CategoryDependencies,
		Name:     "Chromium Browser",
		Status:   StatusError,
		Message:  "Chromium browser not found in PATH",
		FixHint:  "Install Chromium browser: sudo apt-get install -y chromium-browser || sudo apt-get install -y chromium",
	}
}

func checkCecClient() CheckResult {
	path, err := exec.LookPath("cec-client")
	if err == nil {
		return CheckResult{
			Category: CategoryDependencies,
			Name:     "CEC Client (cec-utils)",
			Status:   StatusOK,
			Message:  fmt.Sprintf("Found %s", path),
		}
	}

	return CheckResult{
		Category: CategoryDependencies,
		Name:     "CEC Client (cec-utils)",
		Status:   StatusWarn,
		Message:  "cec-client not found; HDMI CEC TV power management will be disabled",
		FixHint:  "Install cec-utils: sudo apt-get install -y cec-utils",
	}
}

func checkCrontab() CheckResult {
	path, err := exec.LookPath("crontab")
	if err == nil {
		return CheckResult{
			Category: CategoryDependencies,
			Name:     "Cron Utility (crontab)",
			Status:   StatusOK,
			Message:  fmt.Sprintf("Found %s", path),
		}
	}

	return CheckResult{
		Category: CategoryDependencies,
		Name:     "Cron Utility (crontab)",
		Status:   StatusError,
		Message:  "crontab command not found; scheduled jobs and screen power cannot be installed",
		FixHint:  "Install cron: sudo apt-get install -y cron (or cronie)",
	}
}

func checkUnclutter() CheckResult {
	path, err := exec.LookPath("unclutter")
	if err == nil {
		return CheckResult{
			Category: CategoryDependencies,
			Name:     "Mouse Cursor Hider (unclutter)",
			Status:   StatusOK,
			Message:  fmt.Sprintf("Found %s", path),
		}
	}

	return CheckResult{
		Category: CategoryDependencies,
		Name:     "Mouse Cursor Hider (unclutter)",
		Status:   StatusWarn,
		Message:  "unclutter not found; idle mouse cursor may remain visible on screen",
		FixHint:  "Install unclutter: sudo apt-get install -y unclutter",
	}
}

func checkSSH() CheckResult {
	path, err := exec.LookPath("ssh")
	if err == nil {
		return CheckResult{
			Category: CategoryDependencies,
			Name:     "SSH Client",
			Status:   StatusOK,
			Message:  fmt.Sprintf("Found %s", path),
		}
	}

	return CheckResult{
		Category: CategoryDependencies,
		Name:     "SSH Client",
		Status:   StatusWarn,
		Message:  "ssh command not found; git synchronization via SSH will fail",
		FixHint:  "Install openssh-client: sudo apt-get install -y openssh-client",
	}
}

func getBinaryVersion(binaryPath string, flag string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, flag)
	out, err := cmd.Output()
	if err != nil {
		return "version unknown"
	}
	firstLine := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if firstLine != "" {
		return firstLine
	}
	return "version unknown"
}
