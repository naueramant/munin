package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	includeTag        = "!include"
	includeDirListTag = "!include_dir_merge_list"
)

// ResolveOptions controls how a screen configuration and its includes are resolved.
type ResolveOptions struct {
	// Root, when set, rejects includes that resolve outside this directory
	// (e.g. the git checkout in git mode).
	Root string
	// Annotate adds "from <file>" comments to included content.
	Annotate bool
}

// Resolved is a screen configuration after env substitution and include resolution.
type Resolved struct {
	Node      *yaml.Node // Document node; Kind is 0 for an empty file
	Sources   []string   // Absolute paths of every file and directory read
	UnsetVars []string   // ${var} references that were unset and had no default
}

type resolver struct {
	opts       ResolveOptions
	root       string
	baseDir    string
	stack      []string
	sources    []string
	seen       map[string]bool
	unset      []string
	unsetSeen  map[string]bool
	includeFor map[*yaml.Node]*yaml.Node
}

// Resolve reads a screen configuration, substitutes environment variables, and
// replaces !include and !include_dir_merge_list tags with the referenced content.
// The returned Resolved is never nil; its Sources are filled even on error so
// callers can watch the files involved.
func Resolve(filename string, opts ResolveOptions) (*Resolved, error) {
	abs, err := filepath.Abs(filename)
	if err != nil {
		abs = filename
	}

	r := &resolver{
		opts:       opts,
		baseDir:    filepath.Dir(abs),
		seen:       map[string]bool{},
		unsetSeen:  map[string]bool{},
		includeFor: map[*yaml.Node]*yaml.Node{},
	}
	if opts.Root != "" {
		r.root = realPath(opts.Root)
	}

	res := &Resolved{}
	r.addSource(abs)

	if _, err := os.Stat(abs); err != nil {
		res.Sources = r.sources
		return res, fmt.Errorf("failed to load configuration file: %w", err)
	}

	doc, err := r.loadFile(abs)
	res.Node = doc
	res.Sources = r.sources
	res.UnsetVars = r.unset
	if err != nil {
		return res, err
	}

	if opts.Annotate && doc.Kind != 0 && len(r.sources) > 1 {
		lines := []string{"# Sources:"}
		for _, s := range r.sources {
			lines = append(lines, "#   "+r.display(s))
		}
		doc.HeadComment = strings.Join(lines, "\n")
	}

	return res, nil
}

// loadFile parses a single file and resolves the includes inside it.
func (r *resolver) loadFile(path string) (*yaml.Node, error) {
	r.addSource(path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", r.display(path), err)
	}

	data, unset := substituteEnvVarsReport(data)
	for _, name := range unset {
		if !r.unsetSeen[name] {
			r.unsetSeen[name] = true
			r.unset = append(r.unset, name)
		}
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", r.display(path), err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return &doc, nil
	}

	r.stack = append(r.stack, path)
	defer func() { r.stack = r.stack[:len(r.stack)-1] }()

	content, err := r.resolve(doc.Content[0], path)
	if err != nil {
		return nil, err
	}
	doc.Content[0] = content

	return &doc, nil
}

func (r *resolver) resolve(n *yaml.Node, file string) (*yaml.Node, error) {
	if isIncludeTag(n.Tag) && n.Kind != yaml.ScalarNode {
		return nil, r.errorf(file, n, "%s must be followed by a path", n.Tag)
	}

	switch n.Kind {
	case yaml.ScalarNode:
		switch n.Tag {
		case includeTag:
			return r.includeFile(n, file)
		case includeDirListTag:
			return r.includeDir(n, file)
		}
		if strings.HasPrefix(n.Tag, includeTag) {
			return nil, r.errorf(file, n, "unknown tag %s (supported: %s, %s)", n.Tag, includeTag, includeDirListTag)
		}

	case yaml.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			value := n.Content[i]
			resolved, err := r.resolve(value, file)
			if err != nil {
				return nil, err
			}
			if r.opts.Annotate && isIncludeTag(value.Tag) && resolved.Kind != yaml.SequenceNode {
				addLineComment(n.Content[i-1], "from "+r.display(r.includeTarget(value, file)))
			}
			n.Content[i] = resolved
		}

	case yaml.SequenceNode:
		items := make([]*yaml.Node, 0, len(n.Content))
		for _, item := range n.Content {
			resolved, err := r.resolve(item, file)
			if err != nil {
				return nil, err
			}
			if !isIncludeTag(item.Tag) {
				items = append(items, resolved)
				continue
			}
			// Included lists are spliced into the parent list; an empty file adds nothing.
			switch {
			case resolved.Kind == yaml.SequenceNode:
				items = append(items, resolved.Content...)
			case isNull(resolved):
			default:
				asListItem(resolved)
				if r.opts.Annotate {
					addHeadComment(resolved, fmt.Sprintf("from %s:%d", r.display(r.includeTarget(item, file)), resolved.Line))
				}
				items = append(items, resolved)
			}
		}
		n.Content = items

	case yaml.AliasNode:
		if resolved, ok := r.includeFor[n.Alias]; ok {
			n.Alias = resolved
		}
	}

	return n, nil
}

