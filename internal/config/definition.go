package config

import (
	"fmt"
	"strings"
	"time"
)

// AgentConfig defines the host-level settings for the munin agent.
type AgentConfig struct {
	Mode       string        `yaml:"mode,omitempty"`        // "git" or "local"
	LogLevel   string        `yaml:"log_level,omitempty"`   // "debug", "info", "warn", "error"
	Git        GitConfig     `yaml:"git,omitempty"`         // Git sync configuration
	ScreenPath string        `yaml:"screen_path,omitempty"` // Path to screen.yaml when in local mode
	Update     UpdateConfig  `yaml:"update,omitempty"`      // GitHub release auto-updater
	Display    DisplayConfig `yaml:"display,omitempty"`     // Display environment settings
}

// GitConfig holds Git repository and synchronization settings.
type GitConfig struct {
	Repo      string `yaml:"repo"`                 // e.g. git@github.com:user/repo.git or https://...
	DeployKey string `yaml:"deploy_key,omitempty"` // Path to private SSH key for authentication
	Branch    string `yaml:"branch,omitempty"`     // Git branch, defaults to "main"
	Subdir    string `yaml:"subdir,omitempty"`     // Subdirectory in repo containing screen.yaml
	Schedule  string `yaml:"schedule,omitempty"`   // Cron expression for sync (defaults to "* * * * *" - every minute)
	TargetDir string `yaml:"target_dir,omitempty"` // Local path to clone into, defaults to "~/.munin/repo"

	Interval string `yaml:"interval,omitempty"` // Deprecated alias for schedule
}

// GetSchedule returns the cron expression or default "* * * * *" (every minute).
func (g GitConfig) GetSchedule() string {
	if g.Schedule != "" {
		return g.Schedule
	}
	if g.Interval != "" {
		return g.Interval
	}
	return "* * * * *"
}

// GetInterval parses the Interval/Schedule string or returns default 1m.
func (g GitConfig) GetInterval() time.Duration {
	str := g.Interval
	if str == "" {
		str = g.Schedule
	}
	if str == "" {
		return time.Minute
	}
	d, err := time.ParseDuration(str)
	if err != nil || d <= 0 {
		return time.Minute
	}
	return d
}

// UpdateConfig holds GitHub releases auto-updater settings.
type UpdateConfig struct {
	Enabled  *bool  `yaml:"enabled,omitempty"`  // Whether auto-update is enabled (defaults to true)
	Repo     string `yaml:"repo,omitempty"`     // GitHub repo: owner/repo (defaults to "naueramant/munin")
	Schedule string `yaml:"schedule,omitempty"` // Cron expression for update check (defaults to "0 4 * * *" - daily at 04:00)

	When     string `yaml:"when,omitempty"`     // Deprecated alias for schedule
	Interval string `yaml:"interval,omitempty"` // Deprecated alias for schedule
}

// IsEnabled returns whether auto-update is enabled.
func (u UpdateConfig) IsEnabled() bool {
	if u.Enabled == nil {
		return true
	}
	return *u.Enabled
}

// GetSchedule returns the cron schedule or default "0 4 * * *" (daily at 04:00).
func (u UpdateConfig) GetSchedule() string {
	if u.Schedule != "" {
		return u.Schedule
	}
	if u.When != "" {
		return u.When
	}
	if u.Interval != "" {
		return u.Interval
	}
	return "0 4 * * *"
}

// GetRepo returns the repository to check for releases.
func (u UpdateConfig) GetRepo() string {
	if u.Repo == "" {
		return "naueramant/munin"
	}
	return u.Repo
}

// GetInterval parses the update interval duration.
func (u UpdateConfig) GetInterval() time.Duration {
	if u.Interval == "" {
		return 24 * time.Hour
	}
	d, err := time.ParseDuration(u.Interval)
	if err != nil || d <= 0 {
		return 24 * time.Hour
	}
	return d
}

// DisplayConfig specifies display and chromium behavior.
type DisplayConfig struct {
	Env           string   `yaml:"env,omitempty"`            // e.g. ":0"
	ChromiumFlags []string `yaml:"chromium_flags,omitempty"` // Extra chromium flags
}

// Configuration represents the contents of screen.yaml.
type Configuration struct {
	Syntax string        `yaml:"syntax" validate:"required,syntax"`
	Tabs   []Tab         `yaml:"tabs,omitempty" validate:"omitempty,dive"`
	Power  PowerConfig   `yaml:"power,omitempty" validate:"omitempty,dive"`
	Jobs   []Job         `yaml:"jobs,omitempty" validate:"omitempty,dive"`
	Files  []FileMapping `yaml:"files,omitempty" validate:"omitempty,dive"`
}

