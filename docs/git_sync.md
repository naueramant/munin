# Git Synchronization & Fleet Management

Munin allows you to manage dozens or hundreds of screens across offices, campuses, or data centers from a single central Git repository.

---

## Recommended Repository Layout

Organize your Git repository with a dedicated folder per screen, room type, or display function:

```
my-screens-repo/
├── screens/
│   ├── office-lobby/
│   │   ├── screen.yaml      # Screen config for Lobby display
│   │   └── custom.css       # Custom CSS injected into tabs
│   ├── ops-dashboard/
│   │   ├── screen.yaml      # Screen config for NOC display
│   │   └── healthcheck.sh   # Host script synced to filesystem
│   └── meeting-room-a/
│       └── screen.yaml
└── shared/
    ├── common.css           # Global theme shared across displays
    └── logo.svg             # Corporate branding asset
```

Each Raspberry Pi points to the same repository URL, but specifies its assigned `subdir` in its host `~/.munin/agent.yaml`:
- Lobby Pi: `subdir: "screens/office-lobby"`
- NOC Pi: `subdir: "screens/ops-dashboard"`
- Room A Pi: `subdir: "screens/meeting-room-a"`

---

## Setting up SSH Deploy Keys

Munin supports standard SSH authentication for GitHub, GitLab, Gitea, and private Git servers.

### 1. Generate an SSH Key on the Raspberry Pi
```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_munin_deploy -N ""
```

### 2. Add the Public Key to your Git Host
- Display the public key:
  ```bash
  cat ~/.ssh/id_munin_deploy.pub
  ```
- In GitHub / GitLab: Navigate to your repository > **Settings** > **Deploy keys** > **Add deploy key**.
- Paste the public key and save. *(Read-only access is sufficient).*

### 3. Automatic Discovery
Munin automatically detects SSH keys at standard locations (`~/.ssh/id_munin_deploy`, `~/.ssh/id_ed25519`, `~/.ssh/id_rsa`). If using `id_munin_deploy`, you do not even need to specify `deploy_key` in your configuration!

```yaml
# ~/.munin/agent.yaml
git:
  repo: "git@github.com:myorg/my-screens-repo.git"
  subdir: "screens/office-lobby"
```

To specify an explicit key path:
```yaml
git:
  repo: "git@github.com:myorg/my-screens-repo.git"
  subdir: "screens/office-lobby"
  deploy_key: "/home/pi/.ssh/custom_key"
```

---

## How Synchronization Works

1. **Initial Clone (First Boot)**:
   - Munin executes a shallow clone (`--depth 1`) into `git.target_dir` (defaults to `~/.munin/repo`).
   - The entire worktree is checked out so shared assets (e.g. `../../shared/logo.svg`) can be accessed by any screen subfolder.
   - Munin validates that `screen.yaml` exists in the configured `subdir`.
   - Munin applies local file copies, installs native crontab jobs, reconciles power schedules, and launches Chromium.

2. **Scheduled Sync**:
   - According to `git.schedule` (defaults to `* * * * *`, checking every minute), Munin performs a lightweight `git fetch origin <branch> --depth 1`.
   - If a new commit is detected:
     - The local worktree is atomically updated using `git reset --hard origin/<branch>`.
     - Files declared under `files:` in `screen.yaml` are compared via SHA-256 and updated only if modified.
     - Crontab jobs and HDMI CEC timers are re-calculated and updated in place.
     - Browser tabs and injected stylesheets/scripts are updated live without rebooting the browser process.

3. **Offline Resilience**:
   - If the network goes down or the Git host is unreachable, Munin logs a warning and **continues running the locally cached configuration**.
   - The display remains active and will automatically resume checking once network connectivity is restored.

---

## Related Documentation

- **[Configuration Reference](configuration.md)**: Agent and Screen YAML reference tables.
- **[Examples Directory](../examples/2-git-fleet-management/)**: Complete sample fleet repository structure.
- **[CLI Reference](cli_reference.md)**: Overriding Git sync schedule from the command line (`--git-schedule`).
