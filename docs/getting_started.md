# Getting Started with Munin

Munin is an autonomous, YAML-configurable screen agent designed for Raspberry Pi displays, office TVs, and dashboard kiosks. Named after Munin ("memory"), the raven in Norse mythology that flies across the world gathering information for Odin, Munin pulls display definitions, schedules, and assets to render dynamic dashboards reliably and quietly.

---

## Quick Installation on Raspberry Pi

You can install Munin and all its prerequisites (Chromium, `cec-utils`, `cron`, kiosk tools) on Raspberry Pi OS with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/naueramant/munin/master/install.sh | bash
```

The installer will:
1. Detect your hardware architecture (`arm64`, `armv7`, or `x86_64`).
2. Install system packages (`chromium`, `cec-utils`, `cron`, `unclutter`, fonts).
3. Add user to hardware groups (`video`, `render`, `input`) for CEC and GPU access.
4. Download and install the `munin` binary to `/usr/local/bin/munin`.
5. Launch the interactive **`munin init`** setup wizard to configure your screen and systemd service!

---

## Interactive Setup Wizard (`munin init`)

You can re-run the interactive setup wizard at any time to reconfigure your node:

```bash
munin init
```

The wizard guides you through:
- Choosing between **Git Sync Mode** (fleet) and **Local Standalone Mode**.
- Configuring your Git repository URL, branch, subdirectory, and sync cron schedule.
- Auto-discovering or generating a new SSH deploy key (`~/.ssh/id_munin_deploy`).
- Setting up automatic GitHub Releases updates.
- Creating a starter `screen.yaml` if needed.
- Installing and starting the systemd user service.

---

## Operating Modes

Munin supports two distinct operational modes:

### 1. Git Sync Mode (Fleet Management)
Used when deploying to one or many screens. Munin clones a remote Git repository, monitors a specific subdirectory for your screen, periodically pulls updates, syncs scripts/assets to the local filesystem, updates cron/CEC, and refreshes the browser.

```yaml
# ~/.munin/config.yaml
git:
  repo: "git@github.com:your-org/screens.git"
  subdir: "screens/lobby-pi"
```
*(With sane defaults, branch defaults to `main`, interval to `60s`, target directory to `~/.munin/repo`, and SSH key to standard locations!)*

### 2. Local Mode (Standalone / No Git)
Used when running locally on a test machine or when you want to manage `screen.yaml` directly on the device without Git.

```bash
# Run munin directly with a local file:
munin --config /path/to/screen.yaml
```

Or configure `mode: "local"` in `~/.munin/config.yaml`:

```yaml
# ~/.munin/config.yaml
mode: "local"
screen_path: "/home/pi/screen.yaml"
```

In Local Mode, Munin monitors `screen.yaml` for changes. Whenever you edit the file, it automatically synchronizes files, updates crontab/CEC, and refreshes the screen.

---

## Running as a Service (Runs as the User)

Munin is designed to run securely as your regular desktop user (`pi`), ensuring that it accesses the user's graphical session, display, and personal crontab without requiring root.

### User Service (Recommended)
Managed entirely under your user account with no `sudo` required:

```bash
# Start Munin
systemctl --user start munin

# View service status
systemctl --user status munin

# View live streaming logs
journalctl --user -u munin -f

# Stop Munin
systemctl --user stop munin
```

### System-Wide Service (Alternative)
If preferred, a template system unit is also available:
```bash
sudo systemctl enable --now munin@pi.service
```

---

## Next Steps

- Check out the [Configuration Reference](configuration.md) for full YAML options and tables.
- Learn how to setup [Git Synchronization & Deploy Keys](git_sync.md).
- Learn how to configure [Screen Power (CEC) & Native Cron](cron_and_power.md).
- Learn about [Automatic Agent Updates](auto_update.md).
