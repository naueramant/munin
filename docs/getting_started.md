# Getting Started with Munin

Munin is an autonomous, YAML-configurable screen agent designed for Raspberry Pi displays, office TVs, and dashboard kiosks. Named after Munin ("memory"), the raven in Norse mythology that flies across the world gathering information for Odin, Munin pulls display definitions, schedules, and assets to render dynamic dashboards reliably and quietly.

---

## 1. Installation

### Quick Install (Raspberry Pi OS / Debian / Ubuntu)

Install Munin and all its prerequisites (Chromium, `cec-utils`, `cron`, kiosk tools) with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/naueramant/munin/master/install.sh | bash
```

The installer will:
1. Detect host architecture (`arm64`, `armv7`, or `x86_64`).
2. Install necessary system packages (`chromium`, `cec-utils`, `cron`, `unclutter`, fonts).
3. Add the user to hardware access groups (`video`, `render`, `input`) for CEC and GPU acceleration.
4. Download and install the pre-compiled `munin` binary to `/usr/local/bin/munin`.
5. Launch the interactive setup wizard **`munin init`**.

### Building from Source (Go 1.23+)

To build and install Munin manually:

```bash
git clone https://github.com/naueramant/munin.git
cd munin
go build -v -o munin .
sudo mv munin /usr/local/bin/
```

Run tests to verify the build:
```bash
go test -v -race ./...
```

---

## 2. Interactive Setup Wizard (`munin init`)

Run the setup wizard at any time to configure or reconfigure your node:

```bash
munin init
```

The wizard guides you through:
- Selecting between **Git Sync Mode** (fleet) and **Local Standalone Mode**.
- Setting your Git repository URL, branch, subdirectory, and sync schedule.
- Auto-discovering or generating an SSH deploy key (`~/.ssh/id_munin_deploy`).
- Enabling automatic updates from GitHub Releases.
- Generating a starter `screen.yaml` if needed.
- Installing and enabling the systemd user service.

---

## 3. Operating Modes

Munin supports two operational modes:

### Git Sync Mode (Fleet Management)
Recommended for managing multiple screens from a centralized repository. Munin periodically pulls updates, syncs scripts/assets to the local filesystem, updates crontab/CEC schedules, and refreshes browser tabs:

```yaml
# ~/.munin/agent.yaml
git:
  repo: "git@github.com:your-org/screens.git"
  subdir: "screens/lobby-pi"
```
*(With sane defaults, `branch` defaults to `main`, `schedule` to `* * * * *` (every minute), `target_dir` to `~/.munin/repo`, and SSH key to standard locations).*

For detailed fleet setup, see [Git Synchronization & Fleet Management](git_sync.md).

### Local Mode (Standalone / No Git)
Ideal for a single screen or testing locally without a remote Git repository:

```bash
# Run munin directly pointing to a local screen.yaml:
munin --config /path/to/screen.yaml
```

Or configure `mode: "local"` in `~/.munin/agent.yaml`:

```yaml
# ~/.munin/agent.yaml
mode: "local"
screen_path: "/home/pi/screen.yaml"
```

In Local Mode, Munin monitors `screen.yaml` for file changes. Whenever you edit the file, changes are applied live automatically.

---

## 4. Running as a Service (Runs as User)

Munin runs securely under your desktop user account (e.g. `pi`), ensuring seamless access to the graphical desktop session and user crontab without root privileges.

### Managing the User Service

```bash
# Check service status
systemctl --user status munin

# View live streaming logs
journalctl --user -u munin -f

# Start, stop, or restart
systemctl --user start munin
systemctl --user stop munin
systemctl --user restart munin
```

### Enable Boot Autostart (Lingering)
To allow the user service to start on boot without requiring an active GUI login session:
```bash
loginctl enable-linger $USER
```

---

## 5. System Diagnostics (`munin doctor`)

Munin includes a built-in doctor command to verify system dependencies, services, permissions, and configuration:

```bash
# Run comprehensive diagnostic check
munin doctor

# Automatically resolve safe issues (enable lingering, fix SSH permissions)
munin doctor --fix

# Output diagnostic results as JSON
munin doctor --json
```

See the [CLI Reference](cli_reference.md#munin-doctor) for more details.

---

## 6. Screen Power & Scheduled Jobs

Munin translates display power rules into native HDMI CEC commands in crontab:

```yaml
# screen.yaml
power:
  screen_on: "0 7 * * 1-5"   # Turn on display Mon-Fri at 07:00
  screen_off: "0 19 * * 1-5" # Standby Mon-Fri at 19:00
  reboot: "0 3 * * *"        # Nightly reboot at 03:00 (auto-restores standby if off-hours)
```

Test your schedule anytime:
```bash
munin power-check
munin power-check --enforce
```

For full details, see [Screen Power (HDMI CEC) & Native Cron](cron_and_power.md).

---

## 7. Uninstalling Munin (`munin remove`)

To cleanly remove services, crontab entries, and configuration:

```bash
# Interactive uninstallation
munin remove

# Non-interactive uninstall (preserves configuration files)
munin remove -y --keep-config

# Purge everything including ~/.munin and deploy keys
munin remove -y --purge
```

---

## Next Steps

- **[Configuration Reference](configuration.md)**: Full syntax and options for `agent.yaml` and `screen.yaml`.
- **[Git Synchronization & Fleet Management](git_sync.md)**: Managing screens across offices from a single Git repo.
- **[Screen Power & Native Cron](cron_and_power.md)**: HDMI CEC configuration and post-reboot recovery.
- **[Automatic Updates](auto_update.md)**: Scheduled auto-updater via GitHub Releases.
- **[CLI Reference](cli_reference.md)**: Complete list of subcommands and flags.
- **[Examples Directory](../examples/)**: Ready-to-use sample configurations.
