# Configuration Reference

Munin uses two levels of configuration designed to be intuitive with **sane defaults** so you only need to configure what you actually want to customize:

1. **Host Agent Configuration (`~/.munin/agent.yaml`)**: Host-level settings: operating mode, Git repository, sync schedule, auto-update schedule, log verbosity, and display parameters.
2. **Screen Configuration (`screen.yaml`)**: Screen-level display settings: web tabs, rotation durations, HDMI CEC power schedules, scheduled cron jobs, and local file synchronization.

---

## 1. Agent Configuration (`~/.munin/agent.yaml`)

### Supported File Locations

| Priority | Location | Description |
| :--- | :--- | :--- |
| 1 | `--agent-config <path>` | Explicit path passed via command-line argument |
| 2 | `~/.munin/agent.yaml` | Standard default configuration file location |

---

### Agent Configuration Reference Table

| Key | Type | Default Value | Description |
| :--- | :--- | :--- | :--- |
| `mode` | string | `"git"` (if repo set) or `"local"` | Operating mode: `"git"` or `"local"`. Automatically inferred if omitted. |
| `log_level` | string | `"info"` | Logging verbosity: `"debug"`, `"info"`, `"warn"`, or `"error"`. Can also be set via `--log-level` flag or `LOG_LEVEL` environment variable. |
| `screen_path` | string | `~/.munin/screen.yaml` or `"screen.yaml"` | Path to local screen configuration file when running in `"local"` mode. |
| **`git.repo`** | string | *Required for git mode* | Remote Git repository clone URL (e.g. `git@github.com:org/screens.git` or `https://...`). |
| `git.branch` | string | `"main"` | Git branch to track. |
| `git.subdir` | string | `""` (repo root) | Subdirectory inside the repository where `screen.yaml` resides. |
| `git.deploy_key` | string | Auto-discovered | Path to private SSH key. Automatically searches for `~/.ssh/id_munin_deploy`, `~/.ssh/id_ed25519`, and `~/.ssh/id_rsa`. |
| `git.schedule` | string | `"* * * * *"` | Standard 5-field cron expression for checking remote Git repository (e.g. `"* * * * *"` for every minute, `"*/5 * * * *"` for every 5m). |
| `git.target_dir` | string | `"~/.munin/repo"` | Directory on the host where the Git repository is cloned and cached. |
| `update.enabled` | boolean | `true` | Enables automatic binary updates from GitHub Releases. |
| `update.repo` | string | `"naueramant/munin"` | GitHub repository (`"owner/repo"`) to check for binary releases. |
| `update.schedule` | string | `"0 4 * * *"` | Standard 5-field cron expression for checking GitHub releases (e.g. `"0 4 * * *"` for daily at 04:00). |
| `display.env` | string | `":0"` | X11 `DISPLAY` environment variable passed to Chromium. |
| `display.chromium_flags` | list | `[]` | Extra command-line flags to pass to Chromium (in addition to default kiosk flags). |

---

### Agent Configuration Examples

#### Minimal Git Fleet Setup
```yaml
# ~/.munin/agent.yaml
git:
  repo: "git@github.com:myorg/dashboards.git"
  subdir: "screens/office-lobby"
```

#### Minimal Local Standalone Setup
```yaml
# ~/.munin/agent.yaml
mode: "local"
screen_path: "~/dashboards/screen.yaml"
```

#### Complete Agent Setup
```yaml
# ~/.munin/agent.yaml
mode: "git"
log_level: "info"

git:
  repo: "git@github.com:myorg/dashboards.git"
  deploy_key: "~/.ssh/id_munin_deploy"
  branch: "main"
  subdir: "screens/hq-reception"
  schedule: "*/2 * * * *" # Sync every 2 minutes
  target_dir: "~/.munin/repo"

update:
  enabled: true
  repo: "naueramant/munin"
  schedule: "0 4 * * *" # Check daily at 04:00

display:
  env: ":0"
  chromium_flags:
    - "--disable-gpu"
```

---

## 2. Screen Configuration (`screen.yaml`)

Placed in the target Git repository's `subdir` (or referenced locally via `screen_path`).

### Screen Configuration Reference Tables

