package wizard

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/naueramant/munin/internal/cron"
	"github.com/naueramant/munin/internal/utils"
)

// RemoveOptions controls the behavior of the removal process.
type RemoveOptions struct {
	Force      bool // Skip interactive confirmation prompts
	Purge      bool // Remove all configuration, data, deploy keys, and binary
	KeepConfig bool // Keep ~/.munin configuration and data directory
}

// RunRemove executes the uninstallation / removal process.
func RunRemove(opts RemoveOptions) error {
	reader := bufio.NewReader(os.Stdin)

	stat, _ := os.Stdin.Stat()
	isTTY := (stat.Mode() & os.ModeCharDevice) != 0

	if !opts.Force && !isTTY {
		return fmt.Errorf("non-interactive environment detected; pass -y or --force to confirm removal")
	}

	printRemoveBanner()

	if !opts.Force && isTTY {
		fmt.Println("This will remove Munin from your system:")
		fmt.Println("  • Stop and disable systemd user service (munin.service)")
		fmt.Println("  • Remove Munin-managed jobs from native crontab")
		if !opts.KeepConfig {
			fmt.Println("  • Remove configuration and data directory (~/.munin)")
		}
		fmt.Println()

		confirm := prompt(reader, "Are you sure you want to proceed? (y/N)", "N")
		if !strings.EqualFold(confirm, "y") && !strings.EqualFold(confirm, "yes") {
			fmt.Println("Removal canceled.")
			return nil
		}
		fmt.Println()
	}

	// 1. Stop & remove systemd user service
	removeSystemdService()

	// 2. Clear native crontab managed entries
	clearManagedCrontab()

	// 3. Remove configuration and data directory (~/.munin)
	removeConfiguration(reader, isTTY, opts)

	// 4. Remove dedicated SSH deploy key if present (~/.ssh/id_munin_deploy)
	removeDeployKeys(reader, isTTY, opts)

	// 5. Optionally remove binary
	removeExecutable(reader, isTTY, opts)

	printRemoveSummary()
	return nil
}

func printRemoveBanner() {
	fmt.Print(`
  __  __             _       
 |  \/  |_   _ _ __ (_)_ __  
 | |\/| | | | | '_ \| | '_ \ 
 | |  | | |_| | | | | | | | |
 |_|  |_|\__,_|_| |_|_|_| |_|
`)
	fmt.Println(" Munin Removal & Uninstaller")
	fmt.Println(" ===========================")
	fmt.Println()
}

func removeSystemdService() {
	userServicePath := utils.ExpandHome("~/.config/systemd/user/munin.service")
	serviceExists := false
	if _, err := os.Stat(userServicePath); err == nil {
		serviceExists = true
	}

	// Always attempt to stop and disable the user service in case it's registered
	_ = exec.Command("systemctl", "--user", "stop", "munin.service").Run()
	_ = exec.Command("systemctl", "--user", "disable", "munin.service").Run()

	if serviceExists {
		_ = os.Remove(userServicePath)
		// Also clean up standard wants link if still present
		wantsPath := utils.ExpandHome("~/.config/systemd/user/default.target.wants/munin.service")
		_ = os.Remove(wantsPath)

		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		_ = exec.Command("systemctl", "--user", "reset-failed").Run()
		fmt.Println("[✓] Stopped and removed systemd user service (munin.service)")
	} else {
		fmt.Println("[i] No systemd user service file found")
	}

	// Check if a system-wide service unit exists
	sysService := "/etc/systemd/system/munin.service"
	if _, err := os.Stat(sysService); err == nil {
		fmt.Printf("[!] System-wide service detected at %s.\n", sysService)
		fmt.Println("    To remove it, run: sudo systemctl disable --now munin && sudo rm /etc/systemd/system/munin.service")
	}
}

