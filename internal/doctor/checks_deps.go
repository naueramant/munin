package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// getChromiumCandidates returns candidate executable names and paths to search
// for Chromium-compatible browsers, aligned with chromedp's locator logic and standard distro paths.
func getChromiumCandidates() []string {
	var candidates []string

	// 1. Explicit environment variable overrides
	for _, env := range []string{"CHROME_BIN", "CHROMIUM_PATH"} {
		if val := os.Getenv(env); val != "" {
			candidates = append(candidates, val)
		}
	}

	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"chromium",
			"google-chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		)
	case "windows":
		candidates = append(candidates,
			"chrome",
			"chrome.exe",
			filepath.Join(os.Getenv("ProgramFiles"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Chromium\Application\chrome.exe`),
		)
	default:
		// Unix-like / Linux:
		// Aligned with and extends chromedp's search locations so doctor reflects actual runtime availability.
		candidates = append(candidates,
			"chromium-browser",
			"chromium",
			"google-chrome-stable",
			"google-chrome",
			"google-chrome-beta",
			"google-chrome-unstable",
			"chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"/snap/bin/chromium-browser",
			"/var/lib/snapd/snap/bin/chromium",
			"/usr/local/bin/chrome",
			"headless_shell",
			"headless-shell",
		)
	}

	return candidates
}

func checkChromium() CheckResult {
	for _, bin := range getChromiumCandidates() {
		if bin == "" {
			continue
		}
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
		Message:  "Chromium browser not found in PATH or standard locations",
		FixHint:  "Install Chromium browser: sudo apt-get install -y chromium-browser || sudo apt-get install -y chromium (or google-chrome-stable)",
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
