package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func TestViewRangeEndClamped(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	// 42-line file
	var content string
	for i := 1; i <= 42; i++ {
		content += "line\n"
	}
	os.WriteFile(file, []byte(content), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	// End exceeds total lines — should be clamped, not error
	result, _, err := handler(context.Background(), nil, ViewArgs{Path: file, ViewRange: []int{10, 100}})
	if err != nil {
		t.Fatal(err)
	}
	if isErrorResult(result) {
		t.Errorf("end clamping should not produce error, got: %s", resultText(result))
	}
	text := resultText(result)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	// Should return lines 10-42 = 33 lines
	if len(lines) != 33 {
		t.Errorf("expected 33 lines (10-42), got %d", len(lines))
	}
}

func TestViewRangeStartExceedsTotal(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	os.WriteFile(file, []byte("a\nb\nc\n"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: file, ViewRange: []int{100, 200}})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("start > totalLines should produce IsError")
	}
	if !hasErrorCode(result, ErrInvalidInput) {
		t.Errorf("expected error code %s, got: %s", ErrInvalidInput, resultText(result))
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := handler(context.Background(), nil, ViewArgs{Path: file, ViewRange: tt.viewRange})
			if err != nil {
				t.Fatal(err)
			}
			if !isErrorResult(result) {
				t.Error("expected IsError for invalid view_range")
			}
			if !hasErrorCode(result, ErrInvalidInput) {
				t.Errorf("expected error code %s, got: %s", ErrInvalidInput, resultText(result))
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

func TestViewLineTruncation(t *testing.T) {
	tmp := t.TempDir()

	t.Run("within limit", func(t *testing.T) {
		file := filepath.Join(tmp, "short.txt")
		os.WriteFile(file, []byte("short line\n"), 0644)

		sess := session.New(tmp)
		resolver, _ := pathscope.NewResolver(nil, nil)
		handler := viewHandler(sess, resolver, 10*1024*1024)

		result, _, _ := handler(context.Background(), nil, ViewArgs{Path: file})
		text := resultText(result)
		if strings.Contains(text, "truncated") {
			t.Error("short line should not be truncated")
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		file := filepath.Join(tmp, "long.txt")
		longLine := strings.Repeat("x", 5000) + "\n"
		os.WriteFile(file, []byte(longLine), 0644)

		sess := session.New(tmp)
		resolver, _ := pathscope.NewResolver(nil, nil)
		handler := viewHandler(sess, resolver, 10*1024*1024)

		result, _, _ := handler(context.Background(), nil, ViewArgs{Path: file})
		text := resultText(result)
		if !strings.Contains(text, "truncated") {
			t.Error("long line should have truncation suffix")
		}
		if !strings.Contains(text, "5000 chars total") {
			t.Errorf("should show total char count, got: %s", text)
		}
	})
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

func TestViewImageDetection(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	t.Run("PNG via magic bytes", func(t *testing.T) {
		file := filepath.Join(tmp, "image.png")
		// PNG magic bytes
		data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		data = append(data, make([]byte, 100)...)
		os.WriteFile(file, data, 0644)

		result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content")
		}
		img, ok := result.Content[0].(*mcp.ImageContent)
		if !ok {
			t.Fatalf("expected ImageContent, got %T", result.Content[0])
		}
		if img.MIMEType != "image/png" {
			t.Errorf("expected image/png, got %s", img.MIMEType)
		}
	})

	t.Run("JPEG via magic bytes", func(t *testing.T) {
		file := filepath.Join(tmp, "image.jpg")
		// JPEG magic bytes
		data := []byte{0xFF, 0xD8, 0xFF}
		data = append(data, make([]byte, 100)...)
		os.WriteFile(file, data, 0644)

		result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
		if err != nil {
			t.Fatal(err)
		}
		img, ok := result.Content[0].(*mcp.ImageContent)
		if !ok {
			t.Fatalf("expected ImageContent, got %T", result.Content[0])
		}
		if !strings.HasPrefix(img.MIMEType, "image/") {
			t.Errorf("expected image/* MIME, got %s", img.MIMEType)
		}
	})

	t.Run("GIF via magic bytes", func(t *testing.T) {
		file := filepath.Join(tmp, "image.gif")
		// GIF magic bytes
		data := []byte("GIF89a")
		data = append(data, make([]byte, 100)...)
		os.WriteFile(file, data, 0644)

		result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
		if err != nil {
			t.Fatal(err)
		}
		img, ok := result.Content[0].(*mcp.ImageContent)
		if !ok {
			t.Fatalf("expected ImageContent, got %T", result.Content[0])
		}
		if img.MIMEType != "image/gif" {
			t.Errorf("expected image/gif, got %s", img.MIMEType)
		}
	})

	t.Run("SVG via extension", func(t *testing.T) {
		file := filepath.Join(tmp, "icon.svg")
		os.WriteFile(file, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="10"/></svg>`), 0644)

		result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
		if err != nil {
			t.Fatal(err)
		}
		img, ok := result.Content[0].(*mcp.ImageContent)
		if !ok {
			t.Fatalf("expected ImageContent for SVG, got %T", result.Content[0])
		}
		if img.MIMEType != "image/svg+xml" {
			t.Errorf("expected image/svg+xml, got %s", img.MIMEType)
		}
	})

	t.Run("misnamed image still detected", func(t *testing.T) {
		file := filepath.Join(tmp, "photo.dat")
		// PNG magic bytes with wrong extension
		data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		data = append(data, make([]byte, 100)...)
		os.WriteFile(file, data, 0644)

		result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
		if err != nil {
			t.Fatal(err)
		}
		_, ok := result.Content[0].(*mcp.ImageContent)
		if !ok {
			t.Fatalf("expected ImageContent for misnamed PNG, got %T", result.Content[0])
		}
	})

	t.Run("unrecognized binary not image", func(t *testing.T) {
		file := filepath.Join(tmp, "program.wasm")
		// Random binary data that's not an image
		data := []byte{0x00, 0x61, 0x73, 0x6D} // wasm magic
		data = append(data, make([]byte, 100)...)
		os.WriteFile(file, data, 0644)

		result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		if !strings.Contains(text, "Binary file") {
			t.Errorf("expected 'Binary file' for non-image binary, got: %s", text)
		}
	})
}

func TestViewDirectoryListing(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "src", "pkg"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "main.go"), []byte("m"), 0644)
	os.MkdirAll(filepath.Join(tmp, ".git"), 0755)
	os.MkdirAll(filepath.Join(tmp, "node_modules"), 0755)
	os.MkdirAll(filepath.Join(tmp, ".github"), 0755)
	os.WriteFile(filepath.Join(tmp, ".env"), []byte("e"), 0644)
	os.WriteFile(filepath.Join(tmp, ".dockerignore"), []byte("d"), 0644)

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
	if strings.Contains(text, ".git") && !strings.Contains(text, ".github") {
		// .git should be excluded but .github should be present
	}
	// .git should be excluded
	if strings.Contains(text, ".git/") {
		t.Error("expected .git/ to be excluded")
	}
	if strings.Contains(text, "node_modules") {
		t.Error("expected node_modules to be excluded")
	}
	// Dotfiles SHOULD now be visible (except .git and node_modules)
	if !strings.Contains(text, ".github/") {
		t.Error("expected .github/ to be visible")
	}
	if !strings.Contains(text, ".env") {
		t.Error("expected .env to be visible")
	}
	if !strings.Contains(text, ".dockerignore") {
		t.Error("expected .dockerignore to be visible")
	}
}

func TestViewDirectorySymlinks(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target.txt")
	os.WriteFile(target, []byte("content"), 0644)
	link := filepath.Join(tmp, "link.txt")
	os.Symlink(target, link)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: tmp})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "link.txt -> ") {
		t.Errorf("expected symlink indication, got: %s", text)
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

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: "/etc/hostname"})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("expected IsError for path scoping violation")
	}
	if !hasErrorCode(result, ErrAccessDenied) {
		t.Errorf("expected error code %s, got: %s", ErrAccessDenied, resultText(result))
	}
}

func TestViewFileNotFound(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: filepath.Join(tmp, "nonexistent")})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("expected IsError for nonexistent file")
	}
	if !hasErrorCode(result, ErrPathNotFound) {
		t.Errorf("expected error code %s, got: %s", ErrPathNotFound, resultText(result))
	}
}

func TestViewMaxFileSize(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "big.txt")
	os.WriteFile(file, make([]byte, 1024), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := viewHandler(sess, resolver, 100) // 100 byte limit

	result, _, err := handler(context.Background(), nil, ViewArgs{Path: file})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("expected IsError for file exceeding max size")
	}
	if !hasErrorCode(result, ErrFileTooLarge) {
		t.Errorf("expected error code %s, got: %s", ErrFileTooLarge, resultText(result))
	}
}