func clearManagedCrontab() {
	removed, err := cron.ClearCrontab()
	if err != nil {
		fmt.Printf("[!] Warning: Failed to clean crontab: %v\n", err)
		return
	}
	if removed {
		fmt.Println("[✓] Removed Munin managed display power and cron jobs from crontab")
	} else {
		fmt.Println("[i] No Munin entries found in crontab")
	}
}

func removeConfiguration(r *bufio.Reader, isTTY bool, opts RemoveOptions) {
	configDir := utils.ExpandHome("~/.munin")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		fmt.Println("[i] No configuration directory (~/.munin) found")
		return
	}

	if opts.KeepConfig {
		fmt.Println("[i] Preserved configuration directory (~/.munin)")
		return
	}

	shouldRemove := opts.Purge || opts.Force
	if !shouldRemove && isTTY {
		ans := prompt(r, "Remove configuration and data directory (~/.munin)? (Y/n)", "Y")
		shouldRemove = strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes")
	}

	if shouldRemove {
		if err := os.RemoveAll(configDir); err != nil {
			fmt.Printf("[!] Warning: Failed to remove %s: %v\n", configDir, err)
		} else {
			fmt.Println("[✓] Removed configuration directory (~/.munin)")
		}
	} else {
		fmt.Println("[i] Kept configuration directory (~/.munin)")
	}
}

func removeDeployKeys(r *bufio.Reader, isTTY bool, opts RemoveOptions) {
	privKey := utils.ExpandHome("~/.ssh/id_munin_deploy")
	pubKey := utils.ExpandHome("~/.ssh/id_munin_deploy.pub")

	_, privErr := os.Stat(privKey)
	_, pubErr := os.Stat(pubKey)

	if os.IsNotExist(privErr) && os.IsNotExist(pubErr) {
		return
	}

	shouldRemove := opts.Purge
	if !shouldRemove && !opts.Force && isTTY {
		ans := prompt(r, "Remove generated SSH deploy key (~/.ssh/id_munin_deploy)? (y/N)", "N")
		shouldRemove = strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes")
	}

	if shouldRemove {
		_ = os.Remove(privKey)
		_ = os.Remove(pubKey)
		fmt.Println("[✓] Removed generated SSH deploy key (~/.ssh/id_munin_deploy)")
	} else {
		fmt.Println("[i] Kept SSH deploy key (~/.ssh/id_munin_deploy)")
	}
}

func removeExecutable(r *bufio.Reader, isTTY bool, opts RemoveOptions) {
	binPath := "/usr/local/bin/munin"

	// Also check current executable if running from another location
	currExec, err := os.Executable()
	targetPath := binPath
	if err == nil && currExec != "" {
		if _, statErr := os.Stat(binPath); os.IsNotExist(statErr) {
			targetPath = currExec
		}
	}

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return
	}

	shouldRemove := opts.Purge
	if !shouldRemove && !opts.Force && isTTY {
		ans := prompt(r, fmt.Sprintf("Remove Munin binary (%s)? (y/N)", targetPath), "N")
		shouldRemove = strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes")
	}

	if shouldRemove {
		if err := os.Remove(targetPath); err != nil {
			if os.IsPermission(err) {
				fmt.Printf("[!] Permission denied removing %s.\n", targetPath)
				fmt.Printf("    To remove the binary, run: sudo rm %s\n", targetPath)
			} else {
				fmt.Printf("[!] Warning: Failed to remove binary %s: %v\n", targetPath, err)
			}
		} else {
			fmt.Printf("[✓] Removed Munin binary (%s)\n", targetPath)
		}
	}
}

func printRemoveSummary() {
	fmt.Println()
	fmt.Println("=== Removal Complete ===")
	fmt.Println("Munin has been successfully removed from this system.")
	fmt.Println()
}

// RemoveDirectoryContent is a helper function to delete a directory tree.
func RemoveDirectoryContent(path string) error {
	return os.RemoveAll(path)
}

// RemoveFileIfExists deletes a file if it exists.
func RemoveFileIfExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	err := os.Remove(path)
	return err == nil, err
}
