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
	Path       string `json:"path" jsonschema:"file path"`
	OldStr     string `json:"old_str" jsonschema:"the string to find (must be unique unless replace_all is true)"`
	NewStr     string `json:"new_str,omitempty" jsonschema:"replacement string (empty or omitted to delete)"`
	ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"replace all occurrences instead of requiring a unique match"`
}

func strReplaceHandler(sess *session.Session, resolver *pathscope.Resolver) mcp.ToolHandlerFor[StrReplaceArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args StrReplaceArgs) (*mcp.CallToolResult, any, error) {
		return doStrReplace(sess, resolver, args.Path, args.OldStr, args.NewStr, args.ReplaceAll)
	}
}

func doStrReplace(sess *session.Session, resolver *pathscope.Resolver, path, oldStr, newStr string, replaceAll bool) (*mcp.CallToolResult, any, error) {
	if oldStr == "" {
		return toolErr(ErrInvalidInput, "old_str must not be empty")
	}

	resolved, err := resolver.Resolve(sess.Cwd(), path)
	if err != nil {
		return toolErr(ErrAccessDenied, "path not allowed: %v", err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return toolErr(ErrPathNotFound, "%s does not exist", resolved)
		}
		return toolErr(ErrIO, "could not stat %s: %v", resolved, err)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return toolErr(ErrIO, "could not read %s: %v", resolved, err)
	}
	content := string(data)

	count := strings.Count(content, oldStr)
	if count == 0 {
		return toolErr(ErrStrReplaceNotFound, "old_str not found in %s", resolved)
	}

	if replaceAll {
		newContent := strings.ReplaceAll(content, oldStr, newStr)
		if err := os.WriteFile(resolved, []byte(newContent), info.Mode().Perm()); err != nil {
			return toolErr(ErrIO, "could not write %s: %v", resolved, err)
		}
		text := fmt.Sprintf("Replaced %d occurrences in %s", count, resolved)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	}

	if count > 1 {
		return toolErr(ErrStrReplaceAmbiguous, "found %d occurrences in %s; match must be unique (use replace_all to replace all)", count, resolved)
	}

	offset := strings.Index(content, oldStr)
	newContent := strings.Replace(content, oldStr, newStr, 1)

	// Preserve file permissions
	if err := os.WriteFile(resolved, []byte(newContent), info.Mode().Perm()); err != nil {
		return toolErr(ErrIO, "could not write %s: %v", resolved, err)
	}

	// Build context snippet around the replacement
	snippet := contextSnippet(newContent, offset)

	text := fmt.Sprintf("Replaced in %s\n\n%s", resolved, snippet)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
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
