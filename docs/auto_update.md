# Automatic Agent Updates

Munin includes a built-in auto-update engine that checks GitHub Releases, downloads updated binaries matching the host CPU architecture, and atomically replaces the running executable.

---

## Configuration

In `~/.munin/agent.yaml`:

```yaml
update:
  # Whether auto-updates are enabled (default: true)
  enabled: true

  # GitHub repository to check (format: "owner/repo", default: "naueramant/munin")
  repo: "naueramant/munin"

  # Schedule when to check for updates using standard 5-part cron syntax
  # Examples:
  # - "0 4 * * *" (daily at 04:00 - default)
  # - "0 4 * * 1" (every Monday at 04:00)
  schedule: "0 4 * * *"
```

You can also override this from the command line:
```bash
# Check daily at 03:30
munin --update-schedule "30 3 * * *"
```

To disable auto-updates completely:
```yaml
update:
  enabled: false
```

---

## How It Works

1. **Scheduled Release Check**:
   - The agent queries `https://api.github.com/repos/{owner}/{repo}/releases/latest` according to your configured `schedule` cron expression.
   - It compares the latest release tag (e.g. `v1.2.0`) with `munin --version`.
2. **Architecture Matching**:
   - Matches the host operating system and CPU architecture against published release assets:
     - `linux/arm64` (Raspberry Pi 3/4/5 64-bit OS)
     - `linux/armv7` (Raspberry Pi 32-bit OS)
     - `linux/amd64` (x86_64 PCs / VMs)
3. **Atomic Self-Replacement**:
   - The archive is downloaded and the new `munin` binary is extracted into a temporary file.
   - Execute permissions (`0755`) are applied.
   - The new binary is renamed over `/usr/local/bin/munin` atomically using Linux `rename()`.
4. **Service Restart**:
   - Munin exits cleanly after self-replacement, allowing systemd (`Restart=always`) to immediately restart the service with the new version.

---

## Related Documentation

- **[Configuration Reference](configuration.md)**: Agent configuration schema for `update:`.
- **[CLI Reference](cli_reference.md)**: Daemon flags including `--update-schedule`.
