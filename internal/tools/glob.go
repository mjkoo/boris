package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GlobArgs is the input schema for the glob tool (normal MCP mode).
type GlobArgs struct {
	Pattern string `json:"pattern" jsonschema:"the glob pattern to match files against,required"`
	Path    string `json:"path,omitempty" jsonschema:"the directory to search in (defaults to cwd)"`
	Type    string `json:"type,omitempty" jsonschema:"filter by type: file or directory"`
}

// GlobCompatArgs is the input schema for the glob tool in --anthropic-compat mode.
type GlobCompatArgs struct {
	Pattern string `json:"pattern" jsonschema:"the glob pattern to match files against,required"`
	Path    string `json:"path,omitempty" jsonschema:"the directory to search in. If not specified, the current working directory will be used. IMPORTANT: Omit this field to use the default directory. DO NOT enter \"undefined\" or \"null\" - simply omit it for the default behavior. Must be a valid directory path if provided."`
}

// globParams holds the normalized parameters for glob.
type globParams struct {
	pattern    string
	path       string
	filterType string // "", "file", or "directory"
}

func normalizeGlobArgs(args GlobArgs) globParams {
	return globParams{
		pattern:    args.Pattern,
		path:       args.Path,
		filterType: args.Type,
	}
}

func normalizeGlobCompatArgs(args GlobCompatArgs) globParams {
	return globParams{
		pattern: args.Pattern,
		path:    args.Path,
	}
}

func globHandler(sess *session.Session, resolver *pathscope.Resolver) mcp.ToolHandlerFor[GlobArgs, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, args GlobArgs) (*mcp.CallToolResult, any, error) {
		return doGlob(ctx, sess, resolver, normalizeGlobArgs(args))
	}
}

func globCompatHandler(sess *session.Session, resolver *pathscope.Resolver) mcp.ToolHandlerFor[GlobCompatArgs, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, args GlobCompatArgs) (*mcp.CallToolResult, any, error) {
		return doGlob(ctx, sess, resolver, normalizeGlobCompatArgs(args))
	}
}

const globMaxOutputChars = 30000

func doGlob(ctx context.Context, sess *session.Session, resolver *pathscope.Resolver, p globParams) (*mcp.CallToolResult, any, error) {
	// Validate pattern
	if p.pattern == "" {
		return toolErr(ErrInvalidInput, "pattern must not be empty")
	}
	if !doublestar.ValidatePattern(p.pattern) {
		return toolErr(ErrGlobInvalidPattern, "invalid glob pattern: %s", p.pattern)
	}

	// Validate type filter
	switch p.filterType {
	case "", "file", "directory":
		// valid
	default:
		return toolErr(ErrGlobInvalidType, "invalid type %q; valid values: file, directory", p.filterType)
	}

	// Check path scoping on the search root
	resolvedRoot, err := resolver.Resolve(sess.Cwd(), p.path)
	if err != nil {
		if p.path == "" {
			resolvedRoot = sess.Cwd()
		} else {
			return toolErr(ErrAccessDenied, "path not allowed: %v", err)
		}
	}

	info, err := os.Lstat(resolvedRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return globNoFiles()
		}
		return toolErr(ErrIO, "could not stat %s: %v", p.path, err)
	}
	if !info.IsDir() {
		return globNoFiles()
	}

	// Walk and collect results
	type globResult struct {
		relPath string
		modTime int64
	}

	gi := newGitignoreStack()
	var results []globResult

	var walkFn func(dir string) error
	walkFn = func(dir string) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		gi.push(dir)
		defer gi.pop()

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil // silently skip unreadable directories
		}

		for _, entry := range entries {
			// Check context cancellation per entry
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			name := entry.Name()
			entryPath := filepath.Join(dir, name)

			// Skip .git and node_modules
			if name == ".git" || name == "node_modules" {
				continue
			}

			isDir := entry.IsDir()
			isSymlink := entry.Type()&os.ModeSymlink != 0

			// For symlinks, determine if target is a directory
			if isSymlink {
				targetInfo, err := os.Stat(entryPath)
				if err != nil {
					// Broken symlink - skip silently
					continue
				}
				if targetInfo.IsDir() {
					// Directory symlink - do NOT follow, do NOT recurse,
					// do NOT include in results. Matches Claude Code behavior
					// where directory symlinks are invisible to glob.
					continue
				}
				// File symlink - include if it matches, don't mark as dir
				isDir = false
			}

			// Check gitignore
			if gi.isIgnored(entryPath, isDir) {
				continue
			}

			if isDir {
				// Check if directory matches pattern (for directory type filter)
				relPath, err := filepath.Rel(resolvedRoot, entryPath)
				if err == nil && matchesGlobPattern(p.pattern, relPath, name) && (p.filterType == "" || p.filterType == "directory") {
					resolvedFile, err := resolver.Resolve(sess.Cwd(), entryPath)
					if err == nil {
						fInfo, err := os.Lstat(resolvedFile)
						if err == nil {
							results = append(results, globResult{
								relPath: relPath,
								modTime: fInfo.ModTime().Unix(),
							})
						}
					}
				}
				// Recurse into directory
				if err := walkFn(entryPath); err != nil {
					return err
				}
				continue
			}

			// File (regular or file symlink)
			relPath, err := filepath.Rel(resolvedRoot, entryPath)
			if err != nil {
				continue
			}

			if !matchesGlobPattern(p.pattern, relPath, name) {
				continue
			}

			// Apply type filter
			if p.filterType == "directory" {
				continue
			}

			// Path scoping: silently skip denied files
			resolvedFile, err := resolver.Resolve(sess.Cwd(), entryPath)
			if err != nil {
				continue
			}

			fInfo, err := os.Lstat(resolvedFile)
			if err != nil {
				continue
			}

			results = append(results, globResult{
				relPath: relPath,
				modTime: fInfo.ModTime().Unix(),
			})
		}
		return nil
	}

	if err := walkFn(resolvedRoot); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return toolErr(ErrIO, "could not walk directory %s: %v", p.path, err)
	}

	if len(results) == 0 {
		return globNoFiles()
	}

	// Sort by mtime descending (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].modTime > results[j].modTime
	})

	// Join paths and truncate at last complete line
	var out strings.Builder
	truncated := false
	for i, r := range results {
		line := r.relPath
		if i > 0 {
			line = "\n" + line
		}
		if out.Len()+len(line) > globMaxOutputChars {
			truncated = true
			break
		}
		out.WriteString(line)
	}

	output := out.String()
	if truncated {
		output += "\n... output truncated (exceeded 30,000 characters)"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
	}, nil, nil
}

func globNoFiles() (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "No files found"}},
	}, nil, nil
}

// matchesGlobPattern checks if an entry matches the glob pattern.
// It matches against both the full relative path and the base name.
func matchesGlobPattern(pattern, relPath, baseName string) bool {
	if matched, err := doublestar.Match(pattern, relPath); err == nil && matched {
		return true
	}
	if matched, err := doublestar.Match(pattern, baseName); err == nil && matched {
		return true
	}
	return false
}