// includeFile resolves `!include path` to the content of that file.
func (r *resolver) includeFile(n *yaml.Node, file string) (*yaml.Node, error) {
	target, err := r.checkTarget(n, file)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, r.errorf(file, n, "cannot include %q: %s", n.Value, statError(err, r.display(target)))
	}
	if info.IsDir() {
		return nil, r.errorf(file, n, "cannot include %q: is a directory (use %s)", n.Value, includeDirListTag)
	}

	doc, err := r.loadFile(target)
	if err != nil {
		return nil, err
	}

	content := documentContent(doc, n)
	if r.opts.Annotate && content.Kind == yaml.SequenceNode {
		for _, item := range content.Content {
			addHeadComment(item, fmt.Sprintf("from %s:%d", r.display(target), item.Line))
		}
	}

	r.includeFor[n] = content
	return content, nil
}

// includeDir resolves `!include_dir_merge_list dir` to one list holding the
// items of every *.yaml / *.yml file in dir, in filename order.
func (r *resolver) includeDir(n *yaml.Node, file string) (*yaml.Node, error) {
	target, err := r.checkTarget(n, file)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, r.errorf(file, n, "cannot include directory %q: %s", n.Value, statError(err, r.display(target)))
	}
	if !info.IsDir() {
		return nil, r.errorf(file, n, "cannot include directory %q: not a directory (use %s)", n.Value, includeTag)
	}

	r.addSource(target)

	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, r.errorf(file, n, "cannot read directory %q: %v", n.Value, err)
	}

	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Line: n.Line, Column: n.Column}
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if e.IsDir() || strings.HasPrefix(name, ".") || (ext != ".yaml" && ext != ".yml") {
			continue
		}

		path := filepath.Join(target, name)
		if err := r.checkCycle(path, n, file); err != nil {
			return nil, err
		}

		doc, err := r.loadFile(path)
		if err != nil {
			return nil, err
		}

		content := documentContent(doc, n)
		var items []*yaml.Node
		switch {
		case content.Kind == yaml.SequenceNode:
			items = content.Content
		case isNull(content):
			continue
		default:
			asListItem(content)
			items = []*yaml.Node{content}
		}

		for _, item := range items {
			if r.opts.Annotate {
				addHeadComment(item, fmt.Sprintf("from %s:%d", r.display(path), item.Line))
			}
		}
		seq.Content = append(seq.Content, items...)
	}

	r.includeFor[n] = seq
	return seq, nil
}

// checkTarget validates the path of an include tag and returns it as an absolute path.
func (r *resolver) checkTarget(n *yaml.Node, file string) (string, error) {
	if strings.TrimSpace(n.Value) == "" {
		return "", r.errorf(file, n, "%s requires a path", n.Tag)
	}

	target := r.includeTarget(n, file)

	if r.root != "" {
		rel, err := filepath.Rel(r.root, realPath(target))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", r.errorf(file, n, "cannot include %q: path is outside %s", n.Value, r.root)
		}
	}

	if n.Tag == includeTag {
		if err := r.checkCycle(target, n, file); err != nil {
			return "", err
		}
	}

	return target, nil
}

// includeTarget returns the absolute path an include tag points at, relative
// to the directory of the file containing it.
func (r *resolver) includeTarget(n *yaml.Node, file string) string {
	target := strings.TrimSpace(n.Value)
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(file), target)
	}
	return filepath.Clean(target)
}

func (r *resolver) checkCycle(target string, n *yaml.Node, file string) error {
	for _, p := range r.stack {
		if p == target {
			chain := make([]string, 0, len(r.stack)+1)
			for _, s := range r.stack {
				chain = append(chain, r.display(s))
			}
			chain = append(chain, r.display(target))
			return r.errorf(file, n, "include cycle: %s", strings.Join(chain, " -> "))
		}
	}
	return nil
}

func (r *resolver) addSource(path string) {
	if !r.seen[path] {
		r.seen[path] = true
		r.sources = append(r.sources, path)
	}
}

// display returns path relative to the top-level screen.yaml directory when possible.
func (r *resolver) display(path string) string {
	if rel, err := filepath.Rel(r.baseDir, path); err == nil {
		return rel
	}
	return path
}

func (r *resolver) errorf(file string, n *yaml.Node, format string, args ...interface{}) error {
	return fmt.Errorf("%s:%d: %s", r.display(file), n.Line, fmt.Sprintf(format, args...))
}

// statError describes a failed os.Stat of an include target.
func statError(err error, display string) string {
	if errors.Is(err, os.ErrNotExist) {
		return display + " does not exist"
	}
	return err.Error()
}

// documentContent returns the top-level node of a parsed file, or a null node
// (positioned at the include tag) for an empty file.
func documentContent(doc *yaml.Node, at *yaml.Node) *yaml.Node {
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null", Line: at.Line, Column: at.Column}
	}
	return doc.Content[0]
}

func isIncludeTag(tag string) bool {
	return tag == includeTag || tag == includeDirListTag
}

func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}

// asListItem moves a mapping's leading comment from its first key to the
// mapping itself, so it renders above the "- " when placed in a list.
func asListItem(n *yaml.Node) {
	if n.Kind != yaml.MappingNode || len(n.Content) == 0 || n.Content[0].HeadComment == "" {
		return
	}
	if n.HeadComment != "" {
		n.HeadComment += "\n"
	}
	n.HeadComment += n.Content[0].HeadComment
	n.Content[0].HeadComment = ""
}

func addHeadComment(n *yaml.Node, text string) {
	if n.HeadComment != "" {
		n.HeadComment = "# " + text + "\n" + n.HeadComment
		return
	}
	n.HeadComment = "# " + text
}

func addLineComment(n *yaml.Node, text string) {
	if n.LineComment == "" {
		n.LineComment = "# " + text
	}
}

// realPath returns the absolute, symlink-resolved form of path, falling back
// to the absolute path when it does not exist.
func realPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
