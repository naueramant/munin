package filesync

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/naueramant/munin/internal/config"
	"github.com/naueramant/munin/internal/utils"
)

// SyncFiles copies configured files from baseDir to their destinations. A
// mapping's src may be a single file, a directory (copied recursively), or a
// glob pattern (with ** for any depth); for directories and globs, dest is a
// directory. A failing mapping does not stop the others; all errors are returned.
func SyncFiles(baseDir string, mappings []config.FileMapping) error {
	var errs []error
	for _, mapping := range mappings {
		copies, err := Expand(baseDir, mapping)
		if err != nil {
			slog.Error("Failed to sync file", "src", mapping.Src, "dest", mapping.Dest, "error", err)
			errs = append(errs, err)
			continue
		}
		for _, c := range copies {
			if err := syncSingleFile(c.Src, c.Dest, mapping.Mode); err != nil {
				slog.Error("Failed to sync file", "src", c.Src, "dest", c.Dest, "error", err)
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// Copy is a single file copy described by a file mapping.
type Copy struct {
	Src  string
	Dest string
}

// Expand turns a mapping into the individual file copies it describes, without
// copying anything. It fails if the source does not exist or a pattern matches
// no files.
func Expand(baseDir string, mapping config.FileMapping) ([]Copy, error) {
	src := mapping.Src
	if !filepath.IsAbs(src) {
		src = filepath.Join(baseDir, src)
	}
	dest := utils.ExpandHome(mapping.Dest)

	if hasGlobMeta(mapping.Src) {
		return expandGlob(src, dest, mapping.Src)
	}

	srcInfo, err := os.Stat(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("source %q not found", mapping.Src)
	}
	if err != nil {
		return nil, fmt.Errorf("source %q: %w", mapping.Src, err)
	}

	if !srcInfo.IsDir() {
		// A dest ending in a slash is a directory, like cp
		if strings.HasSuffix(mapping.Dest, "/") {
			dest = filepath.Join(dest, filepath.Base(src))
		}
		return []Copy{{Src: src, Dest: dest}}, nil
	}

	var copies []Copy
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != src && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		copies = append(copies, Copy{Src: path, Dest: filepath.Join(dest, rel)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory %s: %w", src, err)
	}
	if len(copies) == 0 {
		return nil, fmt.Errorf("source directory %q contains no files", mapping.Src)
	}
	return copies, nil
}

// expandGlob matches pattern against the filesystem. Matched files keep their
// path relative to the part of the pattern before the first wildcard. Hidden
// files and directories are skipped unless the pattern names them explicitly.
func expandGlob(pattern, dest, original string) ([]Copy, error) {
	base, rest := doublestar.SplitPattern(filepath.ToSlash(pattern))
	base = filepath.FromSlash(base)

	matches, err := doublestar.Glob(os.DirFS(base), rest, doublestar.WithFilesOnly())
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", original, err)
	}

	allowHidden := strings.HasPrefix(rest, ".") || strings.Contains(rest, "/.")

	var copies []Copy
	for _, m := range matches {
		if !allowHidden && hasHiddenComponent(m) {
			continue
		}
		copies = append(copies, Copy{
			Src:  filepath.Join(base, filepath.FromSlash(m)),
			Dest: filepath.Join(dest, filepath.FromSlash(m)),
		})
	}
	if len(copies) == 0 {
		return nil, fmt.Errorf("pattern %q matched no files", original)
	}
	return copies, nil
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[{")
}

func hasHiddenComponent(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func syncSingleFile(srcPath, destPath, mode string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("source file %s not found: %w", srcPath, err)
	}

	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	// Parse desired mode if provided
	var targetMode os.FileMode = 0644
	if mode != "" {
		parsedMode, err := strconv.ParseUint(mode, 8, 32)
		if err != nil {
			slog.Warn("Invalid file mode in config, using fallback 0644", "mode", mode, "dest", destPath)
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

	slog.Info("Synchronized file to host", "src", srcPath, "dest", destPath, "mode", fmt.Sprintf("%o", targetMode))
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
