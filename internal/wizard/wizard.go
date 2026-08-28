package wizard

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/naueramant/munin/internal/utils"
	yaml "gopkg.in/yaml.v2"
)

//go:embed templates/munin.service
var defaultServiceTemplate []byte

//go:embed templates/agent.yaml
var defaultAgentTemplate []byte

//go:embed templates/screen.yaml
var defaultScreenTemplate []byte


// Run executes the interactive setup wizard.
func Run() error {
	reader := bufio.NewReader(os.Stdin)

	printBanner()

	// Check if stdin is a terminal/TTY
	stat, _ := os.Stdin.Stat()
	isTTY := (stat.Mode() & os.ModeCharDevice) != 0

	configDir := utils.ExpandHome("~/.munin")
	configFile := filepath.Join(configDir, "agent.yaml")

	if !isTTY {
		fmt.Println("Non-interactive environment detected. Creating standard starter configuration...")
		return createDefaultConfig(configFile)
	}

	fmt.Println("Welcome to Munin! This wizard will guide you through setting up your screen agent.")
	fmt.Println()

	// 1. Choose Operating Mode
	fmt.Println("Choose operating mode:")
	fmt.Println("  [1] Git Sync Mode        (Recommended: manage one or many screens from a Git repo)")
	fmt.Println("  [2] Local Standalone Mode (Single screen: edit a local screen.yaml directly)")
	modeChoice := prompt(reader, "Select mode [1/2]", "1")

	isGit := modeChoice == "1"

	var repoURL, repoSubdir, repoBranch, repoSchedule, deployKey string
	var screenPath string

	if isGit {
		fmt.Println("\n--- Git Fleet Sync Configuration ---")
		repoURL = prompt(reader, "Git repository URL (SSH or HTTPS)", "git@github.com:myorg/office-screens.git")
		repoSubdir = prompt(reader, "Subdirectory in repo for this screen", "screens/default")
		repoBranch = prompt(reader, "Git branch to track", "main")
		repoSchedule = prompt(reader, "Sync cron schedule (* * * * * = every min)", "* * * * *")

		// Deploy Key discovery or generation
		defaultKey := findDefaultSSHKey()
		if defaultKey != "" {
			useExisting := prompt(reader, fmt.Sprintf("Found existing SSH key at %s. Use this key? (Y/n)", defaultKey), "Y")
			if strings.EqualFold(useExisting, "y") || strings.EqualFold(useExisting, "yes") {
				deployKey = defaultKey
			}
		}

		if deployKey == "" {
			genKey := prompt(reader, "Generate a new dedicated SSH deploy key at ~/.ssh/id_munin_deploy? (Y/n)", "Y")
			if strings.EqualFold(genKey, "y") || strings.EqualFold(genKey, "yes") {
				newKeyPath, pubKey, err := generateSSHDeployKey()
				if err != nil {
					fmt.Printf("Warning: Failed to generate SSH key: %v\n", err)
				} else {
					deployKey = newKeyPath
					fmt.Println("\n========================================================")
					fmt.Println("  NEW SSH DEPLOY KEY GENERATED!")
					fmt.Printf("  Private Key: %s\n", newKeyPath)
					fmt.Println("  Add this PUBLIC KEY to your Git repository Deploy Keys:")
					fmt.Println("--------------------------------------------------------")
					fmt.Println(strings.TrimSpace(pubKey))
					fmt.Println("========================================================")
					fmt.Println()
					prompt(reader, "Press Enter after you have copied the public key or added it to GitHub...", "")
				}
			}
		}
	} else {
		fmt.Println("\n--- Local Standalone Configuration ---")
		screenPath = prompt(reader, "Path to local screen.yaml", "~/.munin/screen.yaml")
		expandedScreen := utils.ExpandHome(screenPath)

		if _, err := os.Stat(expandedScreen); os.IsNotExist(err) {
			createSample := prompt(reader, fmt.Sprintf("File %s does not exist. Create starter screen.yaml? (Y/n)", screenPath), "Y")
			if strings.EqualFold(createSample, "y") || strings.EqualFold(createSample, "yes") {
				if err := writeSampleScreenYAML(expandedScreen); err != nil {
					fmt.Printf("Warning: failed to create sample screen.yaml: %v\n", err)
				} else {
					fmt.Printf("[✓] Created starter screen file at %s\n", expandedScreen)
				}
			}
		}
	}

	// Auto-update configuration
	fmt.Println("\n--- Auto-Update Configuration ---")
	updateChoice := prompt(reader, "Enable automatic updates from GitHub Releases? (Y/n)", "Y")
	updateEnabled := strings.EqualFold(updateChoice, "y") || strings.EqualFold(updateChoice, "yes")

	updateSchedule := "0 4 * * *"
	if updateEnabled {
		updateSchedule = prompt(reader, "Auto-update cron schedule (0 4 * * * = daily at 04:00)", "0 4 * * *")
	}

	// Write ~/.munin/agent.yaml
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configMap := make(map[string]interface{})
	if isGit {
		configMap["mode"] = "git"
		configMap["log_level"] = "info"
		gitMap := map[string]interface{}{
			"repo":     repoURL,
			"branch":   repoBranch,
			"subdir":   repoSubdir,
			"schedule": repoSchedule,
		}
		if deployKey != "" {
			gitMap["deploy_key"] = deployKey
		}
		configMap["git"] = gitMap
	} else {
		configMap["mode"] = "local"
		configMap["log_level"] = "info"
		configMap["screen_path"] = screenPath
	}

	configMap["update"] = map[string]interface{}{
		"enabled":  updateEnabled,
		"repo":     "naueramant/munin",
		"schedule": updateSchedule,
	}

	configMap["display"] = map[string]interface{}{
		"env": ":0",
	}

	yamlData, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to encode configuration: %w", err)
	}

	header := "# Munin Agent Configuration\n# Generated by `munin init`\n\n"
	if err := os.WriteFile(configFile, []byte(header+string(yamlData)), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", configFile, err)
	}

	fmt.Printf("\n[✓] Saved configuration to %s\n", configFile)

	// Systemd User Service setup
	fmt.Println("\n--- Systemd User Service Setup ---")
	svcChoice := prompt(reader, "Install and enable Munin as a systemd user service? (Y/n)", "Y")
	if strings.EqualFold(svcChoice, "y") || strings.EqualFold(svcChoice, "yes") {
		if err := installSystemdUserService(); err != nil {
			fmt.Printf("Notice: Could not automatically setup user service: %v\n", err)
			fmt.Println("You can run munin directly by typing: munin")
		} else {
			fmt.Println("[✓] Systemd user service installed and enabled!")
		}
	}

	printSummary(configFile, isGit)
	return nil
}

