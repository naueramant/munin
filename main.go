package main

import (
	"context"
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
	"github.com/naueramant/munin/internal/filesync"
	"github.com/naueramant/munin/internal/git"
	"github.com/naueramant/munin/internal/updater"
	"github.com/naueramant/munin/internal/utils"
	"github.com/naueramant/munin/internal/wizard"
)

var (
	flagAgentConfig    = flag.String("agent-config", "", "path to host agent configuration (defaults to ~/.munin/config.yaml)")
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
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := wizard.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
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
		slog.Error("Failed to load screen configuration", "file", screenFile, "error", err)
		return
	}

	// 1. Synchronize declared files
	if len(c.Files) > 0 {
		if err := filesync.SyncFiles(baseDir, c.Files); err != nil {
			slog.Warn("File synchronization encountered errors", "error", err)
		}
	}

	// 2. Update native user crontab
	if c.Power.TurnOn != "" || c.Power.TurnOff != "" || len(c.Jobs) > 0 {
		if err := cron.UpdateCrontab(c.Power, c.Jobs); err != nil {
			slog.Warn("Failed to update native crontab", "error", err)
		}
	}

	// 3. Restart browser with updated screen
	if bm != nil {
		bm.Close()
	}

	bm = browser.NewBrowserManager(c, as, baseDir, extraChromiumFlags...)
	go bm.Start()
}

func stopBrowser() {
	bmLock.Lock()
	defer bmLock.Unlock()
	if bm != nil {
		bm.Close()
		bm = nil
	}
}
