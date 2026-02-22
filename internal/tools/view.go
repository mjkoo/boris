package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxViewLines = 2000
	maxLineChars = 2000
)

// excluded directories in directory listings
var excludedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
}

// ViewRange is a custom type for view_range so that the JSON schema
// generates {"type": "array"} instead of {"type": ["null", "array"]}.
type ViewRange []int

// ViewArgs is the input schema for the view tool.
type ViewArgs struct {
	Path      string    `json:"path" jsonschema:"file or directory path to view"`
	ViewRange ViewRange `json:"view_range,omitempty" jsonschema:"optional line range [start end] (1-indexed)"`
}

func viewHandler(sess *session.Session, resolver *pathscope.Resolver, maxFileSize int64) mcp.ToolHandlerFor[ViewArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args ViewArgs) (*mcp.CallToolResult, any, error) {
		return doView(sess, resolver, maxFileSize, args.Path, args.ViewRange)
	}
}

func doView(sess *session.Session, resolver *pathscope.Resolver, maxFileSize int64, path string, viewRange []int) (*mcp.CallToolResult, any, error) {
	resolved, err := resolver.Resolve(sess.Cwd(), path)
	if err != nil {
		return toolErr(ErrAccessDenied, "path not allowed: %v", err)
	}

	info, err := os.Lstat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return toolErr(ErrPathNotFound, "%s does not exist", resolved)
		}
		return toolErr(ErrIO, "could not stat %s: %v", resolved, err)
	}

	if info.IsDir() {
		text, err := listDirectory(resolved)
		if err != nil {
			return toolErr(ErrIO, "could not list directory %s: %v", resolved, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	}

	return readFile(resolved, info, viewRange, maxFileSize)
}

func readFile(path string, info os.FileInfo, viewRange []int, maxFileSize int64) (*mcp.CallToolResult, any, error) {
	if info.Size() > maxFileSize {
		return toolErr(ErrFileTooLarge, "file %s is %d bytes, exceeds maximum %d bytes", path, info.Size(), maxFileSize)
	}

	// Binary/image detection: check first 512 bytes
	f, err := os.Open(path)
	if err != nil {
		return toolErr(ErrIO, "could not open %s: %v", path, err)
	}
	defer f.Close()

	header := make([]byte, 512)
	n, _ := f.Read(header)
	header = header[:n]

	// Check for image content
	if mime, ok := detectImage(header, path); ok {
		// Read the full file for image content
		if _, err := f.Seek(0, 0); err != nil {
			return toolErr(ErrIO, "could not seek %s: %v", path, err)
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return toolErr(ErrIO, "could not read %s: %v", path, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{
				Data:     data,
				MIMEType: mime,
			}},
		}, nil, nil
	}

	// Check for binary (NUL bytes in header)
	if isBinaryHeader(header) {
		text := fmt.Sprintf("Binary file (%s)", formatSize(info.Size()))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	}

	// For view_range requests, use efficient range reading
	if len(viewRange) == 2 {
		return readFileRange(f, path, viewRange[0], viewRange[1])
	}

	// Read entire file
	if _, err := f.Seek(0, 0); err != nil {
		return toolErr(ErrIO, "could not seek %s: %v", path, err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return toolErr(ErrIO, "could not read %s: %v", path, err)
	}

	lines := strings.Split(string(data), "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	totalLines := len(lines)

	if totalLines > maxViewLines {
		lines = lines[:maxViewLines]
		text := formatLines(lines, 1)
		text += fmt.Sprintf("\n[Truncated: file has %d lines. Use view_range to read specific sections.]", totalLines)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	}

	text := formatLines(lines, 1)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

// readFileRange reads a specific line range from an already-opened file using
// a scanner to avoid loading the entire file into memory.
func readFileRange(f *os.File, path string, start, end int) (*mcp.CallToolResult, any, error) {
	if start < 1 {
		return toolErr(ErrInvalidInput, "invalid view_range: start must be >= 1, got %d", start)
	}
	if start > end {
		return toolErr(ErrInvalidInput, "invalid view_range: start %d > end %d", start, end)
	}

	if _, err := f.Seek(0, 0); err != nil {
		return toolErr(ErrIO, "could not seek %s: %v", path, err)
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	var lines []string
	for scanner.Scan() {
		lineNum++
		if lineNum >= start && lineNum <= end {
			lines = append(lines, scanner.Text())
		}
		if lineNum > end {
			break
		}
	}
	// Continue scanning to get totalLines for validation
	for scanner.Scan() {
		lineNum++
	}
	totalLines := lineNum

	if start > totalLines {
		return toolErr(ErrInvalidInput, "invalid view_range: start %d exceeds total lines %d in %s", start, totalLines, path)
	}

	// Clamp end to totalLines (already handled by scan stopping)
	text := formatLines(lines, start)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

// detectImage checks if the header bytes represent an image format.
// Uses net/http.DetectContentType for magic byte sniffing, with SVG
// extension fallback since SVG is text-based.
func detectImage(header []byte, path string) (string, bool) {
	if len(header) > 0 {
		mime := http.DetectContentType(header)
		if strings.HasPrefix(mime, "image/") {
			return mime, true
		}
	}
	// SVG fallback: text-based format not detected by magic bytes
	if strings.ToLower(filepath.Ext(path)) == ".svg" {
		return "image/svg+xml", true
	}
	return "", false
}

// truncateLine caps a single line at maxLineChars characters.
func truncateLine(line string) string {
	if len(line) <= maxLineChars {
		return line
	}
	return line[:maxLineChars] + fmt.Sprintf("... [truncated, %d chars total]", len(line))
}

func formatLines(lines []string, startNum int) string {
	var b strings.Builder
	width := len(fmt.Sprintf("%d", startNum+len(lines)-1))
	for i, line := range lines {
		fmt.Fprintf(&b, "%*d\t%s\n", width, startNum+i, truncateLine(line))
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

	// Filter only specifically excluded directories
	var visible []os.DirEntry
	for _, e := range entries {
		if excludedDirs[e.Name()] {
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
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(filepath.Join(path, name))
			if err == nil {
				name += " -> " + target
			}
		} else if entry.IsDir() {
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
