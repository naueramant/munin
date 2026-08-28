package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/naueramant/munin/internal/utils"
)

// checkSystemd inspects systemd user services, linger status, and host cron daemon.
func checkSystemd(opts Options) []CheckResult {
	var results []CheckResult

	// 1. Check systemd user manager availability
	results = append(results, checkSystemdUserAvailable())

	// 2. Check munin.service existence and state
	results = append(results, checkMuninService(opts)...)

	// 3. Check user lingering
	results = append(results, checkUserLingering(opts))

	// 4. Check host cron service
	results = append(results, checkHostCronService())

	return results
}

func checkSystemdUserAvailable() CheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-system-running")
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))

	// "running", "degraded", "initializing" all mean user manager is responsive
	if err == nil || outStr == "running" || outStr == "degraded" {
		return CheckResult{
			Category: CategorySystemd,
			Name:     "Systemd User Manager",
			Status:   StatusOK,
			Message:  fmt.Sprintf("User systemd instance responsive (state: %s)", outStr),
		}
	}

	return CheckResult{
		Category: CategorySystemd,
		Name:     "Systemd User Manager",
		Status:   StatusWarn,
		Message:  "Systemd user manager is not responsive or not running",
		Detail:   fmt.Sprintf("Output: %s", outStr),
		FixHint:  "Ensure systemd user session is running (check XDG_RUNTIME_DIR and DBUS_SESSION_BUS_ADDRESS)",
	}
}

func checkMuninService(opts Options) []CheckResult {
	var results []CheckResult

	servicePath := utils.ExpandHome("~/.config/systemd/user/munin.service")
	_, err := os.Stat(servicePath)
	if os.IsNotExist(err) {
		results = append(results, CheckResult{
			Category: CategorySystemd,
			Name:     "Munin Service Unit",
			Status:   StatusWarn,
			Message:  "Service unit not installed (~/.config/systemd/user/munin.service missing)",
			FixHint:  "Run `munin init` to generate and install the user service",
		})
		return results
	}

	results = append(results, CheckResult{
		Category: CategorySystemd,
		Name:     "Munin Service Unit",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Installed at %s", servicePath),
	})

	// Check if enabled
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmdEnabled := exec.CommandContext(ctx, "systemctl", "--user", "is-enabled", "munin.service")
	outEnabled, _ := cmdEnabled.CombinedOutput()
	enabledStr := strings.TrimSpace(string(outEnabled))

	if enabledStr == "enabled" {
		results = append(results, CheckResult{
			Category: CategorySystemd,
			Name:     "Munin Service Enabled",
			Status:   StatusOK,
			Message:  "munin.service is enabled to launch on boot/session start",
		})
	} else {
		res := CheckResult{
			Category: CategorySystemd,
			Name:     "Munin Service Enabled",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("munin.service is %s (not enabled to launch automatically)", enabledStr),
			Fixable:  true,
			FixHint:  "systemctl --user enable munin.service",
		}
		if opts.Fix {
			fixCmd := exec.Command("systemctl", "--user", "enable", "munin.service")
			if fixErr := fixCmd.Run(); fixErr == nil {
				res.FixApplied = true
				res.Status = StatusOK
				res.Message = "munin.service has been enabled"
			}
		}
		results = append(results, res)
	}

	// Check if active
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	cmdActive := exec.CommandContext(ctx2, "systemctl", "--user", "is-active", "munin.service")
	outActive, _ := cmdActive.CombinedOutput()
	activeStr := strings.TrimSpace(string(outActive))

	if activeStr == "active" {
		results = append(results, CheckResult{
			Category: CategorySystemd,
			Name:     "Munin Service Running",
			Status:   StatusOK,
			Message:  "munin.service is currently active and running",
		})
	} else {
		results = append(results, CheckResult{
			Category: CategorySystemd,
			Name:     "Munin Service Running",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("munin.service is not running (state: %s)", activeStr),
			Detail:   "View logs with: journalctl --user -u munin -e",
			FixHint:  "systemctl --user start munin.service",
		})
	}

	return results
}

func checkUserLingering(opts Options) CheckResult {
	username := getCurrentUsername()
	if username == "" {
		return CheckResult{
			Category: CategorySystemd,
			Name:     "User Lingering",
			Status:   StatusWarn,
			Message:  "Could not determine current username to check linger status",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "loginctl", "show-user", username, "--property=Linger")
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))

	if err == nil && strings.Contains(outStr, "Linger=yes") {
		return CheckResult{
			Category: CategorySystemd,
			Name:     "User Lingering",
			Status:   StatusOK,
			Message:  fmt.Sprintf("User lingering enabled for '%s'", username),
		}
	}

	res := CheckResult{
		Category: CategorySystemd,
		Name:     "User Lingering",
		Status:   StatusWarn,
		Message:  fmt.Sprintf("User lingering is disabled for '%s'", username),
		Detail:   "Without lingering, systemd user services will not start on boot without an interactive login session.",
		Fixable:  true,
		FixHint:  fmt.Sprintf("sudo loginctl enable-linger %s", username),
	}

	if opts.Fix {
		// Attempt to enable linger
		fixCmd := exec.Command("loginctl", "enable-linger", username)
		if fixErr := fixCmd.Run(); fixErr == nil {
			res.FixApplied = true
			res.Status = StatusOK
			res.Message = fmt.Sprintf("User lingering enabled for '%s'", username)
		}
	}

	return res
}

func checkHostCronService() CheckResult {
	// Check cron or cronie service
	for _, svc := range []string{"cron", "cronie"} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		cmd := exec.CommandContext(ctx, "systemctl", "is-active", svc)
		out, err := cmd.CombinedOutput()
		cancel()

		if err == nil && strings.TrimSpace(string(out)) == "active" {
			return CheckResult{
				Category: CategorySystemd,
				Name:     "Host Cron Service",
				Status:   StatusOK,
				Message:  fmt.Sprintf("Service '%s' is active", svc),
			}
		}
	}

	return CheckResult{
		Category: CategorySystemd,
		Name:     "Host Cron Service",
		Status:   StatusWarn,
		Message:  "Host cron service (cron/cronie) is not active",
		Detail:   "Native crontab display power and scheduled jobs will not execute unless cron daemon runs.",
		FixHint:  "sudo systemctl enable --now cron",
	}
}

func getCurrentUsername() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if cur, err := user.Current(); err == nil && cur.Username != "" {
		return cur.Username
	}
	return ""
}
