# Fleet Screens Repository

This sample repository demonstrates how to manage multiple Raspberry Pi screens across offices or data centers from a single Git repository.

## Directory Structure

```
my-screens-repo/
├── screens/
│   ├── office-lobby/
│   │   ├── screen.yaml      # Config for the Lobby display
│   │   └── custom.css       # Custom styles injected into tabs
│   └── ops-dashboard/
│       ├── screen.yaml      # Config for the Operations NOC display
│       └── healthcheck.sh   # Script synchronized to the Pi host
└── shared/
    ├── common.css           # Global theme shared across displays
    ├── logo.svg             # Corporate branding asset
    ├── power/
    │   └── office-hours.yaml   # Power schedule included by both screens
    └── schedules/
        ├── 10-lunch.yaml       # Takeovers included with !include_dir_merge_list
        └── 20-friday.yaml
```

## How It Works

1. Each Raspberry Pi runs Munin with a `~/.munin/agent.yaml` pointing to this repository.
2. The `subdir` setting determines which screen configuration the Pi tracks:
   - Lobby Pi: `subdir: "screens/office-lobby"`
   - NOC Pi: `subdir: "screens/ops-dashboard"`
3. Both screens include the shared power schedule and takeovers from `shared/`. The NOC screen merges the shared power schedule with its own longer hours (`<<: !include ...`). Run `munin config screens/ops-dashboard/screen.yaml --annotate` to see the combined result.
4. When you push a Git commit to `main`, each Pi pulls the changes on its configured cron schedule (`git.schedule`), syncs files to disk, updates HDMI CEC power timers in `crontab`, and seamlessly reloads Chromium tabs!

For comprehensive instructions on fleet deployments and deploy key setup, see the [Git Synchronization & Fleet Management Guide](../../../../docs/git_sync.md).
