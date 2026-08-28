# Munin

**Autonomous, YAML-configurable screen agent and dashboard display manager for Raspberry Pi and Linux kiosks.**

*Named after Munin ("memory"), the raven of Odin that flies across the world to bring back information.*

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Munin displays, cycles, and manages fullscreen web dashboards (Grafana, Datadog, internal metrics) on Raspberry Pi displays and Linux kiosks. It runs securely under an unprivileged user session, automatically tracks a remote Git repository or local YAML file, and controls TV power via native HDMI CEC and crontab.

---

## Get Started

### 1. Install

Install Munin and all system dependencies (`chromium`, `cec-utils`, `cron`, `unclutter`) with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/naueramant/munin/master/install.sh | bash
```

The installer downloads dependencies, installs the binary to `/usr/local/bin/munin`, and automatically launches the interactive **`munin init`** setup wizard.

### 2. Interactive Setup Wizard

Configure your display or connect to a Git fleet repository anytime:

```bash
munin init
```

### 3. Service Control

Munin runs securely as a systemd user service:

```bash
# Check service status & live logs
systemctl --user status munin
journalctl --user -u munin -f

# Start, stop, or restart
systemctl --user restart munin
```

### 4. Diagnostics & Health Check

Verify dependencies, permissions, display variables, and configurations:

```bash
munin doctor         # Run diagnostic checks
munin doctor --fix   # Automatically resolve common issues (lingering, permissions)
```

### 5. Running Locally (Without Git)

Run Munin directly against a local screen configuration file:

```bash
munin --config /path/to/screen.yaml
```

---

## Documentation Overview

Comprehensive documentation is available in the [`docs/`](docs/) directory:

| Guide | Description |
| :--- | :--- |
| **[Getting Started](docs/getting_started.md)** | Step-by-step installation, setup wizard walkthrough, user service setup, and uninstallation |
| **[Configuration Reference](docs/configuration.md)** | Full YAML schema and reference tables for `~/.munin/agent.yaml` and `screen.yaml` |
| **[Git Sync & Fleet Management](docs/git_sync.md)** | Managing multi-screen fleets from a single repository with SSH deploy keys and offline caching |
| **[Screen Power & Native Cron](docs/cron_and_power.md)** | HDMI CEC display power control (`cec-utils`), scheduled reboots, and post-reboot standby recovery |
| **[Automatic Updates](docs/auto_update.md)** | Scheduled background updates directly from GitHub Releases |
| **[CLI Reference](docs/cli_reference.md)** | Complete reference for all CLI subcommands (`init`, `doctor`, `power-check`, `remove`) and flags |
| **[Examples & Architecture](examples/)** | Ready-to-use sample configs for standalone nodes and multi-screen Git fleets |

---

## Building from Source

Requirements: Go 1.23+

```bash
git clone https://github.com/naueramant/munin.git
cd munin
go build -v .
go test -v ./...
```

---

## License

Munin is open source software licensed under the [MIT License](LICENSE).
