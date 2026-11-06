# CLI Reference

Munin provides a command-line interface with dedicated subcommands for setup, health checks, power management, and uninstallation.

---

## Command Overview

| Command | Description |
| :--- | :--- |
| [`munin`](#munin-screen-agent) | Runs the autonomous screen agent daemon |
| [`munin init`](#munin-init) | Launches the interactive setup wizard |
| [`munin doctor`](#munin-doctor) | Diagnoses dependencies, services, permissions, and configuration |
| [`munin config`](#munin-config) | Prints the resolved screen configuration (includes and env vars applied) |
| [`munin lint`](#munin-lint) | Checks screen configurations for errors, e.g. in CI |
| [`munin power-check`](#munin-power-check) | Evaluates screen power schedule and edge-case status |
| [`munin update`](#munin-update) | Checks for and installs the latest release |
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
- **Dependencies**: Presence of Chromium/Chrome browser (`chromium-browser`, `chromium`, `google-chrome-stable`, `google-chrome`, or custom `$CHROME_BIN`), `cec-client`, `crontab`, `unclutter`, and `ssh`.
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

## `munin config`

Prints the screen configuration after [includes](configuration.md#including-other-files) and [environment variables](configuration.md#environment-variable-substitution) have been resolved, then validates it. Use it to debug shared files and `${VAR}` references, or as a lint step in a fleet repository's CI.

```bash
munin config [path] [flags]
```

Without a path, it prints the `screen.yaml` the agent would load: the configured subdirectory of the git checkout in git mode, or `screen_path` in local mode. The YAML goes to stdout; warnings about unset variables and validation errors go to stderr. The exit code is non-zero if the configuration cannot be loaded or is invalid.

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--effective` | bool | `false` | Print the final configuration with defaults applied (e.g. tab durations, merged `<<:` keys) instead of the resolved file |
| `--annotate` | bool | `false` | Add comments showing which file each included part came from, and list all source files at the top |
| `--show-secrets` | bool | `false` | Show `auth.password` values instead of `***` |
| `--agent-config` | string | `~/.munin/agent.yaml` | Path to agent configuration used to find the screen file when no path is given |

### Examples

```bash
# Show what the agent on this host will load
munin config

# Check a screen in a fleet repository checkout, with source annotations
munin config screens/office-lobby/screen.yaml --annotate

# Show the final values Munin will use, including defaults
munin config screens/office-lobby/screen.yaml --effective
```

---

## `munin lint`

Checks one or more screen configurations for errors. As well as everything `munin config` validates, it catches problems that would otherwise only show up on the device:

- `files[].src` paths that don't exist, and patterns that match no files
- invalid `files[].mode` values
- relative `css` / `js` paths that don't exist (absolute paths refer to the device and are skipped)
- jobs without a command

Unset `${VAR}` references without a default are reported as warnings, since CI usually doesn't have the device's secrets.

```bash
munin lint [path...] [flags]
```

Without a path, it checks the `screen.yaml` the agent would load. Exits non-zero if any screen has errors, or warnings with `--strict`.

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--strict` | bool | `false` | Treat warnings as errors |
| `--agent-config` | string | `~/.munin/agent.yaml` | Path to agent configuration used to find the screen file when no path is given |

### Examples

```bash
# Check every screen in a fleet repository
munin lint screens/*/screen.yaml
```

```
✓ screens/office-lobby/screen.yaml
✗ screens/ops-dashboard/screen.yaml
    error: files[0]: pattern "../../shared/file/*" matched no files
! screens/reception/screen.yaml
    warning: ${GRAFANA_USER} is not set and has no default
```

With `--format github`, each error and warning is also printed as a GitHub Actions annotation, so it shows up on the file in a pull request.

### GitHub Action

Fleet repositories can run `munin lint` on every push and pull request with the Munin action. It downloads the Munin release matching the action's version, lints the matching screen files, and annotates problems on the pull request.

```yaml
# .github/workflows/lint.yaml
name: Lint screens

on:
  push:
  pull_request:

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: naueramant/munin@1.5.0
        with:
          paths: |
            screens/*/screen.yaml
            !screens/old-*/screen.yaml
```

| Input | Default | Description |
| :--- | :--- | :--- |
| `paths` | `**/screen.yaml` | One or more glob patterns, separated by newlines or spaces. `**` matches any depth. Patterns starting with `!` exclude their matches. A pattern that matches nothing fails the check. |
| `strict` | `false` | Treat warnings, such as unset environment variables, as errors. Pass the variables with `env:` if you enable it. |
| `version` | the action's version | Munin release to use, e.g. `"1.5.0"` or `"latest"`. When the action is not pinned to a release tag (e.g. `@main`), the latest release is used. |

The action needs a Linux runner (amd64 or arm64) and Munin 1.5.0 or later, the first release with `munin lint`.

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

## `munin update`

Checks GitHub Releases for a newer version and installs it on demand. This complements the scheduled auto-updater (see [Auto Update](auto_update.md)) by letting you update immediately. Manual invocation always runs regardless of the `update.enabled` setting in `agent.yaml`.

By default the command is interactive: it reports the current and latest versions and asks for confirmation before installing. In non-interactive environments (no TTY), it reports availability without installing unless `--yes` is passed.

```bash
munin update [flags]
```

### Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-y`, `--yes` | bool | `false` | Skip confirmation and install the update immediately |
| `--agent-config` | string | `~/.munin/agent.yaml` | Path to agent configuration file (used to resolve the release repository) |

> **Note:** Development builds (version `dev`) cannot be compared against release tags, so `munin update` always offers to install the latest published release.

### Examples

```bash
# Interactively check for and install the latest release
munin update

# Install the latest release without prompting (for scripts and cron)
munin update --yes
```

After a successful update, restart the Munin service to run the new binary (`systemctl --user restart munin`, or rely on `Restart=always`).

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
