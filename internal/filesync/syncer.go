package filesync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/naueramant/munin/internal/config"
	"github.com/naueramant/munin/internal/utils"
)

// SyncFiles copies configured files from baseDir to their destinations.
func SyncFiles(baseDir string, mappings []config.FileMapping) error {
	for _, mapping := range mappings {
		if err := syncSingleFile(baseDir, mapping); err != nil {
			slog.Error("Failed to sync file", "src", mapping.Src, "dest", mapping.Dest, "error", err)
			return err
		}
	}
	return nil
}

func syncSingleFile(baseDir string, mapping config.FileMapping) error {
	srcPath := filepath.Join(baseDir, mapping.Src)
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("source file %s not found: %w", srcPath, err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source %s is a directory, only files are supported", srcPath)
	}

	destPath := utils.ExpandHome(mapping.Dest)
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	// Parse desired mode if provided
	var targetMode os.FileMode = 0644
	if mapping.Mode != "" {
		parsedMode, err := strconv.ParseUint(mapping.Mode, 8, 32)
		if err != nil {
			slog.Warn("Invalid file mode in config, using fallback 0644", "mode", mapping.Mode, "dest", mapping.Dest)
		} else {
			targetMode = os.FileMode(parsedMode)
		}
	} else if srcInfo.Mode()&0111 != 0 {
		// Preserve executable bit if source has it
		targetMode = 0755
	}

	// Compare hashes to avoid redundant flash/SD card writes
	destInfo, err := os.Stat(destPath)
	if err == nil && !destInfo.IsDir() {
		same, err := filesHaveSameHash(srcPath, destPath)
		if err == nil && same {
			// Content is identical, ensure file mode is correct
			if destInfo.Mode().Perm() != targetMode.Perm() {
				if err := os.Chmod(destPath, targetMode); err != nil {
					return fmt.Errorf("failed to chmod %s: %w", destPath, err)
				}
				slog.Info("Updated permissions for file", "path", destPath, "mode", fmt.Sprintf("%o", targetMode))
			} else {
				slog.Debug("File content and mode already up to date, skipping write", "path", destPath)
			}
			return nil
		}
	}

	// Copy file contents atomically
	tmpFile := destPath + ".tmp"
	if err := copyFile(srcPath, tmpFile, targetMode); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to copy %s to %s: %w", srcPath, tmpFile, err)
	}

	if err := os.Rename(tmpFile, destPath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to replace %s with %s: %w", destPath, tmpFile, err)
	}

	slog.Info("Synchronized file to host", "src", mapping.Src, "dest", destPath, "mode", fmt.Sprintf("%o", targetMode))
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}

func filesHaveSameHash(path1, path2 string) (bool, error) {
	hash1, err := fileHash(path1)
	if err != nil {
		return false, err
	}

	hash2, err := fileHash(path2)
	if err != nil {
		return false, err
	}

	return hash1 == hash2, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
