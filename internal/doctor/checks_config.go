package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/naueramant/munin/internal/config"
	"github.com/naueramant/munin/internal/cron"
	"github.com/naueramant/munin/internal/utils"
)

// checkConfigAndGit inspects configuration files, crontab state, and Git synchronization setup.
func checkConfigAndGit(opts Options) []CheckResult {
	var results []CheckResult

	// 1. Agent configuration
	agentCfg, agentRes := checkAgentConfig(opts.AgentConfigPath)
	results = append(results, agentRes...)

	// 2. Screen configuration
	screenPath, screenCfg, screenRes := checkScreenConfig(opts, agentCfg)
	results = append(results, screenRes...)

	// 3. User crontab inspection
	results = append(results, checkCrontabState(screenCfg))

	// 4. Git Fleet configuration & deploy keys (if in Git mode)
	if agentCfg != nil && agentCfg.Mode == "git" {
		results = append(results, checkGitFleet(opts, agentCfg)...)
	}

	_ = screenPath
	return results
}

func checkAgentConfig(explicitPath string) (*config.AgentConfig, []CheckResult) {
	var results []CheckResult

	targetPath := explicitPath
	if targetPath == "" {
		targetPath = utils.ExpandHome("~/.munin/agent.yaml")
	} else {
		targetPath = utils.ExpandHome(targetPath)
	}

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		if explicitPath != "" {
			results = append(results, CheckResult{
				Category: CategoryConfig,
				Name:     "Agent Configuration",
				Status:   StatusError,
				Message:  fmt.Sprintf("Specified agent config file does not exist: %s", targetPath),
			})
		} else {
			results = append(results, CheckResult{
				Category: CategoryConfig,
				Name:     "Agent Configuration",
				Status:   StatusInfo,
				Message:  fmt.Sprintf("No agent config found at %s (running in ad-hoc/local mode)", targetPath),
				Detail:   "Run `munin init` to generate a standard agent configuration.",
			})
		}
		return nil, results
	}

	cfg, err := config.LoadAgentConfig(explicitPath)
	if err != nil {
		results = append(results, CheckResult{
			Category: CategoryConfig,
			Name:     "Agent Configuration",
			Status:   StatusError,
			Message:  fmt.Sprintf("Failed to parse %s: %v", targetPath, err),
		})
		return nil, results
	}

	results = append(results, CheckResult{
		Category: CategoryConfig,
		Name:     "Agent Configuration",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Valid configuration at %s (mode: %s, log_level: %s)", targetPath, cfg.Mode, cfg.LogLevel),
	})

	return cfg, results
}

func checkScreenConfig(opts Options, agentCfg *config.AgentConfig) (string, *config.Configuration, []CheckResult) {
	var results []CheckResult
	var screenPath string

	if opts.ScreenPath != "" {
		screenPath = utils.ExpandHome(opts.ScreenPath)
	} else if agentCfg != nil {
		if agentCfg.Mode == "local" && agentCfg.ScreenPath != "" {
			screenPath = utils.ExpandHome(agentCfg.ScreenPath)
		} else if agentCfg.Mode == "git" {
			screenPath = filepath.Join(agentCfg.Git.TargetDir, agentCfg.Git.Subdir, "screen.yaml")
		}
	}

	if screenPath == "" {
		// Try default paths
		candidates := []string{
			utils.ExpandHome("~/.munin/screen.yaml"),
			"screen.yaml",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				screenPath = c
				break
			}
		}
	}

	if screenPath == "" {
		results = append(results, CheckResult{
			Category: CategoryConfig,
			Name:     "Screen Configuration",
			Status:   StatusWarn,
			Message:  "No screen.yaml found",
			Detail:   "Munin displays tabs defined in screen.yaml. For git mode, this file should reside in the git repo subdirectory.",
			FixHint:  "Create screen.yaml or run `munin init`",
		})
		return "", nil, results
	}

	if _, err := os.Stat(screenPath); os.IsNotExist(err) {
		results = append(results, CheckResult{
			Category: CategoryConfig,
			Name:     "Screen Configuration",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Screen configuration file not found at %s", screenPath),
			Detail:   "In git mode, the file will be populated once the repository is cloned.",
		})
		return screenPath, nil, results
	}

	cfg, err := config.Load(screenPath)
	if err != nil {
		results = append(results, CheckResult{
			Category: CategoryConfig,
			Name:     "Screen Configuration",
			Status:   StatusError,
			Message:  fmt.Sprintf("Failed to load/validate screen config at %s", screenPath),
			Detail:   err.Error(),
		})
		return screenPath, nil, results
	}

	numTabs := len(cfg.Tabs)
	hasPower := cfg.Power.TurnOn != "" || cfg.Power.TurnOff != ""
	numJobs := len(cfg.Jobs)

	details := fmt.Sprintf("%d tabs", numTabs)
	if hasPower {
		details += ", HDMI CEC power schedules enabled"
	}
	if numJobs > 0 {
		details += fmt.Sprintf(", %d custom jobs", numJobs)
	}

	results = append(results, CheckResult{
		Category: CategoryConfig,
		Name:     "Screen Configuration",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Valid screen definition at %s (%s)", screenPath, details),
	})

	return screenPath, cfg, results
}

