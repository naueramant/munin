package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
	"time"
)

func TestMatchArch(t *testing.T) {
	tests := []struct {
		filename   string
		targetArch string
		expected   bool
	}{
		{"mir_linux_amd64.tar.gz", "amd64", true},
		{"mir_Linux_x86_64.tar.gz", "amd64", true},
		{"mir_linux_arm64.tar.gz", "arm64", true},
		{"mir_linux_aarch64.tar.gz", "arm64", true},
		{"mir_linux_armv7.tar.gz", "arm", true},
		{"mir_linux_arm64.tar.gz", "amd64", false},
	}

	for _, tt := range tests {
		got := matchArch(tt.filename, tt.targetArch)
		if got != tt.expected {
			t.Errorf("matchArch(%s, %s) = %v, expected %v", tt.filename, tt.targetArch, got, tt.expected)
		}
	}
}

func TestFindMatchingAsset(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "mir_Darwin_all.tar.gz", BrowserDownloadURL: "http://example.com/darwin"},
		{Name: "mir_Linux_x86_64.tar.gz", BrowserDownloadURL: "http://example.com/linux_amd64"},
		{Name: "mir_Linux_arm64.tar.gz", BrowserDownloadURL: "http://example.com/linux_arm64"},
	}

	asset := findMatchingAsset(assets, "linux", "arm64")
	if asset == nil || asset.Name != "mir_Linux_arm64.tar.gz" {
		t.Fatalf("expected arm64 asset, got: %+v", asset)
	}

	assetAMD := findMatchingAsset(assets, "linux", "amd64")
	if assetAMD == nil || assetAMD.Name != "mir_Linux_x86_64.tar.gz" {
		t.Fatalf("expected amd64 asset, got: %+v", assetAMD)
	}
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("fake-munin-binary")
	header := &tar.Header{
		Name: "munin",
		Mode: 0755,
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	extracted, err := extractBinaryFromTarGz(&buf)
	if err != nil {
		t.Fatalf("extractBinaryFromTarGz failed: %v", err)
	}

	if string(extracted) != string(content) {
		t.Errorf("expected '%s', got '%s'", string(content), string(extracted))
	}
}

func TestIsNewerVersion(t *testing.T) {
	if isNewerVersion("dev", "v1.0.0") {
		t.Errorf("expected dev build to not auto-update")
	}
	if !isNewerVersion("1.0.0", "v1.0.1") {
		t.Errorf("expected 1.0.1 to be newer than 1.0.0")
	}
	if isNewerVersion("1.0.0", "v1.0.0") {
		t.Errorf("expected identical versions to not trigger update")
	}
}

func TestComputeNextDelay(t *testing.T) {
	// Fallback interval test
	delay := ComputeNextDelay("", 3*time.Hour, time.Now())
	if delay != 3*time.Hour {
		t.Errorf("expected 3h delay, got %v", delay)
	}

	// Time of day test: e.g. 04:00 from 02:00 -> 2 hours
	from := time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)
	delayTOD := ComputeNextDelay("04:00", 24*time.Hour, from)
	if delayTOD != 2*time.Hour {
		t.Errorf("expected 2h delay from 02:00 to 04:00, got %v", delayTOD)
	}

	// Time of day test: e.g. 04:00 from 05:00 -> 23 hours
	fromPast := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)
	delayPast := ComputeNextDelay("04:00", 24*time.Hour, fromPast)
	if delayPast != 23*time.Hour {
		t.Errorf("expected 23h delay from 05:00 to 04:00 next day, got %v", delayPast)
	}

	// Cron expression test: 0 4 * * *
	delayCron := ComputeNextDelay("0 4 * * *", 24*time.Hour, from)
	if delayCron != 2*time.Hour {
		t.Errorf("expected 2h delay for cron 0 4 * * *, got %v", delayCron)
	}
}
