# Screen Power (HDMI CEC) & Native Cron

Munin delegates recurring tasks and screen power scheduling to the native Linux `crontab` rather than running an internal scheduler inside the agent process. This ensures that scheduled commands and screen power controls run reliably with native OS tooling.

---

## 1. HDMI CEC Power Management (`cec-utils`)

Modern TVs and monitors connected to a Raspberry Pi via HDMI support the **Consumer Electronics Control (CEC)** standard. This allows the Raspberry Pi to turn the TV on, switch to the Pi's HDMI input, and put the TV into standby.

### Prerequisites
Install `cec-utils` on Raspberry Pi OS:
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

In your screen configuration, specify the `power` block with standard 5-part cron syntax:

```yaml
power:
  turn_on: "0 7 * * 1-5"   # Turn on Monday to Friday at 07:00
  turn_off: "0 19 * * 1-5" # Turn off Monday to Friday at 19:00
  cec_device: 0            # Default CEC target (0 is standard for TV)
```

Munin automatically translates these into native `cec-client` commands in the user's crontab.

---

## 3. Scheduled Jobs in `screen.yaml`

You can also specify general system commands:

```yaml
jobs:
  - when: "0 3 * * *"
    command: "sudo reboot"
  - when: "*/15 * * * *"
    command: "/home/pi/scripts/report-status.sh"
```

---

## 4. How Native Crontab is Managed

Munin manages a dedicated, isolated block inside your user's crontab (`crontab -l`):

```cron
# Existing personal jobs are preserved!
0 12 * * * /home/pi/my-other-job.sh

# --- BEGIN MUNIN MANAGED JOBS [DO NOT EDIT] ---
# Display Power: Turn On & set active source (device 0)
0 7 * * 1-5 echo 'on 0' | cec-client -s -d 1 && echo 'as' | cec-client -s -d 1 > /dev/null 2>&1
# Display Power: Turn Off / Standby (device 0)
0 19 * * 1-5 echo 'standby 0' | cec-client -s -d 1 > /dev/null 2>&1
0 3 * * * sudo reboot
# --- END MUNIN MANAGED JOBS ---
```

- **Safe coexistence**: Any crontab entries outside the `# BEGIN MUNIN` and `# END MUNIN` markers are preserved.
- **No root required**: Munin runs as user `pi` and edits user `pi`'s crontab directly.
- **No redundant writes**: If the generated crontab matches the active crontab, Munin avoids invoking `crontab -`.
