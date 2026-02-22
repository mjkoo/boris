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

func TestCreateFileNew(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "new.txt")

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := createFileHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, CreateFileArgs{
		Path:    file,
		Content: "hello world\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "Created") {
		t.Errorf("expected confirmation, got: %s", text)
	}

	data, _ := os.ReadFile(file)
	if string(data) != "hello world\n" {
		t.Errorf("got %q, want %q", data, "hello world\n")
	}
}

func TestCreateFileOverwriteByDefault(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "existing.txt")
	os.WriteFile(file, []byte("original"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := createFileHandler(sess, resolver, 10*1024*1024)

	// Should overwrite without needing an explicit flag
	result, _, err := handler(context.Background(), nil, CreateFileArgs{
		Path:    file,
		Content: "new content",
	})
	if err != nil {
		t.Fatal(err)
	}
	if isErrorResult(result) {
		t.Errorf("overwrite should succeed by default, got error: %s", resultText(result))
	}

	data, _ := os.ReadFile(file)
	if string(data) != "new content" {
		t.Errorf("got %q, want %q", data, "new content")
	}
}

func TestCreateFileParentDirs(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "a", "b", "c", "file.txt")

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := createFileHandler(sess, resolver, 10*1024*1024)

	_, _, err := handler(context.Background(), nil, CreateFileArgs{
		Path:    file,
		Content: "nested",
	})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(file)
	if string(data) != "nested" {
		t.Errorf("got %q, want %q", data, "nested")
	}
}

func TestCreateFilePermissions(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := createFileHandler(sess, resolver, 10*1024*1024)

	_, _, err := handler(context.Background(), nil, CreateFileArgs{
		Path:    file,
		Content: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(file)
	if info.Mode().Perm() != 0644 {
		t.Errorf("expected mode 0644, got %o", info.Mode().Perm())
	}
}

func TestCreateFileMaxSize(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "big.txt")

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := createFileHandler(sess, resolver, 100)

	result, _, err := handler(context.Background(), nil, CreateFileArgs{
		Path:    file,
		Content: strings.Repeat("x", 200),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("expected IsError for exceeding max file size")
	}
	if !strings.Contains(resultText(result), "exceeds maximum") {
		t.Errorf("expected size error, got: %s", resultText(result))
	}
}

func TestCreateFilePathScoping(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver([]string{tmp}, nil)
	handler := createFileHandler(sess, resolver, 10*1024*1024)

	result, _, err := handler(context.Background(), nil, CreateFileArgs{
		Path:    "/etc/evil.txt",
		Content: "hacked",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("expected IsError for path scoping violation")
	}
}