#### Tabs (`tabs:`)
Each entry defines a web dashboard to render in fullscreen Chromium:

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `url` | string | *Required* | The web URL or local file URL (`http://`, `https://`, `file://`). |
| `duration` | integer | `30` (if multi-tab) or `0` | Seconds to display this tab before rotating to the next. If only 1 tab is defined, `0` displays it permanently. |
| `reload` | boolean | `false` | If `true`, the page is refreshed each time it becomes active. |
| `auth.username` | string | `""` | Optional HTTP Basic Authentication username. |
| `auth.password` | string | `""` | Optional HTTP Basic Authentication password. |
| `css` | string | `""` | Path to custom CSS file to inject (relative to `screen.yaml` or absolute). |
| `js` | string | `""` | Path to custom JavaScript file to inject (relative to `screen.yaml` or absolute). |

#### Display Power Scheduling (`power:`)
Controls TV power, system reboots, and active HDMI source using native `cec-utils` and system crontab. Supports standard 5-field cron syntax or simple 24-hour time format (`"HH:MM"`):

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `screen_on` | string | `""` | Cron expression or `"HH:MM"` for turning on the TV and selecting the HDMI source (e.g. `"0 7 * * 1-5"` or `"07:00"`). |
| `screen_off` | string | `""` | Cron expression or `"HH:MM"` for putting the TV in standby mode (e.g. `"0 19 * * 1-5"` or `"19:00"`). |
| `reboot` | string | `""` | Cron expression or `"HH:MM"` for rebooting the host machine (e.g. `"0 3 * * *"` or `"03:00"`). Automatically restores TV standby after reboot if `screen_on` hasn't run yet. |
| `power_off` | string | `""` | Cron expression or `"HH:MM"` for shutting down the host machine (e.g. `"0 22 * * 5"`). |
| `cec_device` | integer | `0` | CEC target device address (`0` is standard for TV). |

> **Note**: `turn_on` and `turn_off` remain fully supported as backward-compatible aliases for `screen_on` and `screen_off`.

#### Native Scheduled Jobs (`jobs:`)
Commands to install into the user's native crontab:

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `when` | string | *Required* | Standard 5-field cron expression (e.g. `"0 3 * * *"` for 03:00 daily) or `"HH:MM"`. |
| `command` | string | *Required* | Shell command string to execute. |

#### Local File Synchronization (`files:`)
Copies files from the repository to the host filesystem with SHA-256 comparison to prevent SD card flash wear:

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `src` | string | *Required* | Source path relative to `screen.yaml` (e.g. `"scripts/healthcheck.sh"`). |
| `dest` | string | *Required* | Absolute or home-expanded destination path (e.g. `"/home/pi/scripts/healthcheck.sh"`). |
| `mode` | string | `"0755"` (if exec) or `"0644"` | Octal file permissions (e.g. `"0755"` or `"0644"`). |

---

### Complete `screen.yaml` Example

```yaml
syntax: v1

# Fullscreen tab rotation
tabs:
  - url: "https://grafana.internal/d/fleet-metrics"
    duration: 45
    reload: true
    css: "styles/clean-dashboard.css"

  - url: "https://calendar.google.com/calendar/embed?src=company"
    duration: 15
    reload: false

# Display and host power schedules
power:
  screen_on: "0 7 * * 1-5"   # Turn on screen at 07:00 Monday-Friday
  screen_off: "0 18 * * 1-5"  # Put screen on standby at 18:00 Monday-Friday
  reboot: "0 3 * * *"        # Nightly reboot at 03:00 (auto turns screen back off if off-hours)
  cec_device: 0

# Native cron jobs
jobs:
  - when: "0 3 * * *"
    command: "sudo reboot"

# Files synchronized from the repo to host storage
files:
  - src: "scripts/reload-network.sh"
    dest: "/home/pi/scripts/reload-network.sh"
    mode: "0755"
```

---

## Related Documentation

- **[Getting Started](getting_started.md)**: Setup and quick installation guide.
- **[Git Synchronization & Fleet Management](git_sync.md)**: Fleet repository layout and deploy keys.
- **[Screen Power (HDMI CEC) & Native Cron](cron_and_power.md)**: Details on CEC power handling and post-reboot recovery.
- **[Automatic Updates](auto_update.md)**: Details on the auto-updater and GitHub Releases.
- **[CLI Reference](cli_reference.md)**: All CLI flags and subcommands.
