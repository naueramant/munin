package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/naueramant/munin/internal/assets"
	"github.com/naueramant/munin/internal/browser"
	"github.com/naueramant/munin/internal/config"
	"github.com/naueramant/munin/internal/cron"
	"github.com/naueramant/munin/internal/doctor"
	"github.com/naueramant/munin/internal/filesync"
	"github.com/naueramant/munin/internal/git"
	"github.com/naueramant/munin/internal/updater"
	"github.com/naueramant/munin/internal/utils"
	"github.com/naueramant/munin/internal/wizard"
)

var (
	flagAgentConfig    = flag.String("agent-config", "", "path to host agent configuration (defaults to ~/.munin/agent.yaml)")
	flagConfig         = flag.String("config", "", "path to local screen.yaml (runs in local mode without git)")
	flagGitSchedule    = flag.String("git-schedule", "", "override git sync cron expression (e.g. '* * * * *', '*/5 * * * *')")
	flagGitInterval    = flag.String("git-interval", "", "alias for git-schedule")
	flagUpdateSchedule = flag.String("update-schedule", "", "override auto-update cron expression (e.g. '0 4 * * *')")
	flagUpdateWhen     = flag.String("update-when", "", "alias for update-schedule")
	flagUpdateInterval = flag.String("update-interval", "", "alias for update-schedule")
	flagLogLevel       = flag.String("log-level", "", "log level (debug, info, warn, error)")
	flagVersion        = flag.Bool("version", false, "display munin version and exit")

	bmLock sync.Mutex
	bm     *browser.BrowserManager
	as     *assets.Server
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			if err := wizard.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "remove", "uninstall":
			runRemoveCommand(os.Args[2:])
			return
		case "doctor":
			runDoctorCommand(os.Args[2:])
			return
		case "power-check":
			runPowerCheckCommand(os.Args[2:])
			return
		}
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Munin - Autonomous Screen Agent for Raspberry Pi and Linux kiosks\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  munin [flags]               Run screen agent\n")
		fmt.Fprintf(os.Stderr, "  munin init                  Launch interactive setup wizard\n")
		fmt.Fprintf(os.Stderr, "  munin doctor [options]      Diagnose dependencies, systemd services, permissions, and configuration\n")
		fmt.Fprintf(os.Stderr, "  munin power-check [options] Check screen power schedule and edge case state\n")
		fmt.Fprintf(os.Stderr, "  munin remove [options]      Remove Munin service, crontab, and configuration\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *flagVersion {
		fmt.Printf("munin version %s\n", updater.CurrentVersion)
		return
	}


	// Try loading agent config early to read LogLevel if set
	agentCfg, mode, screenPath := determineMode()

	// Configure structured logger (slog) with specified level
	initLogging(agentCfg)

	slog.Info("Starting munin screen agent", "version", updater.CurrentVersion, "mode", mode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigChan
		slog.Info("Received shutdown signal, terminating", "signal", sig.String())
		cancel()
		stopBrowser()
		os.Exit(0)
	}()

	// Start local assets server
	var serverErr error
	as, serverErr = assets.NewServer()
	if serverErr != nil {
		slog.Error("Failed to initialize assets server", "error", serverErr)
		os.Exit(1)
	}
	go func() {
		if err := as.Start(); err != nil {
			slog.Error("Assets server terminated", "error", err)
		}
	}()

	if agentCfg != nil && agentCfg.Display.Env != "" {
		_ = os.Setenv("DISPLAY", agentCfg.Display.Env)
	}

	// Start auto-updater if enabled
	if agentCfg != nil && agentCfg.Update.IsEnabled() {
		if *flagUpdateSchedule != "" {
			agentCfg.Update.Schedule = *flagUpdateSchedule
		} else if *flagUpdateWhen != "" {
			agentCfg.Update.Schedule = *flagUpdateWhen
		} else if *flagUpdateInterval != "" {
			agentCfg.Update.Schedule = *flagUpdateInterval
		}

		up := updater.NewUpdater(agentCfg.Update)
		go up.Start(ctx, func(newVer string) {
			slog.Info("Restarting agent to run newly downloaded version", "version", newVer)
			stopBrowser()
			os.Exit(0)
		})
	}

	if mode == "git" {
		runGitMode(ctx, agentCfg)
	} else {
		runLocalMode(ctx, screenPath, agentCfg)
	}
}

func initLogging(agentCfg *config.AgentConfig) {
	logLevelStr := "info"

	// Precedence: 1. CLI flag, 2. AgentConfig, 3. LOG_LEVEL env, 4. default "info"
	if *flagLogLevel != "" {
		logLevelStr = *flagLogLevel
	} else if agentCfg != nil && agentCfg.LogLevel != "" {
		logLevelStr = agentCfg.LogLevel
	} else if env := os.Getenv("LOG_LEVEL"); env != "" {
		logLevelStr = env
	}

	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(logLevelStr)) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

