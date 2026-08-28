# Munin

Autonomous, YAML-configurable screen agent and dashboard display manager for Raspberry Pi and Linux kiosks.

## Features

- **Dashboard Kiosk Management**: Display and cycle fullscreen web dashboards (Grafana, Datadog, internal metrics) with configurable rotation and reload intervals.
- **GitOps Fleet Management**: Centrally manage multi-screen deployments using a remote Git repository with SSH deploy keys, periodic sync, and offline caching.
- **Local Standalone Mode**: Run without Git against a local YAML file with automatic live reloading on file edits.
- **Display & Power Control**: Manage TV power via native HDMI CEC commands and crontab schedules, including post-reboot standby recovery.
- **Secure by Default**: Runs entirely unprivileged under a systemd user session with no root access required during runtime.
- **Built-in Diagnostics & Self-Healing**: Automated environment and dependency checks via `munin doctor --fix`.
- **Automatic Updates**: Background update checks directly from GitHub Releases.

### About the Name

Named after *Munin* ("memory"), one of Odin's twin ravens in Norse mythology who flies across the world to gather information and bring it back.

---

## Getting Started

Install Munin and its system dependencies with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/naueramant/munin/master/install.sh | bash
```

The installer downloads the binary to `/usr/local/bin/munin`, installs prerequisites (`chromium`, `cec-utils`, `cron`, `unclutter`), and launches the interactive setup wizard:

```bash
munin init
```

Use `munin init` at any time to reconfigure your node or switch between Git Fleet and Local Standalone modes.

> For service management (`systemctl --user`), local standalone mode, diagnostics (`munin doctor`), and manual setup, see the **[Getting Started Guide](docs/getting_started.md)**.

---

## Documentation Overview

Comprehensive documentation is available in the [`docs/`](docs/) directory:

| Guide | Description |
| :--- | :--- |
| **[Getting Started](docs/getting_started.md)** | Step-by-step installation, building from source, setup wizard walkthrough, user service setup, and uninstallation |
| **[Configuration Reference](docs/configuration.md)** | Full YAML schema and reference tables for `~/.munin/agent.yaml` and `screen.yaml` |
| **[Git Sync & Fleet Management](docs/git_sync.md)** | Managing multi-screen fleets from a single repository with SSH deploy keys and offline caching |
| **[Screen Power & Native Cron](docs/cron_and_power.md)** | HDMI CEC display power control (`cec-utils`), scheduled reboots, and post-reboot standby recovery |
| **[Automatic Updates](docs/auto_update.md)** | Scheduled background updates directly from GitHub Releases |
| **[CLI Reference](docs/cli_reference.md)** | Complete reference for all CLI subcommands (`init`, `doctor`, `power-check`, `remove`) and flags |
| **[Examples & Architecture](examples/)** | Ready-to-use sample configs for standalone nodes and multi-screen Git fleets |

---

## License

Munin is open source software licensed under the [MIT License](LICENSE).
