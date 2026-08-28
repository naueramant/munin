package doctor

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
)

// checkHardware inspects display server settings, user group permissions, and CEC hardware nodes.
func checkHardware() []CheckResult {
	var results []CheckResult

	// 1. Display environment
	results = append(results, checkDisplayEnv())

	// 2. User groups (video, render, input)
	results = append(results, checkUserGroups())

	// 3. CEC hardware device nodes
	results = append(results, checkCECDevices())

	return results
}

func checkDisplayEnv() CheckResult {
	disp := os.Getenv("DISPLAY")
	wayland := os.Getenv("WAYLAND_DISPLAY")

	if disp != "" && wayland != "" {
		return CheckResult{
			Category: CategoryHardware,
			Name:     "Display Server",
			Status:   StatusOK,
			Message:  fmt.Sprintf("DISPLAY=%s, WAYLAND_DISPLAY=%s", disp, wayland),
		}
	}
	if disp != "" {
		return CheckResult{
			Category: CategoryHardware,
			Name:     "Display Server",
			Status:   StatusOK,
			Message:  fmt.Sprintf("DISPLAY=%s", disp),
		}
	}
	if wayland != "" {
		return CheckResult{
			Category: CategoryHardware,
			Name:     "Display Server",
			Status:   StatusOK,
			Message:  fmt.Sprintf("WAYLAND_DISPLAY=%s", wayland),
		}
	}

	return CheckResult{
		Category: CategoryHardware,
		Name:     "Display Server",
		Status:   StatusWarn,
		Message:  "Neither DISPLAY nor WAYLAND_DISPLAY is set in current environment",
		Detail:   "Chromium kiosk requires a display server. Munin defaults to DISPLAY=:0 if specified in agent.yaml or systemd service.",
		FixHint:  "Ensure X11 or Wayland is running and export DISPLAY=:0",
	}
}

func checkUserGroups() CheckResult {
	cur, err := user.Current()
	if err != nil {
		return CheckResult{
			Category: CategoryHardware,
			Name:     "Hardware Permissions",
			Status:   StatusWarn,
			Message:  "Unable to determine current user group memberships",
		}
	}

	gids, err := cur.GroupIds()
	if err != nil {
		return CheckResult{
			Category: CategoryHardware,
			Name:     "Hardware Permissions",
			Status:   StatusWarn,
			Message:  "Unable to retrieve group IDs for current user",
		}
	}

	userGroupNames := make(map[string]bool)
	for _, gid := range gids {
		if grp, err := user.LookupGroupId(gid); err == nil {
			userGroupNames[grp.Name] = true
		}
	}

	needed := []string{"video", "render", "input"}
	var missing []string
	var present []string

	for _, req := range needed {
		// Only check if group exists on host system
		if _, err := user.LookupGroup(req); err == nil {
			if userGroupNames[req] {
				present = append(present, req)
			} else {
				missing = append(missing, req)
			}
		}
	}

	if len(missing) == 0 {
		return CheckResult{
			Category: CategoryHardware,
			Name:     "Hardware Permissions",
			Status:   StatusOK,
			Message:  fmt.Sprintf("User belongs to required hardware groups (%s)", formatList(present)),
		}
	}

	return CheckResult{
		Category: CategoryHardware,
		Name:     "Hardware Permissions",
		Status:   StatusWarn,
		Message:  fmt.Sprintf("User missing recommended hardware groups: %s", formatList(missing)),
		Detail:   "Access to 'video' and 'render' is required for hardware GPU acceleration and CEC device control.",
		FixHint:  fmt.Sprintf("sudo usermod -aG %s %s (then log out and back in)", formatList(missing), cur.Username),
	}
}

func checkCECDevices() CheckResult {
	matches, _ := filepath.Glob("/dev/cec*")
	if _, err := os.Stat("/dev/vchiq"); err == nil {
		matches = append(matches, "/dev/vchiq")
	}

	if len(matches) == 0 {
		return CheckResult{
			Category: CategoryHardware,
			Name:     "HDMI CEC Device",
			Status:   StatusInfo,
			Message:  "No CEC device nodes detected (/dev/cec*, /dev/vchiq)",
			Detail:   "Normal on virtual machines or PCs without HDMI-CEC adapters. On Raspberry Pi, ensure HDMI cable connects to CEC-capable display.",
		}
	}

	// Check permissions on detected devices
	var accessible []string
	var restricted []string

	for _, dev := range matches {
		if syscall.Access(dev, syscall.O_RDWR) == nil {
			accessible = append(accessible, dev)
		} else {
			restricted = append(restricted, dev)
		}
	}

	if len(restricted) > 0 {
		return CheckResult{
			Category: CategoryHardware,
			Name:     "HDMI CEC Device",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("CEC devices detected but lack write permissions: %s", formatList(restricted)),
			Detail:   "Ensure current user is in the 'video' or 'dialout' group to communicate with CEC controllers.",
			FixHint:  "sudo usermod -aG video $USER",
		}
	}

	return CheckResult{
		Category: CategoryHardware,
		Name:     "HDMI CEC Device",
		Status:   StatusOK,
		Message:  fmt.Sprintf("CEC device accessible (%s)", formatList(accessible)),
	}
}

func formatList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	res := items[0]
	for i := 1; i < len(items); i++ {
		res += ", " + items[i]
	}
	return res
}
