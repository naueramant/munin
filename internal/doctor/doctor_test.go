package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/naueramant/munin/internal/config"
)

func TestDoctorRun(t *testing.T) {
	doc := New(Options{})
	report := doc.Run()

	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if len(report.Results) == 0 {
		t.Fatal("expected at least one diagnostic result")
	}
	if report.Summary.Total != len(report.Results) {
		t.Errorf("expected summary.Total=%d, got %d", len(report.Results), report.Summary.Total)
	}
}

func TestDoctorRenderText(t *testing.T) {
	doc := New(Options{})
	report := &Report{
		Results: []CheckResult{
			{
				Category: CategoryDependencies,
				Name:     "Chromium Browser",
				Status:   StatusOK,
				Message:  "Found /usr/bin/chromium",
			},
			{
				Category: CategorySystemd,
				Name:     "User Lingering",
				Status:   StatusWarn,
				Message:  "User lingering disabled",
				Detail:   "Linger is off",
				FixHint:  "sudo loginctl enable-linger pi",
			},
		},
		Summary: Summary{
			Total:   2,
			Passed:  1,
			Warning: 1,
		},
	}

	var buf bytes.Buffer
	err := doc.Render(&buf, report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Munin System Diagnostics") {
		t.Errorf("output missing header: %s", output)
	}
	if !strings.Contains(output, "[Dependencies]") {
		t.Errorf("output missing [Dependencies] section: %s", output)
	}
	if !strings.Contains(output, "Chromium Browser") {
		t.Errorf("output missing Chromium check: %s", output)
	}
	if !strings.Contains(output, "sudo loginctl enable-linger pi") {
		t.Errorf("output missing fix recommendation: %s", output)
	}
	if !strings.Contains(output, "1 warning(s)") {
		t.Errorf("output missing warning summary: %s", output)
	}
}

func TestDoctorRenderJSON(t *testing.T) {
	doc := New(Options{JSON: true})
	report := &Report{
		Results: []CheckResult{
			{
				Category: CategoryDependencies,
				Name:     "Chromium Browser",
				Status:   StatusOK,
				Message:  "Found /usr/bin/chromium",
			},
		},
		Summary: Summary{
			Total:  1,
			Passed: 1,
		},
	}

	var buf bytes.Buffer
	err := doc.Render(&buf, report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}
	if len(decoded.Results) != 1 || decoded.Results[0].Name != "Chromium Browser" {
		t.Errorf("unexpected decoded content: %+v", decoded)
	}
}

func TestStatusSymbols(t *testing.T) {
	tests := []struct {
		s    Status
		want string
	}{
		{StatusOK, "✓"},
		{StatusWarn, "!"},
		{StatusError, "✗"},
		{StatusInfo, "ℹ"},
	}

	for _, tt := range tests {
		if got := tt.s.Symbol(); got != tt.want {
			t.Errorf("Status.Symbol() for %s = %s, want %s", tt.s, got, tt.want)
		}
	}
}

func TestCheckAgentConfig_ValidAndMissing(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Missing file with explicit path
	missingPath := filepath.Join(tmpDir, "missing.yaml")
	_, results := checkAgentConfig(missingPath)
	if len(results) == 0 || results[0].Status != StatusError {
		t.Errorf("expected StatusError for missing explicit config, got %+v", results)
	}

	// 2. Valid agent config
	validPath := filepath.Join(tmpDir, "agent.yaml")
	validYAML := `mode: "local"
log_level: "debug"
screen_path: "screen.yaml"
`
	if err := os.WriteFile(validPath, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, results2 := checkAgentConfig(validPath)
	if cfg == nil {
		t.Fatal("expected non-nil AgentConfig")
	}
	if len(results2) == 0 || results2[0].Status != StatusOK {
		t.Errorf("expected StatusOK for valid config, got %+v", results2)
	}
}

func TestCheckScreenConfig_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	screenPath := filepath.Join(tmpDir, "screen.yaml")
	sampleScreen := `syntax: v1
tabs:
  - url: "https://example.com"
    duration: 10
power:
  turn_on: "0 8 * * *"
`
	if err := os.WriteFile(screenPath, []byte(sampleScreen), 0644); err != nil {
		t.Fatal(err)
	}

	_, cfg, results := checkScreenConfig(Options{ScreenPath: screenPath}, nil)
	if cfg == nil {
		t.Fatal("expected non-nil screen config")
	}
	if len(results) == 0 || results[0].Status != StatusOK {
		t.Errorf("expected StatusOK for valid screen.yaml, got %+v", results)
	}
}

func TestDeployKeyPermissionsFix(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test")
	if err := os.WriteFile(keyPath, []byte("fake private key"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.AgentConfig{
		Mode: "git",
		Git: config.GitConfig{
			Repo:      "git@github.com:example/repo.git",
			DeployKey: keyPath,
		},
	}

	// 1. Without fix: should report StatusWarn
	resWithoutFix := checkGitFleet(Options{Fix: false}, cfg)
	var permResult *CheckResult
	for i := range resWithoutFix {
		if resWithoutFix[i].Name == "SSH Deploy Key Permissions" {
			permResult = &resWithoutFix[i]
			break
		}
	}
	if permResult == nil || permResult.Status != StatusWarn {
		t.Fatalf("expected StatusWarn for open permissions, got %+v", resWithoutFix)
	}

	// 2. With fix: should fix to 0600
	resWithFix := checkGitFleet(Options{Fix: true}, cfg)
	var fixedResult *CheckResult
	for i := range resWithFix {
		if resWithFix[i].Name == "SSH Deploy Key Permissions" {
			fixedResult = &resWithFix[i]
			break
		}
	}
	if fixedResult == nil || !fixedResult.FixApplied || fixedResult.Status != StatusOK {
		t.Fatalf("expected FixApplied and StatusOK, got %+v", fixedResult)
	}

	fi, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %#o", fi.Mode().Perm())
	}
}

func TestGetChromiumCandidates(t *testing.T) {
	candidates := getChromiumCandidates()
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate browser")
	}

	foundChromeStable := false
	for _, c := range candidates {
		if c == "google-chrome-stable" {
			foundChromeStable = true
			break
		}
	}
	if !foundChromeStable {
		t.Errorf("expected google-chrome-stable to be in candidate list, got %v", candidates)
	}

	// Test CHROME_BIN override
	t.Setenv("CHROME_BIN", "/custom/bin/my-chrome")
	candidatesWithEnv := getChromiumCandidates()
	if len(candidatesWithEnv) == 0 || candidatesWithEnv[0] != "/custom/bin/my-chrome" {
		t.Errorf("expected /custom/bin/my-chrome as first candidate, got %v", candidatesWithEnv)
	}
}

func TestCheckChromium(t *testing.T) {
	result := checkChromium()
	if result.Category != CategoryDependencies {
		t.Errorf("expected CategoryDependencies, got %s", result.Category)
	}
	if result.Name != "Chromium Browser" {
		t.Errorf("expected 'Chromium Browser', got %s", result.Name)
	}
}