func checkCrontabState(screenCfg *config.Configuration) CheckResult {
	if _, err := exec.LookPath("crontab"); err != nil {
		return CheckResult{
			Category: CategoryConfig,
			Name:     "User Crontab",
			Status:   StatusInfo,
			Message:  "crontab command not installed; crontab inspection skipped",
		}
	}

	cmd := exec.Command("crontab", "-l")
	out, err := cmd.CombinedOutput()
	outStr := string(out)

	if err != nil {
		if strings.Contains(outStr, "no crontab for") {
			if screenCfg != nil && (screenCfg.Power.TurnOn != "" || screenCfg.Power.TurnOff != "" || len(screenCfg.Jobs) > 0) {
				return CheckResult{
					Category: CategoryConfig,
					Name:     "User Crontab",
					Status:   StatusWarn,
					Message:  "No user crontab exists, but screen.yaml defines power schedules or jobs",
					Detail:   "Munin will install cron entries when running.",
				}
			}
			return CheckResult{
				Category: CategoryConfig,
				Name:     "User Crontab",
				Status:   StatusOK,
				Message:  "No active user crontab (empty)",
			}
		}

		return CheckResult{
			Category: CategoryConfig,
			Name:     "User Crontab",
			Status:   StatusWarn,
			Message:  "Unable to read user crontab",
			Detail:   strings.TrimSpace(outStr),
		}
	}

	hasMuninBlock := strings.Contains(outStr, cron.BeginMarker) || strings.Contains(outStr, "# --- BEGIN MIMIR MANAGED JOBS")

	if hasMuninBlock {
		return CheckResult{
			Category: CategoryConfig,
			Name:     "User Crontab",
			Status:   StatusOK,
			Message:  "Munin-managed cron block is present in user crontab",
		}
	}

	if screenCfg != nil && (screenCfg.Power.TurnOn != "" || screenCfg.Power.TurnOff != "" || len(screenCfg.Jobs) > 0) {
		return CheckResult{
			Category: CategoryConfig,
			Name:     "User Crontab",
			Status:   StatusWarn,
			Message:  "Screen config defines power/jobs, but Munin block is not yet installed in crontab",
			Detail:   "Munin will automatically synchronize crontab entries when the agent runs.",
		}
	}

	return CheckResult{
		Category: CategoryConfig,
		Name:     "User Crontab",
		Status:   StatusOK,
		Message:  "Crontab active (no Munin jobs required)",
	}
}

func checkGitFleet(opts Options, agentCfg *config.AgentConfig) []CheckResult {
	var results []CheckResult

	if agentCfg.Git.Repo == "" {
		results = append(results, CheckResult{
			Category: CategoryGit,
			Name:     "Git Repository URL",
			Status:   StatusError,
			Message:  "git.repo is not configured in agent.yaml",
		})
		return results
	}

	results = append(results, CheckResult{
		Category: CategoryGit,
		Name:     "Git Repository URL",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Tracking %s (branch: %s, schedule: %s)", agentCfg.Git.Repo, agentCfg.Git.Branch, agentCfg.Git.Schedule),
	})

	// Check Deploy Key
	keyPath := agentCfg.Git.DeployKey
	if keyPath != "" {
		fi, err := os.Stat(keyPath)
		if err != nil {
			results = append(results, CheckResult{
				Category: CategoryGit,
				Name:     "SSH Deploy Key",
				Status:   StatusError,
				Message:  fmt.Sprintf("Deploy key not found at %s", keyPath),
				FixHint:  "Generate an SSH key: ssh-keygen -t ed25519 -f ~/.ssh/id_munin_deploy",
			})
		} else {
			perm := fi.Mode().Perm()
			// SSH requires private keys to be 0600 or 0400
			if perm&0077 != 0 {
				res := CheckResult{
					Category: CategoryGit,
					Name:     "SSH Deploy Key Permissions",
					Status:   StatusWarn,
					Message:  fmt.Sprintf("Permissions on %s are too open (%#o); SSH requires 0600", keyPath, perm),
					Fixable:  true,
					FixHint:  fmt.Sprintf("chmod 600 %s", keyPath),
				}
				if opts.Fix {
					if chmodErr := os.Chmod(keyPath, 0600); chmodErr == nil {
						res.FixApplied = true
						res.Status = StatusOK
						res.Message = fmt.Sprintf("Permissions on %s corrected to 0600", keyPath)
					}
				}
				results = append(results, res)
			} else {
				results = append(results, CheckResult{
					Category: CategoryGit,
					Name:     "SSH Deploy Key",
					Status:   StatusOK,
					Message:  fmt.Sprintf("Found %s (permissions: %#o)", keyPath, perm),
				})
			}
		}
	}

	// Check remote connectivity if git command exists
	if _, err := exec.LookPath("git"); err == nil {
		results = append(results, checkGitRemoteReachability(agentCfg)...)
	}

	return results
}

func checkGitRemoteReachability(agentCfg *config.AgentConfig) []CheckResult {
	var results []CheckResult

	// Skip remote network probe for dummy/example test repository URLs
	if strings.Contains(agentCfg.Git.Repo, "example") || strings.Contains(agentCfg.Git.Repo, "test.local") {
		return results
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", agentCfg.Git.Repo)
	if agentCfg.Git.DeployKey != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=accept-new", agentCfg.Git.DeployKey))
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			results = append(results, CheckResult{
				Category: CategoryGit,
				Name:     "Remote Connectivity",
				Status:   StatusWarn,
				Message:  fmt.Sprintf("Timed out connecting to git repository '%s'", agentCfg.Git.Repo),
				Detail:   "Check internet connection or SSH deploy key authentication.",
			})
		} else {
			results = append(results, CheckResult{
				Category: CategoryGit,
				Name:     "Remote Connectivity",
				Status:   StatusWarn,
				Message:  fmt.Sprintf("Failed to query remote repository '%s'", agentCfg.Git.Repo),
				Detail:   strings.TrimSpace(string(out)),
				FixHint:  "Ensure your SSH deploy key is added to the remote Git provider with read access.",
			})
		}
		return results
	}

	results = append(results, CheckResult{
		Category: CategoryGit,
		Name:     "Remote Connectivity",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Successfully connected to remote repository '%s'", agentCfg.Git.Repo),
	})

	return results
}
