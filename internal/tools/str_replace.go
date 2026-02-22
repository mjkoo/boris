package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// StrReplaceArgs is the input schema for the str_replace tool.
type StrReplaceArgs struct {
	Path   string `json:"path" jsonschema:"file path"`
	OldStr string `json:"old_str" jsonschema:"the string to find (must be unique in the file)"`
	NewStr string `json:"new_str,omitempty" jsonschema:"replacement string (empty to delete)"`
}

func strReplaceHandler(sess *session.Session, resolver *pathscope.Resolver) mcp.ToolHandlerFor[StrReplaceArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args StrReplaceArgs) (*mcp.CallToolResult, any, error) {
		resolved, err := resolver.Resolve(sess.Cwd(), args.Path)
		if err != nil {
			return nil, nil, err
		}

		info, err := os.Stat(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil, fmt.Errorf("file not found: %s", resolved)
			}
			return nil, nil, err
		}

		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, nil, err
		}
		content := string(data)

		count := strings.Count(content, args.OldStr)
		if count == 0 {
			return nil, nil, fmt.Errorf("string not found in %s", resolved)
		}
		if count > 1 {
			return nil, nil, fmt.Errorf("found %d occurrences of the string in %s; match must be unique", count, resolved)
		}

		offset := strings.Index(content, args.OldStr)
		newContent := strings.Replace(content, args.OldStr, args.NewStr, 1)

		// Preserve file permissions
		if err := os.WriteFile(resolved, []byte(newContent), info.Mode().Perm()); err != nil {
			return nil, nil, fmt.Errorf("failed to write file: %w", err)
		}

		// Build context snippet around the replacement
		snippet := contextSnippet(newContent, offset)

		text := fmt.Sprintf("Replaced in %s\n\n%s", resolved, snippet)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	}
}

const snippetContext = 4

// contextSnippet returns a few lines of context around the given byte offset.
func contextSnippet(content string, offset int) string {
	if content == "" {
		return ""
	}
	// Clamp offset to valid range so deletions at end of file still show context
	if offset < 0 {
		offset = 0
	}
	if offset >= len(content) {
		offset = len(content) - 1
	}

	lines := strings.Split(content, "\n")
	// Find which line the offset falls on
	charCount := 0
	targetLine := 0
	for i, line := range lines {
		charCount += len(line) + 1 // +1 for newline
		if charCount > offset {
			targetLine = i
			break
		}
	}

	start := targetLine - snippetContext
	if start < 0 {
		start = 0
	}
	end := targetLine + snippetContext + 1
	if end > len(lines) {
		end = len(lines)
	}

	return formatLines(lines[start:end], start+1)
}
