# Munin Examples

This directory contains production-ready reference configurations and architecture examples for deploying Munin.

---

## 1. [Local Standalone (`1-local-standalone/`)](1-local-standalone/)
- **Use Case**: Single screen, display kiosk, or testing locally without a remote Git repository.
- **Files**:
  - `agent.yaml`: Node agent configuration with `mode: "local"`.
  - `screen.yaml`: Screen configuration with fullscreen tabs and HDMI CEC power scheduling.
- **Documentation**: See [Getting Started: Local Mode](../docs/getting_started.md#local-mode-standalone--no-git).

---

## 2. [Git Fleet Management (`2-git-fleet-management/`)](2-git-fleet-management/)
- **Use Case**: Managing multiple screens across offices, NOCs, reception lobbies, and meeting rooms from a central Git repository.
- **Files**:
  - `node-agent.yaml`: Example `~/.munin/agent.yaml` deployed onto an individual Raspberry Pi node tracking a subfolder.
  - `sample-repo/`: Complete reference Git repository showing how to organize screens:
    - `screens/office-lobby/`: Reception display with rotating dashboard, events calendar, and custom CSS.
    - `screens/ops-dashboard/`: NOC display with 60s Grafana rotation, Datadog status, and healthcheck script synchronization.
    - `shared/`: Shared stylesheets (`common.css`) and corporate branding (`logo.svg`).
- **Documentation**: See [Git Synchronization & Fleet Management](../docs/git_sync.md).
