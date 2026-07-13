package fastfs

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Default skip directory names (always ignored, even without .gitignore).
var defaultSkipDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"build":        {},
	".next":        {},
	"target":       {},
	"__pycache__":  {},
	".turbo":       {},
	".cache":       {},
	"coverage":     {},
}

// IgnoreMatcher applies default skips + simple .gitignore patterns from root.
type IgnoreMatcher struct {
	// patterns are gitignore-style lines (no negation for v1 simplicity).
	patterns []string
}

// LoadIgnore reads .gitignore at workspace root (best-effort).
func LoadIgnore(rootAbs string) *IgnoreMatcher {
	m := &IgnoreMatcher{}
	f, err := os.Open(filepath.Join(rootAbs, ".gitignore"))
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// v1: skip negation patterns
		if strings.HasPrefix(line, "!") {
			continue
		}
		m.patterns = append(m.patterns, line)
	}
	return m
}

// SkipDir reports whether a directory name (base) should not be descended into.
func (m *IgnoreMatcher) SkipDir(base string) bool {
	if _, ok := defaultSkipDirs[base]; ok {
		return true
	}
	return m.match(base+"/") || m.match(base)
}

// SkipFile reports whether a relative path (slash-separated) should be skipped.
func (m *IgnoreMatcher) SkipFile(relSlash string) bool {
	base := filepath.Base(relSlash)
	if _, ok := defaultSkipDirs[base]; ok {
		return true
	}
	return m.match(relSlash) || m.match(base)
}

func (m *IgnoreMatcher) match(path string) bool {
	path = filepath.ToSlash(path)
	for _, p := range m.patterns {
		if matchGitIgnore(p, path) {
			return true
		}
	}
	return false
}

// matchGitIgnore is a minimal subset: exact, trailing slash dir, leading **,
// simple * wildcards via filepath.Match on the full path or basename.
func matchGitIgnore(pattern, path string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	pattern = filepath.ToSlash(pattern)
	// Directory-only patterns end with /
	dirOnly := strings.HasSuffix(pattern, "/")
	if dirOnly {
		pattern = strings.TrimSuffix(pattern, "/")
	}
	// **/foo → suffix match
	if strings.HasPrefix(pattern, "**/") {
		suf := strings.TrimPrefix(pattern, "**/")
		if path == suf || strings.HasSuffix(path, "/"+suf) {
			return true
		}
		ok, _ := filepath.Match(suf, filepath.Base(path))
		return ok
	}
	if strings.Contains(pattern, "/") {
		ok, _ := filepath.Match(pattern, path)
		if ok {
			return true
		}
		// also try without leading ./
		ok, _ = filepath.Match(strings.TrimPrefix(pattern, "./"), path)
		return ok
	}
	// pattern is basename-style: match any path segment
	ok, _ := filepath.Match(pattern, filepath.Base(path))
	if ok {
		return true
	}
	// path itself
	ok, _ = filepath.Match(pattern, path)
	return ok
}
