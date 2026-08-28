package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadValidScreenConfig(t *testing.T) {
	content := `
syntax: v1
tabs:
  - url: "https://example.com"
    duration: 30
    reload: true
power:
  turn_on: "0 7 * * 1-5"
  turn_off: "0 19 * * 1-5"
jobs:
  - when: "0 3 * * *"
    command: "sudo reboot"
files:
  - src: "scripts/test.sh"
    dest: "/tmp/test.sh"
    mode: "0755"
`
	tmpFile := filepath.Join(t.TempDir(), "screen.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("failed to load valid config: %v", err)
	}

	if len(cfg.Tabs) != 1 || cfg.Tabs[0].URL != "https://example.com" {
		t.Errorf("unexpected tabs: %+v", cfg.Tabs)
	}
	if cfg.Power.TurnOn != "0 7 * * 1-5" || cfg.Power.TurnOff != "0 19 * * 1-5" {
		t.Errorf("unexpected power config: %+v", cfg.Power)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].GetCommandLine() != "sudo reboot" {
		t.Errorf("unexpected jobs: %+v", cfg.Jobs)
	}
	if len(cfg.Files) != 1 || cfg.Files[0].Src != "scripts/test.sh" {
		t.Errorf("unexpected files: %+v", cfg.Files)
	}
}

func TestLoadScreenConfigWithPowerOptions(t *testing.T) {
	content := `
syntax: v1
power:
  screen_on: "18:00"
  screen_off: "10:00"
  reboot: "11:00"
  power_off: "0 22 * * 5"
  cec_device: 2
`
	tmpFile := filepath.Join(t.TempDir(), "screen.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("failed to load screen config: %v", err)
	}

	if cfg.Power.GetScreenOn() != "18:00" {
		t.Errorf("expected screen_on '18:00', got %q", cfg.Power.GetScreenOn())
	}
	if cfg.Power.GetScreenOff() != "10:00" {
		t.Errorf("expected screen_off '10:00', got %q", cfg.Power.GetScreenOff())
	}
	if cfg.Power.GetReboot() != "11:00" {
		t.Errorf("expected reboot '11:00', got %q", cfg.Power.GetReboot())
	}
	if cfg.Power.GetPowerOff() != "0 22 * * 5" {
		t.Errorf("expected power_off '0 22 * * 5', got %q", cfg.Power.GetPowerOff())
	}
	if cfg.Power.GetCecDevice() != 2 {
		t.Errorf("expected cec_device 2, got %d", cfg.Power.GetCecDevice())
	}
	if !cfg.Power.HasEntries() {
		t.Errorf("expected HasEntries to be true")
	}
}

