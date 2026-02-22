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
	Path      string `json:"path" jsonschema:"file path to create"`
	Content   string `json:"content" jsonschema:"file content"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"allow overwriting existing files"`
}

func createFileHandler(sess *session.Session, resolver *pathscope.Resolver, maxFileSize int64) mcp.ToolHandlerFor[CreateFileArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args CreateFileArgs) (*mcp.CallToolResult, any, error) {
		if int64(len(args.Content)) > maxFileSize {
			return nil, nil, fmt.Errorf("content size %d bytes exceeds maximum %d bytes", len(args.Content), maxFileSize)
		}

		resolved, err := resolver.Resolve(sess.Cwd(), args.Path)
		if err != nil {
			return nil, nil, err
		}

		// Check if file exists
		if _, err := os.Stat(resolved); err == nil {
			if !args.Overwrite {
				return nil, nil, fmt.Errorf("file already exists: %s (use overwrite: true to replace)", resolved)
			}
		}

		// Create parent directories
		dir := filepath.Dir(resolved)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create directories: %w", err)
		}

		// Write file
		if err := os.WriteFile(resolved, []byte(args.Content), 0644); err != nil {
			return nil, nil, fmt.Errorf("failed to write file: %w", err)
		}

		text := fmt.Sprintf("Created %s (%d bytes)", resolved, len(args.Content))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	}
}
