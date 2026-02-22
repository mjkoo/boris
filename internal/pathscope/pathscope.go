package pathscope

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Resolver checks paths against allow/deny lists.
type Resolver struct {
	allowDirs    []string
	denyPatterns []string
}

// NewResolver creates a Resolver. allowDirs are canonicalized at construction time.
// If allowDirs is empty, all paths are allowed (canonicalization only).
// denyPatterns support doublestar glob syntax.
func NewResolver(allowDirs []string, denyPatterns []string) (*Resolver, error) {
	canonical := make([]string, 0, len(allowDirs))
	for _, d := range allowDirs {
		abs, err := filepath.Abs(d)
		if err != nil {
			return nil, fmt.Errorf("allow dir %q: %w", d, err)
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("allow dir %q: %w", d, err)
		}
		canonical = append(canonical, resolved)
	}
	return &Resolver{allowDirs: canonical, denyPatterns: denyPatterns}, nil
}

// Resolve canonicalizes a path and checks it against allow/deny lists.
// baseCwd is the session's current working directory, used to resolve relative paths.
func (r *Resolver) Resolve(baseCwd string, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseCwd, path)
	}

	resolved, err := resolveSymlinks(path)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}

	// Check allow list
	if len(r.allowDirs) > 0 {
		allowed := false
		for _, dir := range r.allowDirs {
			if resolved == dir || strings.HasPrefix(resolved, dir+string(filepath.Separator)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("access denied: path %q is outside allowed directories", resolved)
		}
	}

	// Check deny list (deny overrides allow)
	if pattern, matched := r.matchesDeny(resolved); matched {
		return "", fmt.Errorf("access denied: path %q matches deny pattern %q", resolved, pattern)
	}

	return resolved, nil
}

// matchesDeny checks if the resolved path or any of its parent directories
// match a deny pattern. Returns the matching pattern and true if denied.
func (r *Resolver) matchesDeny(resolved string) (string, bool) {
	for _, pattern := range r.denyPatterns {
		// Check the path itself
		if matched, _ := doublestar.PathMatch(pattern, resolved); matched {
			return pattern, true
		}
		// Check parent directories (handles patterns like **/.git when
		// path is /workspace/.git/config)
		dir := resolved
		for {
			dir = filepath.Dir(dir)
			if dir == "/" || dir == "." {
				break
			}
			if matched, _ := doublestar.PathMatch(pattern, dir); matched {
				return pattern, true
			}
		}
	}
	return "", false
}

// resolveSymlinks resolves symlinks for paths that may not fully exist yet.
// It walks up the path tree until finding an existing component, resolves
// symlinks on that part, then joins with the remaining components.
func resolveSymlinks(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	parent := filepath.Dir(path)
	base := filepath.Base(path)
	if parent == path {
		return path, nil
	}

	resolvedParent, err := resolveSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, base), nil
}
