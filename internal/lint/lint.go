package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/naueramant/munin/internal/config"
	"github.com/naueramant/munin/internal/filesync"
)

// Result holds the problems found in one screen configuration.
type Result struct {
	Path     string
	Errors   []string
	Warnings []string
}

// OK reports whether the screen has no errors and no warnings.
func (r Result) OK() bool {
	return len(r.Errors) == 0 && len(r.Warnings) == 0
}

// Screen checks a screen configuration for problems that would only show up on
// the device: on top of the regular load validation it checks that files[]
// sources exist and match, that relative css/js files exist, and that jobs
// have a command. Includes must stay inside root when it is set.
func Screen(path, root string) Result {
	r := Result{Path: path}

	res, err := config.Resolve(path, config.ResolveOptions{Root: root})
	for _, name := range res.UnsetVars {
		r.Warnings = append(r.Warnings, fmt.Sprintf("${%s} is not set and has no default", name))
	}
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return r
	}

	cfg, err := config.Decode(res)
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
		return r
	}

	baseDir := filepath.Dir(path)
	if abs, err := filepath.Abs(path); err == nil {
		baseDir = filepath.Dir(abs)
	}

	for i, m := range cfg.Files {
		if _, err := filesync.Expand(baseDir, m); err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("files[%d]: %v", i, err))
		}
		if m.Mode != "" {
			if _, err := strconv.ParseUint(m.Mode, 8, 32); err != nil {
				r.Errors = append(r.Errors, fmt.Sprintf("files[%d].mode: %q is not an octal file mode", i, m.Mode))
			}
		}
	}

	for i, t := range cfg.Tabs {
		r.checkExtra(baseDir, fmt.Sprintf("tabs[%d].css", i), t.CSS)
		r.checkExtra(baseDir, fmt.Sprintf("tabs[%d].js", i), t.JS)
	}
	for i, s := range cfg.Schedules {
		r.checkExtra(baseDir, fmt.Sprintf("schedules[%d].css", i), s.CSS)
		r.checkExtra(baseDir, fmt.Sprintf("schedules[%d].js", i), s.JS)
	}

	for i, j := range cfg.Jobs {
		if j.GetCommandLine() == "" {
			r.Errors = append(r.Errors, fmt.Sprintf("jobs[%d]: no command", i))
		}
	}

	return r
}

// checkExtra reports a relative css/js path that does not exist. Absolute paths
// refer to the device's filesystem and cannot be checked here.
func (r *Result) checkExtra(baseDir, key, path string) {
	if path == "" || filepath.IsAbs(path) {
		return
	}
	if _, err := os.Stat(filepath.Join(baseDir, path)); err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("%s: file %q not found", key, path))
	}
}
