package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/naueramant/munin/internal/config"
	"github.com/naueramant/munin/internal/utils"
)

// CurrentVersion is the active binary version (can be set via ldflags: -X github.com/naueramant/munin/internal/updater.CurrentVersion=v1.2.3)
var CurrentVersion = "dev"

type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Name    string        `json:"name"`
	Assets  []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Updater handles checking and applying binary updates from GitHub Releases.
type Updater struct {
	cfg        config.UpdateConfig
	httpClient *http.Client
}

// NewUpdater creates a new Updater instance.
func NewUpdater(cfg config.UpdateConfig) *Updater {
	return &Updater{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Start runs the periodic update check in a background goroutine until context is cancelled.
func (u *Updater) Start(ctx context.Context, onUpdated func(newVersion string)) {
	if !u.cfg.IsEnabled() {
		slog.Debug("Auto-updater is disabled in configuration")
		return
	}

	cronExpr := u.cfg.GetSchedule()
	slog.Info("Auto-updater scheduled", "repo", u.cfg.GetRepo(), "cron", cronExpr)

	for {
		delay := ComputeNextDelay(cronExpr, 24*time.Hour, time.Now())
		slog.Debug("Next update check scheduled", "delay", delay.Round(time.Second))

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			u.checkAndUpdate(onUpdated)
		}
	}
}

func (u *Updater) checkAndUpdate(onUpdated func(newVersion string)) {
	updated, newVer, err := u.CheckAndApply()
	if err != nil {
		slog.Warn("Auto-update check encountered error", "error", err)
		return
	}
	if updated {
		slog.Info("Munin agent successfully updated", "version", newVer)
		if onUpdated != nil {
			onUpdated(newVer)
		}
	}
}

// CheckAndApply checks for a new release and applies it if available.
func (u *Updater) CheckAndApply() (bool, string, error) {
	rel, err := u.FetchLatestRelease()
	if err != nil {
		return false, "", err
	}

	if !isNewerVersion(CurrentVersion, rel.TagName) {
		slog.Debug("Release check completed; agent is up to date", "current_version", CurrentVersion, "latest_release", rel.TagName)
		return false, rel.TagName, nil
	}

	slog.Info("New agent version available, downloading update...", "current", CurrentVersion, "latest", rel.TagName)

	asset := findMatchingAsset(rel.Assets, runtime.GOOS, runtime.GOARCH)
	if asset == nil {
		return false, "", fmt.Errorf("no matching release asset found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, rel.TagName)
	}

	slog.Debug("Downloading release asset", "name", asset.Name, "url", asset.BrowserDownloadURL)
	binaryData, err := u.downloadAndExtract(asset.BrowserDownloadURL, asset.Name)
	if err != nil {
		return false, "", fmt.Errorf("failed to download release asset: %w", err)
	}

	if err := applyBinaryUpdate(binaryData); err != nil {
		return false, "", fmt.Errorf("failed to apply binary update: %w", err)
	}

	return true, rel.TagName, nil
}

// FetchLatestRelease queries the GitHub API for the latest release.
func (u *Updater) FetchLatestRelease() (*GitHubRelease, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.cfg.GetRepo())

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "munin-screen-agent/"+CurrentVersion)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("github api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to decode release json: %w", err)
	}

	return &rel, nil
}

// ComputeNextDelay calculates how long to sleep until the next scheduled update check.
func ComputeNextDelay(when string, fallbackInterval time.Duration, from time.Time) time.Duration {
	if strings.TrimSpace(when) == "" && fallbackInterval > 0 {
		return fallbackInterval
	}
	return utils.ComputeNextCronDelay(when, "0 4 * * *", from)
}

func findMatchingAsset(assets []GitHubAsset, targetOS, targetArch string) *GitHubAsset {
	osPattern := strings.ToLower(targetOS)

	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if !strings.Contains(name, osPattern) {
			continue
		}

		if matchArch(name, targetArch) {
			return &a
		}
	}

	return nil
}

func matchArch(filename, targetArch string) bool {
	switch targetArch {
	case "amd64":
		return strings.Contains(filename, "amd64") || strings.Contains(filename, "x86_64")
	case "arm64":
		return strings.Contains(filename, "arm64") || strings.Contains(filename, "aarch64")
	case "arm":
		return strings.Contains(filename, "armv7") || strings.Contains(filename, "armv6") || strings.Contains(filename, "arm")
	default:
		return strings.Contains(filename, targetArch)
	}
}

func (u *Updater) downloadAndExtract(url, assetName string) ([]byte, error) {
	resp, err := u.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	if strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz") {
		return extractBinaryFromTarGz(resp.Body)
	}

	// Plain binary
	return io.ReadAll(resp.Body)
}

func extractBinaryFromTarGz(r io.Reader) ([]byte, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		baseName := filepath.Base(header.Name)
		if baseName == "munin" || baseName == "munin.exe" || baseName == "mir" || baseName == "mir.exe" {
			return io.ReadAll(tr)
		}
	}

	return nil, fmt.Errorf("binary 'munin' not found in tar archive")
}

func applyBinaryUpdate(binaryData []byte) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink for executable: %w", err)
	}

	dir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(dir, "munin-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary update file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(binaryData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write update binary: %w", err)
	}

	if err := tmpFile.Chmod(0755); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to set execute permissions on update: %w", err)
	}
	tmpFile.Close()

	// Atomically replace running binary
	if err := os.Rename(tmpName, execPath); err != nil {
		return fmt.Errorf("failed to replace binary %s: %w", execPath, err)
	}

	return nil
}

func isNewerVersion(current, latest string) bool {
	if current == "dev" {
		return false // Do not auto-update development builds
	}
	c := strings.TrimPrefix(strings.TrimSpace(current), "v")
	l := strings.TrimPrefix(strings.TrimSpace(latest), "v")
	return c != l && l != ""
}
