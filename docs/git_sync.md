# Git Synchronization & Fleet Management

Munin allows you to manage multiple screens across an office or campus from a single central Git repository.

---

## Recommended Repository Layout

You can organize your Git repository with a folder per screen or room type:

```
my-screens-repo/
├── screens/
│   ├── lobby/
│   │   ├── screen.yaml
│   │   └── custom.css
│   ├── meeting-room-1/
│   │   ├── screen.yaml
│   │   └── scripts/
│   │       └── refresh.sh
│   └── canteen/
│       └── screen.yaml
└── shared/
    └── logo.png
```

Each Raspberry Pi points to the same repository, but specifies its own `subdir` in `~/.munin.yaml`:
- Lobby Pi: `subdir: "screens/lobby"`
- Meeting Room Pi: `subdir: "screens/meeting-room-1"`

---

## Setting up an SSH Deploy Key

1. **Generate an SSH Key on the Raspberry Pi**:
   ```bash
   ssh-keygen -t ed25519 -f ~/.ssh/id_munin_deploy -N ""
   ```

2. **Add the Public Key to GitHub/GitLab**:
   - Copy the public key contents:
     ```bash
     cat ~/.ssh/id_munin_deploy.pub
     ```
   - On GitHub: Go to your repository > **Settings** > **Deploy keys** > **Add deploy key**.
   - Paste the public key. (Read-only access is sufficient).

3. **Configure Munin**:
   In `~/.munin/config.yaml`:
   ```yaml
   git:
     repo: "git@github.com:myorg/my-screens-repo.git"
     deploy_key: "~/.ssh/id_munin_deploy"
     branch: "main"
     subdir: "screens/lobby"
     schedule: "* * * * *" # Cron expression (defaults to every minute)
   ```

---

## How Git Sync Works

1. **First Boot / Initial Clone**:
   - Munin clones the repository into `~/.munin/repo` using the SSH deploy key.
   - It verifies that the specified `subdir` contains a valid `screen.yaml`.
   - It performs initial file synchronization, crontab setup, and launches Chromium.

2. **Periodic Synchronization**:
   - Every `interval` (e.g. `60s`), Munin runs a fast `git fetch` against the remote.
   - If the commit hash has changed:
     - The worktree is reset to the new commit (`git reset --hard origin/<branch>`).
     - Declared files in `files` are re-synchronized.
     - Crontab entries are updated.
     - Chromium tabs and custom CSS/JS are reloaded.

3. **Offline Resilience**:
   - If the Raspberry Pi loses internet connection or the Git host is unreachable, Munin logs a warning and **continues displaying the locally cached configuration**.
   - The display does not crash or show blank pages when offline.
