# Configuration Reference

Munin uses two levels of configuration designed to be intuitive with **sane defaults** so you only need to configure what you actually want to customize:

1. **Host Agent Configuration (`~/.munin/config.yaml`)**: Controls host-level settings: operating mode, Git repository, sync interval, auto-update schedule, log level, and display parameters.
2. **Screen Configuration (`screen.yaml`)**: Defines what appears on the screen: web tabs, rotation durations, HDMI CEC power schedules, scheduled cron jobs, and local file copies.

---

## 1. Agent Configuration (`~/.munin/config.yaml`)

### Supported File Locations

Munin strictly supports two ways to specify host agent configuration:

| Priority | Location | Description |
| :--- | :--- | :--- |
| 1 | `--agent-config <path>` | Explicit path specified via command-line argument |
| 2 | `~/.munin/config.yaml` | Standard default configuration file location |

No other ambiguous candidate paths are loaded.

---

### Agent Configuration Reference Table

| Key | Type | Default Value | Description |
| :--- | :--- | :--- | :--- |
| `mode` | string | `"git"` (if repo set) or `"local"` | Operating mode: `"git"` or `"local"`. Automatically inferred if omitted. |
| `log_level` | string | `"info"` | Logging verbosity: `"debug"`, `"info"`, `"warn"`, or `"error"`. Can also be set via `--log-level` or `LOG_LEVEL` env var. Under `"info"`, Munin stays quiet and only logs meaningful state changes. |
| `screen_path` | string | `~/.munin/screen.yaml` or `"screen.yaml"` | Path to the local screen configuration file when running in `"local"` mode. |
| **`git.repo`** | string | *Required for git mode* | Git repository clone URL (e.g. `git@github.com:org/screens.git` or `https://...`). |
| `git.branch` | string | `"main"` | Git branch to track. |
| `git.subdir` | string | `""` (repo root) | Subdirectory inside the repository where `screen.yaml` resides. |
| `git.deploy_key` | string | Auto-discovered | Path to private SSH key. Defaults to `~/.ssh/id_munin_deploy`, `~/.ssh/id_ed25519`, or `~/.ssh/id_rsa` if found on disk. |
| `git.schedule` | string | `"* * * * *"` | Standard 5-field cron expression for checking remote Git repository (e.g. `"* * * * *"` for every minute, `"*/5 * * * *"` for every 5m). Override via `--git-schedule`. |
| `git.target_dir` | string | `"~/.munin/repo"` | Directory on the host where the Git repository will be cloned. |
| `update.enabled` | boolean | `true` | Enables automatic binary updates from GitHub Releases. |
| `update.repo` | string | `"naueramant/munin"` | GitHub repository (`"owner/repo"`) to monitor for new binary releases. |
| `update.schedule` | string | `"0 4 * * *"` | Standard 5-field cron expression for checking GitHub releases (e.g. `"0 4 * * *"` for daily at 4:00 AM, `"0 4 * * 1"` for Mondays at 4:00 AM). Override via `--update-schedule`. |
| `display.env` | string | `":0"` | X11 `DISPLAY` environment variable passed to Chromium. |
| `display.chromium_flags` | list | `[]` | Extra command-line flags to pass to Chromium (in addition to default kiosk flags). |

---

### Agent Configuration Examples

#### Minimal Git Setup (Zero-touch SSH key & main branch)
```yaml
# ~/.munin/config.yaml
git:
  repo: "git@github.com:myorg/dashboards.git"
  subdir: "screens/lobby"
```

#### Minimal Local Setup
```yaml
# ~/.munin/config.yaml
mode: "local"
screen_path: "~/dashboards/screen.yaml"
```

#### Complete Fleet Setup
```yaml
# ~/.munin/config.yaml
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
  schedule: "0 4 * * *" # Check daily at 4:00 AM

display:
  env: ":0"
  chromium_flags:
    - "disable-gpu"
```

---

## 2. Screen Configuration (`screen.yaml`)

Placed in the target Git repository's `subdir` (or referenced locally).

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
Controls TV power and active HDMI source using native `cec-utils` and system crontab:

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `turn_on` | string | `""` | Standard 5-field cron expression for turning on the TV and selecting the HDMI source (e.g. `"0 7 * * 1-5"` for Mon-Fri at 7:00 AM). |
| `turn_off` | string | `""` | Standard 5-field cron expression for putting the TV in standby mode (e.g. `"0 19 * * 1-5"` for Mon-Fri at 7:00 PM). |
| `cec_device` | integer | `0` | CEC target device address (`0` is standard for TV). |

#### Native Scheduled Jobs (`jobs:`)
Commands to install into the user's native crontab:

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `when` | string | *Required* | Standard 5-field cron expression (e.g. `"0 3 * * *"` for 3:00 AM daily). |
| `command` | string | *Required* | Shell command string to execute. |

#### Local File Synchronization (`files:`)
Copies files from the repository to the host filesystem with SHA-256 comparison to prevent SD card flash wear:

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `src` | string | *Required* | Path relative to `screen.yaml` (e.g. `"scripts/healthcheck.sh"`). |
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

# Display power schedules (TV turned on during working hours)
power:
  turn_on: "0 7 * * 1-5"   # Turn on screen at 7:00 AM Monday-Friday
  turn_off: "0 18 * * 1-5"  # Put screen on standby at 6:00 PM Monday-Friday
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
