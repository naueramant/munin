package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadFileToString reads a file and returns its string content.
func ReadFileToString(path string) (string, error) {
	byt, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(byt), nil
}

// ExpandHome expands a leading tilde (~) in a path to the current user's home directory.
func ExpandHome(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
