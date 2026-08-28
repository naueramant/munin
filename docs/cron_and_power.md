# Screen Power (HDMI CEC) & Native Cron

Munin delegates recurring tasks and screen power scheduling to the native Linux `crontab` rather than running an internal scheduler inside the agent process. This ensures that scheduled commands and screen power controls run reliably with native OS tooling.

---

## 1. HDMI CEC Power Management (`cec-utils`)

Modern TVs and monitors connected to a Raspberry Pi via HDMI support the **Consumer Electronics Control (CEC)** standard. This allows the Raspberry Pi to turn the TV on, switch to the Pi's HDMI input, and put the TV into standby.

### Prerequisites
Install `cec-utils` on Raspberry Pi OS / Debian:
```bash
sudo apt-get install -y cec-utils
```

Test CEC manually from the command line:
```bash
# Turn on the TV and switch to Raspberry Pi input:
echo 'on 0' | cec-client -s -d 1 && echo 'as' | cec-client -s -d 1

# Put the TV into standby / turn off:
echo 'standby 0' | cec-client -s -d 1
```

---

## 2. Configuring Power in `screen.yaml`

In your screen configuration, specify the `power` block with standard 5-part cron syntax or simple 24-hour time (`HH:MM`):

```yaml
power:
  screen_on: "0 7 * * 1-5"   # Turn on screen Monday to Friday at 07:00 (or "07:00")
  screen_off: "0 19 * * 1-5" # Turn off screen Monday to Friday at 19:00 (or "19:00")
  reboot: "0 3 * * *"        # Reboot host daily at 03:00 (or "03:00")
  power_off: "0 22 * * 5"    # Optional scheduled host shutdown
  cec_device: 0              # Optional CEC target (defaults to 0 for TV)
```

> **Compatibility**: `turn_on` and `turn_off` remain supported as backward-compatible aliases for `screen_on` and `screen_off`.

Munin automatically translates these into native `cec-client` and `sudo` commands in the user's crontab.

---

## 3. Post-Reboot Automatic Standby Recovery

A common problem with HDMI CEC screens is that rebooting the host machine (e.g. Raspberry Pi) resets the GPU and sends an HDMI clock handshake. Many modern TVs automatically wake up when they detect an HDMI signal, even if the reboot occurred during off-hours (e.g. screen off at `19:00`, reboot at `03:00`, screen on at `07:00`).

Munin solves this automatically:
1. Whenever Munin starts up following a reboot, it evaluates the power schedule against the current time.
2. If `screen_off` was scheduled and executed more recently than `screen_on` (i.e. `screen_on` has not run yet), Munin immediately re-asserts TV standby via CEC.
3. A follow-up retry is dispatched after 15 seconds to ensure slow-booting TVs that ignore initial CEC packets are also put back into standby.

You can inspect the power schedule and evaluate current screen status at any time with the CLI:
```bash
munin power-check
munin power-check --enforce  # re-sends standby immediately if screen should be off
```

---

## 4. Scheduled Jobs in `screen.yaml`

You can also specify general system commands:

```yaml
jobs:
  - when: "0 3 * * *"
    command: "sudo reboot"
  - when: "*/15 * * * *"
    command: "/home/pi/scripts/report-status.sh"
```

---

## 5. How Native Crontab is Managed

Munin manages a dedicated, isolated block inside your user's crontab (`crontab -l`):

```cron
# Existing personal jobs are preserved!
0 12 * * * /home/pi/my-other-job.sh

# --- BEGIN MUNIN MANAGED JOBS [DO NOT EDIT] ---
# Display Power: Turn On & set active source (device 0)
0 7 * * 1-5 echo 'on 0' | cec-client -s -d 1 && echo 'as' | cec-client -s -d 1 > /dev/null 2>&1
# Display Power: Turn Off / Standby (device 0)
0 19 * * 1-5 echo 'standby 0' | cec-client -s -d 1 > /dev/null 2>&1
# System Power: Reboot
0 3 * * * sudo reboot > /dev/null 2>&1
# --- END MUNIN MANAGED JOBS ---
```

- **Safe coexistence**: Any crontab entries outside the `# BEGIN MUNIN` and `# END MUNIN` markers are preserved.
- **No root required**: Munin runs as user `pi` and edits user `pi`'s crontab directly.
- **No redundant writes**: If the generated crontab matches the active crontab, Munin avoids invoking `crontab -`.

---

## Related Documentation

- **[Configuration Reference](configuration.md)**: Full syntax for `power:` and `jobs:`.
- **[CLI Reference](cli_reference.md#munin-power-check)**: Details on `munin power-check`.
