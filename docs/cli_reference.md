# CLI Reference

Munin provides a command-line interface with dedicated subcommands for setup, health checks, power management, and uninstallation.

---

## Command Overview

| Command | Description |
| :--- | :--- |
| [`munin`](#munin-screen-agent) | Runs the autonomous screen agent daemon |
| [`munin init`](#munin-init) | Launches the interactive setup wizard |
| [`munin doctor`](#munin-doctor) | Diagnoses dependencies, services, permissions, and configuration |
| [`munin power-check`](#munin-power-check) | Evaluates screen power schedule and edge-case status |
| [`munin remove`](#munin-remove) | Removes Munin service, crontab entries, and configuration |

---

## `munin` (Screen Agent)

Runs the core agent daemon. When run without subcommands, Munin starts the local HTTP asset server, determines the operating mode (Git or Local), applies display configuration, launches Chromium in fullscreen kiosk mode, and enters its synchronization loop.

```bash
munin [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--agent-config` | string | `~/.munin/agent.yaml` | Path to host agent configuration file |
| `--config` | string | `""` | Path to a local `screen.yaml` file (forces Local Mode without Git) |
| `--git-schedule` | string | `""` | Override Git synchronization cron schedule (e.g. `"* * * * *"`, `"*/5 * * * *"`) |
| `--update-schedule` | string | `""` | Override GitHub Releases auto-update cron schedule (e.g. `"0 4 * * *"`) |
| `--log-level` | string | `"info"` | Logging verbosity (`debug`, `info`, `warn`, `error`). Can also be set via `LOG_LEVEL` environment variable |
| `--version` | bool | `false` | Display Munin version and exit |

### Examples

```bash
# Start Munin using default agent configuration (~/.munin/agent.yaml)
munin

# Run Munin directly against a local screen file without Git
munin --config /home/pi/screen.yaml

# Run with verbose debug logging enabled
munin --log-level debug

# Override Git sync schedule to run every 10 minutes
munin --git-schedule "*/10 * * * *"
```

---

## `munin init`

Launches an interactive terminal setup wizard for zero-friction provisioning of new screens.

```bash
munin init
```

### What the Wizard Does:
1. Prompts to choose between **Git Sync Mode** (fleet management) and **Local Standalone Mode**.
2. Configures repository URL, branch, subdirectory, and Git sync cron schedule.
3. Automatically scans for existing SSH keys (`~/.ssh/id_munin_deploy`, `~/.ssh/id_ed25519`, `~/.ssh/id_rsa`) or generates a new dedicated deploy key (`~/.ssh/id_munin_deploy`).
4. Generates a starter `~/.munin/agent.yaml` (and `~/.munin/screen.yaml` if in Local mode).
5. Configures and starts the systemd user service (`munin.service`).

---

## `munin doctor`

Runs a comprehensive health check on the host system to diagnose missing dependencies, systemd issues, hardware permissions, and configuration errors.

```bash
munin doctor [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--fix` | bool | `false` | Attempt automatic remediation for safe, detected issues |
| `--json` | bool | `false` | Output diagnostic results as structured JSON for automation/monitoring |
| `-v`, `--verbose` | bool | `false` | Display detailed inspection steps and debug information |
| `--agent-config` | string | `~/.munin/agent.yaml` | Path to agent configuration to validate |
| `--config` | string | `""` | Path to screen configuration to validate |

### Diagnostic Checks Performed:
- **Dependencies**: Presence of Chromium browser (`chromium-browser`, `chromium`, `google-chrome`), `cec-client`, `crontab`, `unclutter`, and `ssh`.
- **Systemd & Services**: User systemd manager responsiveness, `munin.service` unit presence and active status, systemd user lingering (`loginctl enable-linger`), and host `cron` daemon status.
- **Hardware & Display**: `$DISPLAY` / `$WAYLAND_DISPLAY` accessibility, user membership in `video`, `render`, and `input` groups, and `/dev/cec*` / `/dev/vchiq` device access.
- **Configuration & Crontab**: YAML syntax validation, tab URL validity, cron schedule syntax, and inspection of active Munin crontab entries.
- **Git Fleet**: SSH deploy key presence, private key permissions (`0600`), and remote Git repository reachability.

### Auto-Fix (`--fix`):
When run with `--fix`, the doctor will automatically:
- Enable systemd user lingering via `loginctl enable-linger` (ensuring background services start without interactive login).
- Fix insecure SSH key permissions (sets deploy key to `0600`).
- Enable the systemd user service if disabled.

---

## `munin power-check`

Inspects the configured HDMI CEC screen power schedule and evaluates whether the screen should currently be ON or OFF.

```bash
munin power-check [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--config` | string | `""` | Path to `screen.yaml` (defaults to auto-discovery from agent configuration) |
| `--enforce` | bool | `false` | Immediately sends the CEC standby command if the screen is determined to be off |

### Examples

```bash
# Check current screen power schedule status
munin power-check

# Test power schedule and immediately enforce standby if during off-hours
munin power-check --enforce
```

---

## `munin remove`

Uninstalls Munin services, removes native crontab entries, and cleans up configuration and binaries.

```bash
munin remove [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-y`, `--yes` | bool | `false` | Non-interactive mode, automatically accept confirmation prompts |
| `-f`, `--force` | bool | `false` | Alias for `--yes` |
| `--purge` | bool | `false` | Automatically purge configuration directory (`~/.munin`), deploy keys, and binary |
| `--keep-config` | bool | `false` | Remove systemd service and crontab entries, but keep `~/.munin` files |

### Examples

```bash
# Interactive uninstallation
munin remove

# Non-interactive uninstall keeping configuration
munin remove -y --keep-config

# Complete automated purge
munin remove -y --purge
```
