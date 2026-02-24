package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateFileArgs is the input schema for the create_file tool.
type CreateFileArgs struct {
	Path    string `json:"path" jsonschema:"file path to create or overwrite"`
	Content string `json:"content" jsonschema:"file content"`
}

func createFileHandler(sess *session.Session, resolver *pathscope.Resolver, cfg Config) mcp.ToolHandlerFor[CreateFileArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args CreateFileArgs) (*mcp.CallToolResult, any, error) {
		return doCreateFile(sess, resolver, cfg, args.Path, args.Content)
	}
}

func doCreateFile(sess *session.Session, resolver *pathscope.Resolver, cfg Config, path, content string) (*mcp.CallToolResult, any, error) {
	if int64(len(content)) > cfg.MaxFileSize {
		return toolErr(ErrFileTooLarge, "content is %d bytes, exceeds maximum %d bytes", len(content), cfg.MaxFileSize)
	}

	resolved, err := resolver.Resolve(sess.Cwd(), path)
	if err != nil {
		return toolErr(ErrAccessDenied, "path not allowed: %v", err)
	}

	// Check view-before-edit for overwrites of existing files
	if cfg.RequireViewBeforeEdit {
		if _, statErr := os.Stat(resolved); statErr == nil {
			// File exists — this is an overwrite, check if it was viewed
			if !sess.HasViewed(resolved) {
				return toolErr(ErrFileNotViewed, "file %s must be viewed before overwriting. Use the view tool first.", resolved)
			}
		}
	}

	// Create parent directories
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return toolErr(ErrIO, "could not create directories for %s: %v", resolved, err)
	}

	// Write file (overwrites if exists)
	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return toolErr(ErrIO, "could not write %s: %v", resolved, err)
	}

	text := fmt.Sprintf("Created %s (%d bytes)", resolved, len(content))
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}
