package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestScreenValid(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"shared/config/files.yaml": "- src: \"../shared/files/*\"\n  dest: \"/home/pi/\"\n",
		"shared/files/lunch.html":  "lunch",
		"lobby/custom.css":         "body {}",
		"lobby/screen.yaml": `
syntax: v1
tabs:
  - url: "https://example.com"
    css: "custom.css"
    js: "/home/pi/ai.js"
files: !include ../shared/config/files.yaml
jobs:
  - when: "@daily"
    command: "true"
`,
	})

	r := Screen(filepath.Join(dir, "lobby/screen.yaml"), "")
	if !r.OK() {
		t.Errorf("expected no problems, got errors %v warnings %v", r.Errors, r.Warnings)
	}
}

func TestScreenProblems(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screen.yaml": `
syntax: v1
tabs:
  - url: "https://example.com"
    css: "missing.css"
schedules:
  - when: "12:00"
    duration: 60
    url: "https://example.com/lunch"
    js: "missing.js"
files:
  - src: "pages/*.html"
    dest: "/home/pi/"
  - src: "nope.sh"
    dest: "/home/pi/nope.sh"
    mode: "0999"
jobs:
  - when: "0 3 * * *"
`,
	})

	r := Screen(filepath.Join(dir, "screen.yaml"), "")

	want := []string{
		`files[0]: pattern "pages/*.html" matched no files`,
		`files[1]: source "nope.sh" not found`,
		`files[1].mode: "0999" is not an octal file mode`,
		`tabs[0].css: file "missing.css" not found`,
		`schedules[0].js: file "missing.js" not found`,
		`jobs[0]: no command`,
	}
	if got := strings.Join(r.Errors, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("unexpected errors:\n got:\n%s\n want:\n%s", got, strings.Join(want, "\n"))
	}
}

func TestScreenLoadErrorAndWarnings(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"screen.yaml": "syntax: v1\ntabs:\n  - url: \"${MUNIN_LINT_TEST_UNSET}\"\npower:\n  screen_on: \"0 7 * * 1-55\"\n",
	})

	r := Screen(filepath.Join(dir, "screen.yaml"), "")
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "${MUNIN_LINT_TEST_UNSET}") {
		t.Errorf("expected unset variable warning, got %v", r.Warnings)
	}
	if len(r.Errors) != 1 {
		t.Fatalf("expected a single load error, got %v", r.Errors)
	}
}

func TestScreenRespectsRoot(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"repo/screen.yaml": "syntax: v1\npower: !include ../outside.yaml\n",
		"outside.yaml":     "screen_on: \"07:00\"\n",
	})

	r := Screen(filepath.Join(dir, "repo/screen.yaml"), filepath.Join(dir, "repo"))
	if len(r.Errors) != 1 || !strings.Contains(r.Errors[0], "outside") {
		t.Errorf("expected outside-root error, got %v", r.Errors)
	}
}
