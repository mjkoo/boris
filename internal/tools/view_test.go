package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
)

func TestViewEntireFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	os.WriteFile(file, []byte("line1\nline2\nline3\n"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "1\tline1") {
		t.Errorf("expected line numbers, got: %s", text)
	}
	if !strings.Contains(text, "3\tline3") {
		t.Errorf("expected all lines, got: %s", text)
	}
}

func TestViewLineRange(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	var content string
	for i := 1; i <= 100; i++ {
		content += "line" + strings.Repeat("x", i) + "\n"
	}
	os.WriteFile(file, []byte(content), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: file, ViewRange: []int{10, 20}})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 11 {
		t.Errorf("expected 11 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "10\t") {
		t.Errorf("first line should start with line number 10: %s", lines[0])
	}
}

func TestViewInvalidRange(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	os.WriteFile(file, []byte("a\nb\nc\n"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	tests := []struct {
		name      string
		viewRange []int
	}{
		{"start < 1", []int{0, 2}},
		{"start > end", []int{3, 1}},
		{"end > total", []int{1, 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := handler(context.Background(), nil, ViewArgs{Path: file, ViewRange: tt.viewRange})
			if err == nil {
				t.Error("expected error for invalid view_range")
			}
		})
	}
}

func TestViewLargeFileTruncation(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "large.txt")
	var content string
	for i := 0; i < 5000; i++ {
		content += "line\n"
	}
	os.WriteFile(file, []byte(content), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 100*1024*1024)

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "Truncated") {
		t.Error("expected truncation message")
	}
	if !strings.Contains(text, "5000") {
		t.Error("expected total line count in truncation message")
	}
}

func TestViewBinaryDetection(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "binary.bin")
	data := make([]byte, 1024)
	data[100] = 0 // NUL byte
	os.WriteFile(file, data, 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "Binary file") {
		t.Errorf("expected binary file message, got: %s", text)
	}
}

func TestViewDirectoryListing(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "src", "pkg"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "main.go"), []byte("m"), 0644)
	os.MkdirAll(filepath.Join(tmp, ".git"), 0755)
	os.MkdirAll(filepath.Join(tmp, "node_modules"), 0755)
	os.WriteFile(filepath.Join(tmp, ".env"), []byte("e"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: tmp})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "src/") {
		t.Error("expected src/ in listing")
	}
	if strings.Contains(text, ".git") {
		t.Error("expected .git to be excluded")
	}
	if strings.Contains(text, "node_modules") {
		t.Error("expected node_modules to be excluded")
	}
	if strings.Contains(text, ".env") {
		t.Error("expected .env to be excluded")
	}
}

func TestViewRelativePath(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "sub"), 0755)
	file := filepath.Join(tmp, "sub", "test.txt")
	os.WriteFile(file, []byte("content\n"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: "sub/test.txt"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "content") {
		t.Errorf("expected 'content' in output, got: %s", text)
	}
}

func TestViewPathScopingEnforcement(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver([]string{tmp}, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	_, _, err := handler(context.Background(), nil, ViewArgs{Path: "/etc/hostname"})
	if err == nil {
		t.Error("expected path scoping error")
	}
}

func TestViewFileNotFound(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	_, _, err := handler(context.Background(), nil, ViewArgs{Path: filepath.Join(tmp, "nonexistent")})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestViewMaxFileSize(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "big.txt")
	os.WriteFile(file, make([]byte, 1024), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 100) // 100 byte limit

	_, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
	if err == nil {
		t.Error("expected error for file exceeding max size")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("expected size error message, got: %v", err)
	}
}