// PowerConfig defines HDMI CEC power schedule and system power operations.
type PowerConfig struct {
	ScreenOn  string `yaml:"screen_on,omitempty"`  // Cron expression or HH:MM to power on screen
	ScreenOff string `yaml:"screen_off,omitempty"` // Cron expression or HH:MM to standby screen
	Reboot    string `yaml:"reboot,omitempty"`     // Cron expression or HH:MM to reboot system
	PowerOff  string `yaml:"power_off,omitempty"` // Cron expression or HH:MM to power off / shutdown system
	CecDevice *int   `yaml:"cec_device,omitempty"` // CEC device number, defaults to 0 (TV)

	// Deprecated backward-compatibility aliases
	TurnOn  string `yaml:"turn_on,omitempty"`
	TurnOff string `yaml:"turn_off,omitempty"`
}

// GetScreenOn returns the screen on cron expression (or legacy turn_on).
func (p PowerConfig) GetScreenOn() string {
	if p.ScreenOn != "" {
		return p.ScreenOn
	}
	return p.TurnOn
}

// GetScreenOff returns the screen off cron expression (or legacy turn_off).
func (p PowerConfig) GetScreenOff() string {
	if p.ScreenOff != "" {
		return p.ScreenOff
	}
	return p.TurnOff
}

// GetReboot returns the system reboot cron expression.
func (p PowerConfig) GetReboot() string {
	return p.Reboot
}

// GetPowerOff returns the system power off cron expression.
func (p PowerConfig) GetPowerOff() string {
	return p.PowerOff
}

// GetCecDevice returns the CEC device ID (default 0).
func (p PowerConfig) GetCecDevice() int {
	if p.CecDevice != nil {
		return *p.CecDevice
	}
	return 0
}

// HasEntries returns whether any power or reboot directives are configured.
func (p PowerConfig) HasEntries() bool {
	return p.GetScreenOn() != "" || p.GetScreenOff() != "" || p.GetReboot() != "" || p.GetPowerOff() != ""
}

// Tab defines a URL to display and cycle through.
type Tab struct {
	URL      string `yaml:"url" validate:"required"`
	Duration uint64 `yaml:"duration,omitempty"`
	Reload   bool   `yaml:"reload,omitempty"`
	Auth     Auth   `yaml:"auth,omitempty" validate:"omitempty,dive"`
	CSS      string `yaml:"css,omitempty"`
	JS       string `yaml:"js,omitempty"`
}

// Auth defines basic authentication credentials for a Tab.
type Auth struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// Job defines a cron task to be managed in native crontab.
type Job struct {
	When    string  `yaml:"when" validate:"required"`
	Command string  `yaml:"command,omitempty"`
	Type    string  `yaml:"type,omitempty"`
	Options Options `yaml:"options,omitempty" validate:"omitempty,dive"`
}

// GetCommandLine returns the command line string to be run by cron.
func (j Job) GetCommandLine() string {
	if strings.TrimSpace(j.Command) != "" {
		return strings.TrimSpace(j.Command)
	}
	if strings.TrimSpace(j.Options.Command) != "" {
		cmd := strings.TrimSpace(j.Options.Command)
		if len(j.Options.Args) > 0 {
			return fmt.Sprintf("%s %s", cmd, strings.Join(j.Options.Args, " "))
		}
		return cmd
	}
	return ""
}

// Options provides backwards-compatibility for legacy job definitions.
type Options struct {
	Command string   `yaml:"command,omitempty"`
	Args    []string `yaml:"args,omitempty"`

	Duration uint64 `yaml:"duration,omitempty"`
	URL      string `yaml:"url,omitempty"`

	Message         string `yaml:"message,omitempty"`
	FontSize        uint64 `yaml:"fontSize,omitempty"`
	TextColor       string `yaml:"textColor,omitempty"`
	BackgroundColor string `yaml:"backgroundColor,omitempty"`
	Blink           bool   `yaml:"blink,omitempty"`
}

// FileMapping defines a file to copy from the repo/config dir to the local filesystem.
type FileMapping struct {
	Src  string `yaml:"src" validate:"required"`  // Path relative to screen.yaml directory
	Dest string `yaml:"dest" validate:"required"` // Absolute destination path on local machine (supports ~)
	Mode string `yaml:"mode,omitempty"`           // Optional chmod octal mode, e.g. "0755" or "0644"
}
