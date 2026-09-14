package config

import (
	"os"
	"testing"
	"time"
)

func TestSubstituteEnvVars(t *testing.T) {
	t.Setenv("MUNIN_TEST_SET", "hello")
	t.Setenv("MUNIN_TEST_EMPTY", "")
	os.Unsetenv("MUNIN_TEST_UNSET")

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"set var", "${MUNIN_TEST_SET}", "hello"},
		{"unset var empty", "a${MUNIN_TEST_UNSET}b", "ab"},
		{"unset var with default", "${MUNIN_TEST_UNSET:=fallback}", "fallback"},
		{"empty var with default", "${MUNIN_TEST_EMPTY:=fallback}", "fallback"},
		{"set var ignores default", "${MUNIN_TEST_SET:=fallback}", "hello"},
		{"multiple refs", "${MUNIN_TEST_SET}-${MUNIN_TEST_UNSET:=x}", "hello-x"},
		{"no substitution", "plain text", "plain text"},
		{"empty default", "${MUNIN_TEST_UNSET:=}", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(substituteEnvVars([]byte(tc.input)))
			if got != tc.want {
				t.Errorf("substituteEnvVars(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestLoadScreenConfigWithEnvSubstitution(t *testing.T) {
	t.Setenv("MUNIN_TEST_URL", "https://dashboard.example.com")

	content := `
syntax: v1
tabs:
  - url: "${MUNIN_TEST_URL}"
    duration: ${MUNIN_TEST_DURATION:=45}
`
	tmpFile := t.TempDir() + "/screen.yaml"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Tabs) != 1 || cfg.Tabs[0].URL != "https://dashboard.example.com" {
		t.Errorf("unexpected tab url: %+v", cfg.Tabs)
	}
	if cfg.Tabs[0].Duration != Duration(45*time.Second) {
		t.Errorf("expected duration 45s, got %v", cfg.Tabs[0].Duration.Duration())
	}
}