func TestLoadAgentConfig(t *testing.T) {
	content := `
mode: "git"
git:
  repo: "git@github.com:test/repo.git"
  deploy_key: "~/.ssh/id_test"
  interval: "45s"
update:
  enabled: false
  interval: "12h"
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAgentConfig(tmpFile)
	if err != nil {
		t.Fatalf("failed to load agent config: %v", err)
	}

	if cfg.Mode != "git" {
		t.Errorf("expected mode git, got %s", cfg.Mode)
	}
	if cfg.Git.GetInterval() != 45*time.Second {
		t.Errorf("expected interval 45s, got %v", cfg.Git.GetInterval())
	}
	if cfg.Update.IsEnabled() {
		t.Errorf("expected auto update to be disabled")
	}
	if cfg.Update.GetInterval() != 12*time.Hour {
		t.Errorf("expected update interval 12h, got %v", cfg.Update.GetInterval())
	}
}

func TestLoadAgentConfigDefaultLocation(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Before file exists
	_, err := LoadAgentConfig("")
	if err == nil {
		t.Fatalf("expected error when ~/.munin/agent.yaml does not exist")
	}

	// Create ~/.munin/agent.yaml
	muninDir := filepath.Join(tmpHome, ".munin")
	if err := os.MkdirAll(muninDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(muninDir, "agent.yaml")
	if err := os.WriteFile(cfgFile, []byte("log_level: 'debug'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAgentConfig("")
	if err != nil {
		t.Fatalf("expected success reading ~/.munin/agent.yaml, got %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug log level, got %s", cfg.LogLevel)
	}

	// Verify that legacy config.yaml or .munin.yaml is NOT picked up (no legacy fallback)
	if err := os.Remove(cfgFile); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(muninDir, "config.yaml")
	if err := os.WriteFile(legacyFile, []byte("log_level: 'warn'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadAgentConfig("")
	if err == nil {
		t.Errorf("expected error since legacy ~/.munin/config.yaml should not be picked up")
	}
}

func TestSaneDefaults(t *testing.T) {
	// Minimal agent config
	minimalAgent := `
git:
  repo: "git@github.com:foo/bar.git"
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(minimalAgent), 0644); err != nil {
		t.Fatal(err)
	}

	agentCfg, err := LoadAgentConfig(tmpFile)
	if err != nil {
		t.Fatalf("failed to load minimal agent config: %v", err)
	}

	if agentCfg.Mode != "git" {
		t.Errorf("expected default mode 'git', got %s", agentCfg.Mode)
	}
	if agentCfg.LogLevel != "info" {
		t.Errorf("expected default log_level 'info', got %s", agentCfg.LogLevel)
	}
	if agentCfg.Display.Env != ":0" {
		t.Errorf("expected default display ':0', got %s", agentCfg.Display.Env)
	}
	if agentCfg.Git.Branch != "main" {
		t.Errorf("expected default branch 'main', got %s", agentCfg.Git.Branch)
	}
	if agentCfg.Git.GetSchedule() != "* * * * *" {
		t.Errorf("expected default git schedule '* * * * *', got %s", agentCfg.Git.GetSchedule())
	}
	if agentCfg.Update.GetSchedule() != "0 4 * * *" {
		t.Errorf("expected default update schedule '0 4 * * *', got %s", agentCfg.Update.GetSchedule())
	}
	if !agentCfg.Update.IsEnabled() {
		t.Errorf("expected update to be enabled by default")
	}

	// Minimal screen config with multi tabs (omitting duration)
	minimalScreen := `
tabs:
  - url: "https://site1.example.com"
  - url: "https://site2.example.com"
`
	screenFile := filepath.Join(t.TempDir(), "screen.yaml")
	if err := os.WriteFile(screenFile, []byte(minimalScreen), 0644); err != nil {
		t.Fatal(err)
	}

	screenCfg, err := Load(screenFile)
	if err != nil {
		t.Fatalf("failed to load minimal screen config: %v", err)
	}

	if screenCfg.Syntax != "v1" {
		t.Errorf("expected default syntax 'v1', got %s", screenCfg.Syntax)
	}
	if screenCfg.Tabs[0].Duration != 30 || screenCfg.Tabs[1].Duration != 30 {
		t.Errorf("expected default multi-tab duration 30s, got %d and %d", screenCfg.Tabs[0].Duration, screenCfg.Tabs[1].Duration)
	}
	if screenCfg.Power.GetCecDevice() != 0 {
		t.Errorf("expected default CEC device 0, got %d", screenCfg.Power.GetCecDevice())
	}
}

func TestLoadNonExistentScreenConfig(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatalf("expected error when screen config does not exist")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected errors.Is(err, os.ErrNotExist) to be true, got %v", err)
	}
}

func TestExamplesValidity(t *testing.T) {
	// 1. Standalone agent.yaml
	agentCfg1, err := LoadAgentConfig("../../examples/1-local-standalone/agent.yaml")
	if err != nil {
		t.Fatalf("failed to load examples/1-local-standalone/agent.yaml: %v", err)
	}
	if agentCfg1.Mode != "local" {
		t.Errorf("expected mode 'local', got %q", agentCfg1.Mode)
	}

	// 2. Standalone screen.yaml
	screenCfg1, err := Load("../../examples/1-local-standalone/screen.yaml")
	if err != nil {
		t.Fatalf("failed to load examples/1-local-standalone/screen.yaml: %v", err)
	}
	if len(screenCfg1.Tabs) != 2 {
		t.Errorf("expected 2 tabs in standalone screen.yaml, got %d", len(screenCfg1.Tabs))
	}

	// 3. Fleet node-agent.yaml
	fleetAgent, err := LoadAgentConfig("../../examples/2-git-fleet-management/node-agent.yaml")
	if err != nil {
		t.Fatalf("failed to load examples/2-git-fleet-management/node-agent.yaml: %v", err)
	}
	if fleetAgent.Mode != "git" {
		t.Errorf("expected mode 'git', got %q", fleetAgent.Mode)
	}

	// 4. Fleet office-lobby screen.yaml
	lobbyScreen, err := Load("../../examples/2-git-fleet-management/sample-repo/screens/office-lobby/screen.yaml")
	if err != nil {
		t.Fatalf("failed to load office-lobby screen.yaml: %v", err)
	}
	if len(lobbyScreen.Tabs) != 2 {
		t.Errorf("expected 2 tabs in office-lobby, got %d", len(lobbyScreen.Tabs))
	}

	// 5. Fleet ops-dashboard screen.yaml
	opsScreen, err := Load("../../examples/2-git-fleet-management/sample-repo/screens/ops-dashboard/screen.yaml")
	if err != nil {
		t.Fatalf("failed to load ops-dashboard screen.yaml: %v", err)
	}
	if len(opsScreen.Jobs) != 1 {
		t.Errorf("expected 1 job in ops-dashboard, got %d", len(opsScreen.Jobs))
	}
}

