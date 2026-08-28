package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/naueramant/munin/internal/utils"
	yaml "gopkg.in/yaml.v2"
)

// Load reads, parses, and applies sane defaults to a screen configuration file.
func Load(filename string) (*Configuration, error) {
	c := Configuration{}

	data, err := os.ReadFile(filename)
	if err != nil {
		return &c, fmt.Errorf("failed to load configuration file: %w", err)
	}

	err = yaml.Unmarshal(data, &c)
	if err != nil {
		return &c, fmt.Errorf("failed to unmarshal configuration file: %w", err)
	}

	// Sane defaults
	if c.Syntax == "" {
		c.Syntax = "v1"
	}

	// Default tab duration to 30s if cycling multiple tabs and duration is omitted
	if len(c.Tabs) > 1 {
		for i := range c.Tabs {
			if c.Tabs[i].Duration == 0 {
				c.Tabs[i].Duration = 30
			}
		}
	}

	err = Validate(c)
	if err != nil {
		return &c, fmt.Errorf("configuration file invalid: %w", err)
	}

	return &c, nil
}

// LoadAgentConfig loads an agent configuration file from explicit path or default ~/.munin/agent.yaml.
func LoadAgentConfig(explicitPath string) (*AgentConfig, error) {
	var targetPath string

	if explicitPath != "" {
		expanded := utils.ExpandHome(explicitPath)
		if _, err := os.Stat(expanded); err != nil {
			return nil, fmt.Errorf("agent config file not found at %s: %w", explicitPath, err)
		}
		targetPath = expanded
	} else {
		defaultPath := utils.ExpandHome("~/.munin/agent.yaml")
		if _, err := os.Stat(defaultPath); err != nil {
			return nil, os.ErrNotExist
		}
		targetPath = defaultPath
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent config from %s: %w", targetPath, err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse agent config from %s: %w", targetPath, err)
	}

	applyAgentConfigDefaults(&cfg)

	return &cfg, nil
}

func applyAgentConfigDefaults(cfg *AgentConfig) {
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if cfg.Mode == "" {
		if cfg.Git.Repo != "" {
			cfg.Mode = "git"
		} else {
			cfg.Mode = "local"
		}
	}

	if cfg.Display.Env == "" {
		cfg.Display.Env = ":0"
	}

	if cfg.Git.Branch == "" {
		cfg.Git.Branch = "main"
	}

	if cfg.Git.Schedule == "" {
		if cfg.Git.Interval != "" {
			cfg.Git.Schedule = cfg.Git.Interval
		} else {
			cfg.Git.Schedule = "* * * * *"
		}
	}

	if cfg.Git.TargetDir == "" {
		cfg.Git.TargetDir = "~/.munin/repo"
	}
	cfg.Git.TargetDir = utils.ExpandHome(cfg.Git.TargetDir)

	if cfg.Git.DeployKey != "" {
		cfg.Git.DeployKey = utils.ExpandHome(cfg.Git.DeployKey)
	} else {
		// Auto-discover standard SSH keys if present
		defaultKeys := []string{
			utils.ExpandHome("~/.ssh/id_munin_deploy"),
			utils.ExpandHome("~/.ssh/id_ed25519"),
			utils.ExpandHome("~/.ssh/id_rsa"),
		}
		for _, key := range defaultKeys {
			if _, err := os.Stat(key); err == nil {
				cfg.Git.DeployKey = key
				break
			}
		}
	}

	if cfg.Update.Schedule == "" {
		if cfg.Update.When != "" {
			cfg.Update.Schedule = cfg.Update.When
		} else if cfg.Update.Interval != "" {
			cfg.Update.Schedule = cfg.Update.Interval
		} else {
			cfg.Update.Schedule = "0 4 * * *"
		}
	}

	if cfg.ScreenPath != "" {
		cfg.ScreenPath = utils.ExpandHome(cfg.ScreenPath)
	} else if cfg.Mode == "local" {
		// Check ~/.munin/screen.yaml then local screen.yaml
		userScreen := utils.ExpandHome("~/.munin/screen.yaml")
		if _, err := os.Stat(userScreen); err == nil {
			cfg.ScreenPath = userScreen
		} else {
			cfg.ScreenPath = "screen.yaml"
		}
	}

	// Make sure TargetDir is absolute
	if abs, err := filepath.Abs(cfg.Git.TargetDir); err == nil {
		cfg.Git.TargetDir = abs
	}
}
