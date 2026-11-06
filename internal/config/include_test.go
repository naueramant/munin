package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFiles creates files (relative path -> content) under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIncludeValue(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screens/lobby/screen.yaml": `
syntax: v1
power: !include ../../shared/power.yaml
`,
		"shared/power.yaml": `
screen_on: "07:00"
screen_off: "19:00"
`,
	})

	cfg, err := Load(filepath.Join(dir, "screens/lobby/screen.yaml"))
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.Power.GetScreenOn() != "07:00" || cfg.Power.GetScreenOff() != "19:00" {
		t.Errorf("unexpected power config: %+v", cfg.Power)
	}
}

func TestIncludeListSplice(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screen.yaml": `
syntax: v1
schedules:
  - !include shared/lunch.yaml
  - when: "09:00"
    duration: 600
    url: "https://intranet/standup"
  - !include shared/single.yaml
  - !include shared/empty.yaml
`,
		"shared/lunch.yaml": `
- when: "12:00"
  duration: 900
  message: "Lunch"
- when: "14:00"
  duration: 300
  message: "Fika"
`,
		"shared/single.yaml": `
when: "16:00"
duration: 60
message: "Go home"
`,
		"shared/empty.yaml": "",
	})

	cfg, err := Load(filepath.Join(dir, "screen.yaml"))
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	var whens []string
	for _, s := range cfg.Schedules {
		whens = append(whens, s.When)
	}
	if got := strings.Join(whens, ","); got != "12:00,14:00,09:00,16:00" {
		t.Errorf("unexpected schedule order: %s", got)
	}
}

func TestIncludeMergeKey(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screen.yaml": `
syntax: v1
power:
  <<: !include shared/power.yaml
  screen_off: "20:00"
`,
		"shared/power.yaml": `
screen_on: "07:00"
screen_off: "19:00"
cec_device: 1
`,
	})

	cfg, err := Load(filepath.Join(dir, "screen.yaml"))
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.Power.GetScreenOn() != "07:00" {
		t.Errorf("expected merged screen_on '07:00', got %q", cfg.Power.GetScreenOn())
	}
	if cfg.Power.GetScreenOff() != "20:00" {
		t.Errorf("expected local screen_off override '20:00', got %q", cfg.Power.GetScreenOff())
	}
	if cfg.Power.GetCecDevice() != 1 {
		t.Errorf("expected merged cec_device 1, got %d", cfg.Power.GetCecDevice())
	}
}

func TestIncludeNestedRelativeToIncludingFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screen.yaml": `
syntax: v1
schedules: !include shared/schedules.yaml
`,
		"shared/schedules.yaml": `
- !include parts/lunch.yaml
`,
		"shared/parts/lunch.yaml": `
when: "12:00"
duration: 900
message: "Lunch"
`,
	})

	cfg, err := Load(filepath.Join(dir, "screen.yaml"))
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if len(cfg.Schedules) != 1 || cfg.Schedules[0].Message != "Lunch" {
		t.Errorf("unexpected schedules: %+v", cfg.Schedules)
	}
}

func TestIncludeDirMergeList(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screen.yaml": `
syntax: v1
schedules:
  - !include_dir_merge_list schedules/
  - when: "09:00"
    duration: 600
    message: "Standup"
`,
		"schedules/20-fika.yaml": `
- when: "14:00"
  duration: 300
  message: "Fika"
- when: "15:00"
  duration: 300
  message: "Fika 2"
`,
		"schedules/10-lunch.yml": `
when: "12:00"
duration: 900
message: "Lunch"
`,
		"schedules/.hidden.yaml": `
when: "01:00"
duration: 1
message: "hidden"
`,
		"schedules/notes.txt":       "not yaml",
		"schedules/sub/nested.yaml": "when: \"02:00\"\nduration: 1\nmessage: nested\n",
		"schedules/30-empty.yaml":   "",
	})

	res, err := Resolve(filepath.Join(dir, "screen.yaml"), ResolveOptions{})
	if err != nil {
		t.Fatalf("failed to resolve config: %v", err)
	}
	cfg, err := Decode(res)
	if err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}

	var whens []string
	for _, s := range cfg.Schedules {
		whens = append(whens, s.When)
	}
	if got := strings.Join(whens, ","); got != "12:00,14:00,15:00,09:00" {
		t.Errorf("unexpected schedule order: %s", got)
	}

	schedulesDir := filepath.Join(dir, "schedules")
	found := false
	for _, s := range res.Sources {
		if s == schedulesDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected sources to contain %s, got %v", schedulesDir, res.Sources)
	}
}

func TestIncludeEnvSubstitutionInIncludedFile(t *testing.T) {
	t.Setenv("MUNIN_TEST_ON", "06:30")

	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screen.yaml": `
syntax: v1
power: !include ${MUNIN_TEST_POWER_FILE:=power.yaml}
`,
		"power.yaml": `
screen_on: "${MUNIN_TEST_ON}"
screen_off: "${MUNIN_TEST_OFF_UNSET}"
`,
	})

	res, err := Resolve(filepath.Join(dir, "screen.yaml"), ResolveOptions{})
	if err != nil {
		t.Fatalf("failed to resolve config: %v", err)
	}
	cfg, err := Decode(res)
	if err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}
	if cfg.Power.GetScreenOn() != "06:30" {
		t.Errorf("expected screen_on '06:30', got %q", cfg.Power.GetScreenOn())
	}
	if len(res.UnsetVars) != 1 || res.UnsetVars[0] != "MUNIN_TEST_OFF_UNSET" {
		t.Errorf("unexpected unset vars: %v", res.UnsetVars)
	}
}

func TestIncludeErrors(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		opts    func(dir string) ResolveOptions
		wantErr string
	}{
		{
			name: "missing file",
			files: map[string]string{
				"screen.yaml": "syntax: v1\npower: !include missing.yaml\n",
			},
			wantErr: `screen.yaml:2: cannot include "missing.yaml"`,
		},
		{
			name: "cycle",
			files: map[string]string{
				"screen.yaml": "syntax: v1\npower: !include a.yaml\n",
				"a.yaml":      "<<: !include b.yaml\n",
				"b.yaml":      "<<: !include a.yaml\n",
			},
			wantErr: "include cycle: screen.yaml -> a.yaml -> b.yaml -> a.yaml",
		},
		{
			name: "include on a mapping",
			files: map[string]string{
				"screen.yaml": "syntax: v1\npower: !include {a: b}\n",
			},
			wantErr: "!include must be followed by a path",
		},
		{
			name: "unknown include tag",
			files: map[string]string{
				"screen.yaml": "syntax: v1\nschedules: !include_dir_named schedules/\n",
			},
			wantErr: "unknown tag !include_dir_named",
		},
		{
			name: "file include of a directory",
			files: map[string]string{
				"screen.yaml":        "syntax: v1\nschedules: !include schedules\n",
				"schedules/one.yaml": "[]\n",
			},
			wantErr: "is a directory",
		},
		{
			name: "outside root",
			files: map[string]string{
				"repo/screen.yaml": "syntax: v1\npower: !include ../outside.yaml\n",
				"outside.yaml":     "screen_on: \"07:00\"\n",
			},
			opts:    func(dir string) ResolveOptions { return ResolveOptions{Root: filepath.Join(dir, "repo")} },
			wantErr: "path is outside",
		},
		{
			name: "parse error in included file",
			files: map[string]string{
				"screen.yaml": "syntax: v1\npower: !include bad.yaml\n",
				"bad.yaml":    "screen_on: [\n",
			},
			wantErr: "failed to parse bad.yaml",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tc.files)

			screen := filepath.Join(dir, "screen.yaml")
			if _, ok := tc.files["repo/screen.yaml"]; ok {
				screen = filepath.Join(dir, "repo/screen.yaml")
			}
			opts := ResolveOptions{}
			if tc.opts != nil {
				opts = tc.opts(dir)
			}

			_, err := Resolve(screen, opts)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestIncludeInsideRootAllowed(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screens/lobby/screen.yaml": "syntax: v1\npower: !include ../../shared/power.yaml\n",
		"shared/power.yaml":         "screen_on: \"07:00\"\n",
	})

	_, err := Resolve(filepath.Join(dir, "screens/lobby/screen.yaml"), ResolveOptions{Root: dir})
	if err != nil {
		t.Fatalf("expected include inside root to succeed, got %v", err)
	}
}

func TestResolveAnnotateAndMask(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screen.yaml": `
syntax: v1
tabs:
  - !include tabs.yaml
power: !include power.yaml
`,
		"tabs.yaml": `
- url: "https://example.com"
  duration: 90
  auth:
    username: kiosk
    password: hunter2
`,
		"power.yaml": `
screen_on: "07:00"
`,
	})

	res, err := Resolve(filepath.Join(dir, "screen.yaml"), ResolveOptions{Annotate: true})
	if err != nil {
		t.Fatalf("failed to resolve config: %v", err)
	}
	MaskSecrets(res.Node)

	var buf bytes.Buffer
	if err := EncodeYAML(&buf, res.Node); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"# from tabs.yaml:2",
		"power: # from power.yaml",
		"password: '***'",
		"#   power.yaml",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "hunter2") {
		t.Errorf("expected password to be masked, got:\n%s", out)
	}
}

func TestEffectiveConfigEncodesDurations(t *testing.T) {
	cfg := Configuration{Syntax: "v1", Tabs: []Tab{{Content: Content{URL: "https://example.com"}, Duration: Duration(90 * time.Second)}}}

	var buf bytes.Buffer
	if err := EncodeYAML(&buf, cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "duration: 1m30s") {
		t.Errorf("expected duration rendered as 1m30s, got:\n%s", buf.String())
	}

	var round Configuration
	res, err := resolveBytes(t, buf.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := res.Node.Decode(&round); err != nil {
		t.Fatal(err)
	}
	if round.Tabs[0].Duration.Duration() != 90*time.Second {
		t.Errorf("expected round-trip duration 90s, got %v", round.Tabs[0].Duration.Duration())
	}
}

func resolveBytes(t *testing.T, content string) (*Resolved, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "screen.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return Resolve(path, ResolveOptions{})
}

func TestFleetExampleScreensLoad(t *testing.T) {
	repo := filepath.Join("..", "..", "examples", "2-git-fleet-management", "sample-repo")

	for _, screen := range []string{"office-lobby", "ops-dashboard"} {
		t.Run(screen, func(t *testing.T) {
			res, err := Resolve(filepath.Join(repo, "screens", screen, "screen.yaml"), ResolveOptions{Root: repo})
			if err != nil {
				t.Fatalf("failed to resolve example: %v", err)
			}
			cfg, err := Decode(res)
			if err != nil {
				t.Fatalf("failed to decode example: %v", err)
			}
			if len(cfg.Schedules) == 0 || !cfg.Power.HasEntries() {
				t.Errorf("expected shared schedules and power to be included, got %+v", cfg)
			}
		})
	}
}

func TestDecodeValidatesPowerAndJobTimes(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"valid", "syntax: v1\npower:\n  screen_on: \"07:00\"\n  screen_off: \"0 17 * * 1-5\"\n  reboot: \"@daily\"\njobs:\n  - when: \"@reboot\"\n    command: \"true\"\n", ""},
		{"bad power cron", "syntax: v1\npower:\n  screen_on: \"0 7 * * 1-55\"\n", `power.screen_on "0 7 * * 1-55"`},
		{"legacy alias checked", "syntax: v1\npower:\n  turn_off: \"25:00\"\n", `power.screen_off "25:00"`},
		{"duration not allowed for power", "syntax: v1\npower:\n  reboot: \"5m\"\n", `power.reboot "5m"`},
		{"six fields", "syntax: v1\njobs:\n  - when: \"0 0 3 * * *\"\n    command: \"true\"\n", `jobs[0].when "0 0 3 * * *"`},
		{"unknown shortcut", "syntax: v1\njobs:\n  - when: \"@every 5m\"\n    command: \"true\"\n", `jobs[0].when "@every 5m"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := resolveBytes(t, tc.content)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Decode(res)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid config, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
