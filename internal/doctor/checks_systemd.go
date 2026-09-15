package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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

	// 5. Check that the running binary can be self-replaced by the auto-updater
	results = append(results, checkAutoUpdateWritable(opts))

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

// checkAutoUpdateWritable verifies the running binary's directory is writable by the
// current user, since the auto-updater self-replaces the binary in place and cannot
// do so when running unprivileged (e.g. as a systemd --user service) against a
// root-owned directory such as /usr/local/bin.
func checkAutoUpdateWritable(opts Options) CheckResult {
	execPath, err := os.Executable()
	if err != nil {
		return CheckResult{
			Category: CategorySystemd,
			Name:     "Auto-Update Writable",
			Status:   StatusWarn,
			Message:  "Could not determine running executable path",
		}
	}

	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		resolved = execPath
	}
	dir := filepath.Dir(resolved)

	if isDirWritable(dir) {
		return CheckResult{
			Category: CategorySystemd,
			Name:     "Auto-Update Writable",
			Status:   StatusOK,
			Message:  fmt.Sprintf("Binary directory %s is writable; auto-update can self-replace the binary", dir),
		}
	}

	username := getCurrentUsername()
	res := CheckResult{
		Category: CategorySystemd,
		Name:     "Auto-Update Writable",
		Status:   StatusWarn,
		Message:  fmt.Sprintf("Binary directory %s is not writable by the current user; auto-update will fail", dir),
		Detail:   "Munin usually runs as an unprivileged systemd --user service and cannot self-replace a root-owned binary.",
		Fixable:  true,
		FixHint: fmt.Sprintf(
			"Move the binary to a user-owned directory and symlink it back: "+
				"sudo mkdir -p /opt/munin && sudo install -o %s -g %s -m 0755 %s /opt/munin/munin && sudo ln -sf /opt/munin/munin %s",
			username, username, resolved, execPath),
	}

	if opts.Fix && username != "" {
		if err := migrateBinaryToUserOwnedDir(resolved, execPath, username); err == nil {
			res.FixApplied = true
			res.Status = StatusOK
			res.Message = "Migrated binary to /opt/munin and relinked; auto-update can now self-replace"
		}
	}

	return res
}

// isDirWritable reports whether the current user can create files in dir.
func isDirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".munin-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	_ = os.Remove(name)
	return true
}

// migrateBinaryToUserOwnedDir moves the real binary into /opt/munin (owned by username)
// and replaces linkPath with a symlink to it. Requires sudo since /opt and linkPath's
// directory are typically root-owned.
func migrateBinaryToUserOwnedDir(realPath, linkPath, username string) error {
	const realDir = "/opt/munin"

	if err := exec.Command("sudo", "mkdir", "-p", realDir).Run(); err != nil {
		return err
	}
	if err := exec.Command("sudo", "install", "-o", username, "-g", username, "-m", "0755", realPath, filepath.Join(realDir, "munin")).Run(); err != nil {
		return err
	}
	if err := exec.Command("sudo", "chown", username+":"+username, realDir).Run(); err != nil {
		return err
	}
	return exec.Command("sudo", "ln", "-sf", filepath.Join(realDir, "munin"), linkPath).Run()
}
