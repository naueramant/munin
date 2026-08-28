package wizard

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrompt(t *testing.T) {
	// User enters a custom value
	reader := bufio.NewReader(strings.NewReader("custom-value\n"))
	ans := prompt(reader, "Enter text", "default")
	if ans != "custom-value" {
		t.Errorf("expected 'custom-value', got '%s'", ans)
	}

	// User enters newline (takes default)
	readerDef := bufio.NewReader(strings.NewReader("\n"))
	ansDef := prompt(readerDef, "Enter text", "my-default")
	if ansDef != "my-default" {
		t.Errorf("expected 'my-default', got '%s'", ansDef)
	}
}

func TestWriteSampleScreenYAML(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sub", "screen.yaml")
	if err := writeSampleScreenYAML(tmpFile); err != nil {
		t.Fatalf("failed to write sample screen yaml: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "syntax: v1") {
		t.Errorf("sample screen yaml missing syntax header")
	}
	if !strings.Contains(string(data), "power:") {
		t.Errorf("sample screen yaml missing power block")
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sub", "config.yaml")
	if err := createDefaultConfig(tmpFile); err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), `mode: "local"`) {
		t.Errorf("expected default config to be local mode")
	}
}
