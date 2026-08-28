# Munin Examples

This directory contains reference architectures and examples for running Munin:

## 1. [Local Standalone (`1-local-standalone/`)](1-local-standalone/)
- Ideal for a single screen or testing locally without a Git server.
- Shows how to configure `~/.munin/config.yaml` to point directly to a local `~/.munin/screen.yaml`.
- Includes TV power scheduling (HDMI CEC) and tab cycling.

## 2. [Git Fleet Management (`2-git-fleet-management/`)](2-git-fleet-management/)
- Ideal for managing multiple screens (lobbies, meeting rooms, ops dashboards, canteens) across one or multiple offices.
- Shows what the node's `~/.munin/config.yaml` looks like.
- Contains a complete `sample-repo/` demonstrating how to organize folders per screen, share global stylesheets/logos, and sync scripts to node filesystems.
