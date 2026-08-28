package config

import (
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
		t.Fatalf("expected error when ~/.munin/config.yaml does not exist")
	}

	// Create ~/.munin/config.yaml
	muninDir := filepath.Join(tmpHome, ".munin")
	if err := os.MkdirAll(muninDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(muninDir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("log_level: 'debug'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAgentConfig("")
	if err != nil {
		t.Fatalf("expected success reading ~/.munin/config.yaml, got %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug log level, got %s", cfg.LogLevel)
	}

	// Verify that another file (e.g. .munin.yaml) is NOT picked up
	if err := os.Remove(cfgFile); err != nil {
		t.Fatal(err)
	}
	otherFile := filepath.Join(tmpHome, ".munin.yaml")
	if err := os.WriteFile(otherFile, []byte("log_level: 'warn'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadAgentConfig("")
	if err == nil {
		t.Errorf("expected error since only ~/.munin/config.yaml is supported, but picked up another file")
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
