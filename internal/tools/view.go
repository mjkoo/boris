package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxViewLines = 2000

// ViewArgs is the input schema for the view tool.
type ViewArgs struct {
	Path      string `json:"path" jsonschema:"file or directory path to view"`
	ViewRange []int  `json:"view_range,omitempty" jsonschema:"optional line range [start end] (1-indexed)"`
}

func viewHandler(sess *session.Session, resolver *pathscope.Resolver, maxFileSize int64) mcp.ToolHandlerFor[ViewArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args ViewArgs) (*mcp.CallToolResult, any, error) {
		resolved, err := resolver.Resolve(sess.Cwd(), args.Path)
		if err != nil {
			return nil, nil, err
		}

		info, err := os.Stat(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil, fmt.Errorf("path not found: %s", resolved)
			}
			return nil, nil, err
		}

		var text string
		if info.IsDir() {
			text, err = listDirectory(resolved)
		} else {
			text, err = readFile(resolved, info, args.ViewRange, maxFileSize)
		}
		if err != nil {
			return nil, nil, err
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	}
}

func readFile(path string, info os.FileInfo, viewRange []int, maxFileSize int64) (string, error) {
	if info.Size() > maxFileSize {
		return "", fmt.Errorf("file size %d bytes exceeds maximum %d bytes", info.Size(), maxFileSize)
	}

	// Binary detection: check first 512 bytes for NUL
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	header := make([]byte, 512)
	n, _ := f.Read(header)
	header = header[:n]
	for _, b := range header {
		if b == 0 {
			return fmt.Sprintf("Binary file (%s)", formatSize(info.Size())), nil
		}
	}

	// Read entire file from the already-open handle
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	totalLines := len(lines)

	if len(viewRange) == 2 {
		start, end := viewRange[0], viewRange[1]
		if start < 1 {
			return "", fmt.Errorf("invalid view_range: start must be >= 1, got %d", start)
		}
		if end > totalLines {
			return "", fmt.Errorf("invalid view_range: end %d exceeds total lines %d", end, totalLines)
		}
		if start > end {
			return "", fmt.Errorf("invalid view_range: start %d > end %d", start, end)
		}
		lines = lines[start-1 : end]
		return formatLines(lines, start), nil
	}

	if totalLines > maxViewLines {
		lines = lines[:maxViewLines]
		text := formatLines(lines, 1)
		text += fmt.Sprintf("\n[Truncated: file has %d lines. Use view_range to read specific sections.]", totalLines)
		return text, nil
	}

	return formatLines(lines, 1), nil
}

func formatLines(lines []string, startNum int) string {
	var b strings.Builder
	width := len(fmt.Sprintf("%d", startNum+len(lines)-1))
	for i, line := range lines {
		fmt.Fprintf(&b, "%*d\t%s\n", width, startNum+i, line)
	}
	return b.String()
}

func formatSize(size int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case size >= gb:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(gb))
	case size >= mb:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}

func listDirectory(path string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s/\n", filepath.Base(path))
	err := walkDir(path, "", 0, 2, &b)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func walkDir(path string, prefix string, depth int, maxDepth int, b *strings.Builder) error {
	if depth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	// Filter hidden files and node_modules
	var visible []os.DirEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		visible = append(visible, e)
	}

	for i, entry := range visible {
		isLast := i == len(visible)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		fmt.Fprintf(b, "%s%s%s\n", prefix, connector, name)

		if entry.IsDir() {
			childPrefix := prefix + "│   "
			if isLast {
				childPrefix = prefix + "    "
			}
			if err := walkDir(filepath.Join(path, entry.Name()), childPrefix, depth+1, maxDepth, b); err != nil {
				return err
			}
		}
	}
	return nil
}