func determineMode() (*config.AgentConfig, string, string) {
	// 1. If --config is explicitly provided, always run in Local Mode
	if *flagConfig != "" {
		expanded := utils.ExpandHome(*flagConfig)
		return nil, "local", expanded
	}

	// 2. Try loading agent config
	agentCfg, err := config.LoadAgentConfig(*flagAgentConfig)
	if err == nil && agentCfg != nil {
		if *flagGitSchedule != "" {
			agentCfg.Git.Schedule = *flagGitSchedule
		} else if *flagGitInterval != "" {
			agentCfg.Git.Schedule = *flagGitInterval
		}

		if agentCfg.Mode == "git" && agentCfg.Git.Repo != "" {
			return agentCfg, "git", ""
		}

		screenPath := agentCfg.ScreenPath
		if screenPath == "" {
			screenPath = "screen.yaml"
		}
		return agentCfg, "local", screenPath
	}

	// 3. Fallback: check if screen.yaml exists in current directory
	if _, err := os.Stat("screen.yaml"); err == nil {
		return nil, "local", "screen.yaml"
	}

	return nil, "local", "screen.yaml"
}

func runLocalMode(ctx context.Context, screenPath string, agentCfg *config.AgentConfig) {
	if abs, err := filepath.Abs(screenPath); err == nil {
		screenPath = abs
	}
	baseDir := filepath.Dir(screenPath)
	var chromiumFlags []string
	if agentCfg != nil {
		chromiumFlags = agentCfg.Display.ChromiumFlags
	}

	applyConfig(screenPath, baseDir, chromiumFlags)

	// Watch screen.yaml for changes
	slog.Debug("Watching local configuration for changes", "file", screenPath)
	utils.Watch(ctx, screenPath, func() {
		slog.Info("Local screen configuration modified, reloading", "path", screenPath)
		applyConfig(screenPath, baseDir, chromiumFlags)
	})

	<-ctx.Done()
}

func runGitMode(ctx context.Context, agentCfg *config.AgentConfig) {
	syncer, err := git.NewSyncer(agentCfg.Git)
	if err != nil {
		slog.Error("Failed to initialize git syncer", "error", err)
		os.Exit(1)
	}

	syncAndApply := func() {
		changed, subdirPath, err := syncer.Sync()
		if err != nil {
			slog.Warn("Git sync encountered warning", "error", err)
		}

		if changed && subdirPath != "" {
			screenFile := filepath.Join(subdirPath, "screen.yaml")
			if _, err := os.Stat(screenFile); err != nil {
				screenFile = filepath.Join(subdirPath, "screen.yml")
			}
			slog.Info("Applying updated configuration from Git", "file", screenFile)
			applyConfig(screenFile, subdirPath, agentCfg.Display.ChromiumFlags)
		}
	}

	// Initial sync on startup
	syncAndApply()

	// Cron-based git sync loop
	schedule := agentCfg.Git.GetSchedule()
	slog.Info("Git sync scheduled", "cron", schedule)

	for {
		delay := utils.ComputeNextCronDelay(schedule, "* * * * *", time.Now())
		slog.Debug("Next git sync scheduled", "delay", delay.Round(time.Second))

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			syncAndApply()
		}
	}
}

func applyConfig(screenFile, baseDir string, extraChromiumFlags []string) {
	bmLock.Lock()
	defer bmLock.Unlock()

	c, err := config.Load(screenFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) || strings.Contains(err.Error(), "no such file or directory") {
			slog.Warn("Screen configuration file not found", "file", screenFile)
			c = &config.Configuration{Syntax: ""}
		} else {
			slog.Error("Failed to load screen configuration", "file", screenFile, "error", err)
			return
		}
	}

	// 1. Synchronize declared files
	if c.Syntax != "" && len(c.Files) > 0 {
		if err := filesync.SyncFiles(baseDir, c.Files); err != nil {
			slog.Warn("File synchronization encountered errors", "error", err)
		}
	}

	// 2. Update native user crontab
	if c.Syntax != "" && (c.Power.HasEntries() || len(c.Jobs) > 0) {
		if err := cron.UpdateCrontab(c.Power, c.Jobs); err != nil {
			slog.Warn("Failed to update native crontab", "error", err)
		}
	}

	// 3. Reconcile screen power state (e.g. after reboot when TV may auto turn on)
	if c.Syntax != "" && c.Power.HasEntries() {
		cron.ReconcilePowerState(c.Power)
	}

	// 4. Update browser screen live
	if bm == nil {
		bm = browser.NewBrowserManager(c, as, baseDir, extraChromiumFlags...)
		go bm.Start()
	} else {
		bm.ApplyConfig(c)
	}
}

func stopBrowser() {
	bmLock.Lock()
	defer bmLock.Unlock()
	if bm != nil {
		bm.Close()
		bm = nil
	}
}