func prompt(r *bufio.Reader, message string, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", message, defaultValue)
	} else {
		fmt.Printf("%s: ", message)
	}

	input, err := r.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return defaultValue
	}
	return trimmed
}

func findDefaultSSHKey() string {
	candidates := []string{
		utils.ExpandHome("~/.ssh/id_munin_deploy"),
		utils.ExpandHome("~/.ssh/id_ed25519"),
		utils.ExpandHome("~/.ssh/id_rsa"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func generateSSHDeployKey() (string, string, error) {
	keyPath := utils.ExpandHome("~/.ssh/id_munin_deploy")
	sshDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", "", err
	}

	// Remove old key if exists
	_ = os.Remove(keyPath)
	_ = os.Remove(keyPath + ".pub")

	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", "munin-deploy")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("%s: %w", string(out), err)
	}

	pubBytes, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return keyPath, "", nil
	}

	return keyPath, string(pubBytes), nil
}

func writeSampleScreenYAML(dest string) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(dest, defaultScreenTemplate, 0644)
}

func installSystemdUserService() error {
	userServiceDir := utils.ExpandHome("~/.config/systemd/user")
	if err := os.MkdirAll(userServiceDir, 0755); err != nil {
		return err
	}

	servicePath := filepath.Join(userServiceDir, "munin.service")
	if err := os.WriteFile(servicePath, defaultServiceTemplate, 0644); err != nil {
		return err
	}

	// Try enabling lingering
	currentUser := os.Getenv("USER")
	if currentUser != "" {
		_ = exec.Command("loginctl", "enable-linger", currentUser).Run()
	}

	// Reload and enable user service
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	_ = exec.Command("systemctl", "--user", "enable", "--now", "munin.service").Run()

	return nil
}

func createDefaultConfig(dest string) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(dest, defaultAgentTemplate, 0644); err != nil {
		return err
	}

	screenDest := filepath.Join(dir, "screen.yaml")
	if _, err := os.Stat(screenDest); os.IsNotExist(err) {
		return writeSampleScreenYAML(screenDest)
	}
	return nil
}

func printBanner() {
	fmt.Print(`
  __  __             _       
 |  \/  |_   _ _ __ (_)_ __  
 | |\/| | | | | '_ \| | '_ \ 
 | |  | | |_| | | | | | | | |
 |_|  |_|\__,_|_| |_|_|_| |_|
`)
	fmt.Println(" Munin Screen Agent Setup Wizard")
	fmt.Println(" ===============================")
}

func printSummary(configFile string, isGit bool) {
	fmt.Println()
	fmt.Println("=== Setup Complete! ===")
	fmt.Printf("Configuration saved at: %s\n", configFile)
	if isGit {
		fmt.Println("Git fleet management is enabled.")
		fmt.Println("Munin will synchronize dashboards and schedules from your repository.")
	} else {
		fmt.Println("Local mode is enabled. Edit ~/.munin/screen.yaml to customize your display.")
	}

	fmt.Println("\nHelpful commands:")
	fmt.Println("  systemctl --user status munin     # Check service status")
	fmt.Println("  journalctl --user -u munin -f     # View live logs")
	fmt.Println("  systemctl --user restart munin    # Restart service")
	fmt.Println("  munin --help                      # Show CLI options")
	fmt.Println()
}
