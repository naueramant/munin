# Munin

**Autonomous, YAML-configurable screen agent and dashboard display manager for Raspberry Pi and Linux kiosks.**

*Named after Munin ("memory"), the raven of Odin that flies across the world to bring back information.*

[![CI](https://github.com/naueramant/munin/actions/workflows/ci.yaml/badge.svg)](https://github.com/naueramant/munin/actions/workflows/ci.yaml)
[![Release](https://github.com/naueramant/munin/actions/workflows/release.yaml/badge.svg)](https://github.com/naueramant/munin/actions/workflows/release.yaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Features

- **Fullscreen Chromium Kiosk**: Display and automatically cycle between multiple web tabs (e.g. Grafana, Datadog, internal dashboards).
- **Runs Securely as User**: Runs under the desktop user account (e.g. `pi`) via systemd user service (`systemctl --user`), without root permissions.
- **Git Fleet Synchronization**: Auto-sync configuration and assets from a Git repository with offline caching and automatic SSH deploy key discovery.
- **Sane Defaults**: Minimal configuration required out-of-the-box — just set your Git repo and subdirectory.
- **Local Standalone Mode**: Run directly from a local `screen.yaml` file without Git.
- **Native Raspberry Pi Cron**: Recurring jobs and screen power schedules are managed directly in the host's native `crontab`.
- **HDMI CEC Screen Power**: Power TVs on/off and switch active HDMI inputs automatically using `cec-utils`.
- **Local File Synchronization**: Copy scripts, configs, and assets from your Git repo to the host filesystem with SD-card flash wear prevention.
- **Automatic Agent Updates**: Built-in auto-updater downloads and applies new releases directly from GitHub Releases on your schedule.
- **Structured Logging (`slog`)**: Clean, configurable log levels (`debug`, `info`, `warn`, `error`) that stay quiet during normal operations.
- **One-Line Installer**: Single command install script tailored for Raspberry Pi OS.

---

## Quick Start (Raspberry Pi)

Install Munin and all system prerequisites (`chromium`, `cec-utils`, `cron`, `unclutter`) on Raspberry Pi OS with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/naueramant/munin/master/install.sh | bash
```

The installer downloads dependencies, installs the `munin` binary, and automatically launches the interactive **`munin init`** setup wizard!

You can also run or re-run the wizard manually anytime:
```bash
munin init
```

Control Munin as a regular user via systemd:
```bash
# Check status
systemctl --user status munin

# Stream live logs
journalctl --user -u munin -f

# Restart or stop
systemctl --user restart munin
```

---

## Configuration Overview

Munin is built with **sane defaults** so you only configure what you need:

### 1. Host Agent Config (`~/.munin/config.yaml`)

```yaml
# ~/.munin/config.yaml
git:
  repo: "git@github.com:myorg/dashboards.git"
  subdir: "screens/lobby"
```

*(Defaults automatically applied: `branch: main`, `schedule: "* * * * *"` (every minute), `target_dir: ~/.munin/repo`, auto-discovered SSH keys from `~/.ssh`, and daily update checks at 4:00 AM).*

#### Summary Configuration Options Table:
| Option | Default | Description |
| :--- | :--- | :--- |
| `mode` | `"git"` / `"local"` | Operating mode (auto-detected) |
| `log_level` | `"info"` | Log level: `debug`, `info`, `warn`, `error` |
| `git.repo` | *Required in git mode* | Git repository clone URL |
| `git.branch` | `"main"` | Branch to track |
| `git.subdir` | `""` (root) | Subfolder containing `screen.yaml` |
| `git.schedule` | `"* * * * *"` | Cron expression for Git sync (e.g. `"* * * * *"`, `"*/5 * * * *"`) |
| `git.deploy_key` | Auto-detected | Path to SSH private key |
| `update.enabled` | `true` | Automatic GitHub release updates |
| `update.schedule` | `"0 4 * * *"` | Cron expression for update check (daily at 4:00 AM) |

*See [docs/configuration.md](docs/configuration.md) for the complete reference table.*

---

### 2. Screen Config (`screen.yaml`)
Defined in your Git repository's `subdir` (or local file):

```yaml
syntax: v1

# Fullscreen tabs to cycle through (defaults to 30s rotation)
tabs:
  - url: "https://grafana.internal/d/lobby"
    duration: 60
    reload: true
  - url: "https://xkcd.com/"
    duration: 15

# HDMI CEC screen power management (cec-utils)
power:
  turn_on: "0 7 * * 1-5"   # Turn on Mon-Fri 07:00
  turn_off: "0 19 * * 1-5" # Standby Mon-Fri 19:00

# Native cron jobs
jobs:
  - when: "0 3 * * *"
    command: "sudo reboot"

# Files to copy to the local host filesystem
files:
  - src: "scripts/check.sh"
    dest: "/home/pi/scripts/check.sh"
    mode: "0755"
```

---

## Running in Local Mode (Without Git)

You can run Munin directly with a local configuration file:

```bash
munin --config /path/to/screen.yaml
```

Whenever `/path/to/screen.yaml` is modified, Munin automatically synchronizes files, updates crontab/CEC, and refreshes the browser tabs.

---

## Documentation

Full documentation is available in the [docs/](docs/) directory:

- [Getting Started & Installation Guide](docs/getting_started.md)
- [Configuration Reference (~/.munin/config.yaml & screen.yaml)](docs/configuration.md)
- [Git Synchronization & Fleet Management](docs/git_sync.md)
- [Screen Power (HDMI CEC) & Native Cron](docs/cron_and_power.md)
- [Automatic Agent Updates](docs/auto_update.md)

---

## Building from Source

Requirements: Go 1.23+ and `golangci-lint`.

```bash
git clone https://github.com/naueramant/munin.git
cd munin
go build -v .
```

Run tests:
```bash
go test -v -race ./...
```

Run linter:
```bash
golangci-lint run ./...
```

---

## License

Munin is licensed under the [MIT License](LICENSE).