func runRemoveCommand(args []string) {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	var (
		flagYes        = fs.Bool("yes", false, "automatic yes to prompts (non-interactive)")
		flagY          = fs.Bool("y", false, "alias for --yes")
		flagForce      = fs.Bool("force", false, "alias for --yes")
		flagF          = fs.Bool("f", false, "alias for --yes")
		flagPurge      = fs.Bool("purge", false, "purge configuration, data (~/.munin), deploy keys, and binary")
		flagKeepConfig = fs.Bool("keep-config", false, "do not remove ~/.munin configuration and data")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: munin remove [options]\n\n")
		fmt.Fprintf(os.Stderr, "Remove Munin service, crontab entries, and configuration from the system.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -y, --yes         Automatic yes to prompts (non-interactive)\n")
		fmt.Fprintf(os.Stderr, "  -f, --force       Alias for --yes\n")
		fmt.Fprintf(os.Stderr, "      --purge       Purge configuration, data (~/.munin), deploy keys, and binary\n")
		fmt.Fprintf(os.Stderr, "      --keep-config Do not remove configuration directory (~/.munin)\n")
		fmt.Fprintf(os.Stderr, "  -h, --help        Show this help message\n")
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing remove flags: %v\n", err)
		os.Exit(1)
	}

	opts := wizard.RemoveOptions{
		Force:      *flagYes || *flagY || *flagForce || *flagF,
		Purge:      *flagPurge,
		KeepConfig: *flagKeepConfig,
	}

	if err := wizard.RunRemove(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runPowerCheckCommand(args []string) {
	fs := flag.NewFlagSet("power-check", flag.ExitOnError)
	configPath := fs.String("config", "", "path to screen.yaml")
	enforce := fs.Bool("enforce", false, "send CEC standby command if screen is scheduled to be off")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: munin power-check [options]\n\n")
		fmt.Fprintf(os.Stderr, "Check screen power schedule and evaluate if the screen should be off.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  --config string   Path to screen.yaml (optional, defaults to agent/local discovery)\n")
		fmt.Fprintf(os.Stderr, "  --enforce         Send CEC standby command if screen is determined to be off\n")
		fmt.Fprintf(os.Stderr, "  -h, --help        Show this help message\n")
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing power-check flags: %v\n", err)
		os.Exit(1)
	}

	targetPath := *configPath
	if targetPath == "" {
		_, _, targetPath = determineMode()
	}

	c, err := config.Load(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading screen config from %s: %v\n", targetPath, err)
		os.Exit(1)
	}

	isOff := cron.ShouldScreenBeOff(c.Power, time.Now())
	fmt.Printf("Config file: %s\n", targetPath)
	fmt.Printf("Power options: screen_on=%q, screen_off=%q, reboot=%q, power_off=%q, cec_device=%d\n",
		c.Power.GetScreenOn(), c.Power.GetScreenOff(), c.Power.GetReboot(), c.Power.GetPowerOff(), c.Power.GetCecDevice())
	fmt.Printf("Current time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Screen should be OFF: %t\n", isOff)

	if isOff && *enforce {
		fmt.Println("Enforcing TV standby via CEC...")
		if err := cron.StandbyScreen(c.Power.GetCecDevice()); err != nil {
			fmt.Fprintf(os.Stderr, "Notice: could not send CEC standby: %v\n", err)
		} else {
			fmt.Println("[✓] Sent CEC standby command successfully.")
		}
	}
}

func runDoctorCommand(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	flagFix := fs.Bool("fix", false, "attempt automatic fixes for detected issues where possible")
	flagJSON := fs.Bool("json", false, "output diagnostic results in JSON format")
	flagVerbose := fs.Bool("verbose", false, "display detailed diagnostic information")
	flagV := fs.Bool("v", false, "alias for --verbose")
	flagAgentConfig := fs.String("agent-config", "", "path to agent.yaml (defaults to ~/.munin/agent.yaml)")
	flagConfig := fs.String("config", "", "path to screen.yaml")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: munin doctor [options]\n\n")
		fmt.Fprintf(os.Stderr, "Diagnose system dependencies, systemd services, permissions, and configuration.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "      --fix           Attempt automatic fixes for detected issues where possible\n")
		fmt.Fprintf(os.Stderr, "      --json          Output diagnostic results in JSON format\n")
		fmt.Fprintf(os.Stderr, "  -v, --verbose       Display detailed diagnostic information\n")
		fmt.Fprintf(os.Stderr, "      --agent-config  Path to agent.yaml (defaults to ~/.munin/agent.yaml)\n")
		fmt.Fprintf(os.Stderr, "      --config        Path to screen.yaml\n")
		fmt.Fprintf(os.Stderr, "  -h, --help          Show this help message\n")
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing doctor flags: %v\n", err)
		os.Exit(1)
	}

	opts := doctor.Options{
		Fix:             *flagFix,
		JSON:            *flagJSON,
		Verbose:         *flagVerbose || *flagV,
		AgentConfigPath: *flagAgentConfig,
		ScreenPath:      *flagConfig,
	}

	doc := doctor.New(opts)
	report := doc.Run()

	if err := doc.Render(os.Stdout, report); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering report: %v\n", err)
		os.Exit(1)
	}

	if report.HasErrors() {
		os.Exit(1)
	}
}

