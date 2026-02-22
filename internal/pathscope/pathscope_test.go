package pathscope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoAllowDirs(t *testing.T) {
	r, err := NewResolver(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := r.Resolve("/", "/etc/hostname")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "/etc/hostname" {
		t.Errorf("got %q, want /etc/hostname", resolved)
	}
}

func TestSingleAllowDir(t *testing.T) {
	tmp := t.TempDir()
	r, err := NewResolver([]string{tmp}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Path inside allowed dir should succeed
	testFile := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(testFile, []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	resolved, err := r.Resolve("/", testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != testFile {
		t.Errorf("got %q, want %q", resolved, testFile)
	}

	// Path outside allowed dir should fail
	_, err = r.Resolve("/", "/etc/hostname")
	if err == nil {
		t.Error("expected error for path outside allowed dirs")
	}
	if !strings.Contains(err.Error(), "outside allowed directories") {
		t.Errorf("error message should mention 'outside allowed directories': %v", err)
	}
}

func TestMultipleAllowDirs(t *testing.T) {
	tmp1 := t.TempDir()
	tmp2 := t.TempDir()
	r, err := NewResolver([]string{tmp1, tmp2}, nil)
	if err != nil {
		t.Fatal(err)
	}

	f1 := filepath.Join(tmp1, "a.txt")
	f2 := filepath.Join(tmp2, "b.txt")
	os.WriteFile(f1, []byte("a"), 0644)
	os.WriteFile(f2, []byte("b"), 0644)

	if _, err := r.Resolve("/", f1); err != nil {
		t.Errorf("f1 should be allowed: %v", err)
	}
	if _, err := r.Resolve("/", f2); err != nil {
		t.Errorf("f2 should be allowed: %v", err)
	}
	if _, err := r.Resolve("/", "/etc/passwd"); err == nil {
		t.Error("path outside both dirs should be denied")
	}
}

func TestDenyOverridesAllow(t *testing.T) {
	tmp := t.TempDir()
	envFile := filepath.Join(tmp, ".env")
	os.WriteFile(envFile, []byte("SECRET=x"), 0644)

	r, err := NewResolver([]string{tmp}, []string{"**/.env"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Resolve("/", envFile)
	if err == nil {
		t.Error("expected deny to override allow")
	}
	if !strings.Contains(err.Error(), "deny pattern") {
		t.Errorf("error should mention deny pattern: %v", err)
	}
}

func TestDenyDoublestarGlob(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b", ".secret")
	os.MkdirAll(filepath.Dir(nested), 0755)
	os.WriteFile(nested, []byte("s"), 0644)

	r, err := NewResolver(nil, []string{"**/.secret"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Resolve("/", nested)
	if err == nil {
		t.Error("expected deny for doublestar glob match")
	}
}

func TestDenySimpleGlob(t *testing.T) {
	tmp := t.TempDir()
	tmpFile := filepath.Join(tmp, "data.tmp")
	os.WriteFile(tmpFile, []byte("t"), 0644)

	r, err := NewResolver(nil, []string{tmp + "/*.tmp"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Resolve("/", tmpFile)
	if err == nil {
		t.Error("expected deny for simple glob match")
	}
}

func TestDenyDirectoryMatchesChildren(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	gitConfig := filepath.Join(gitDir, "config")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(gitConfig, []byte("c"), 0644)

	r, err := NewResolver(nil, []string{"**/.git"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Resolve("/", gitConfig)
	if err == nil {
		t.Error("expected deny for file under matching directory")
	}
}

func TestSymlinkResolution(t *testing.T) {
	tmp := t.TempDir()
	realFile := filepath.Join(tmp, "real.txt")
	os.WriteFile(realFile, []byte("r"), 0644)

	linkFile := filepath.Join(tmp, "link.txt")
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Skip("symlinks not supported")
	}

	r, err := NewResolver([]string{tmp}, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := r.Resolve("/", linkFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != realFile {
		t.Errorf("got %q, want %q (resolved symlink)", resolved, realFile)
	}
}

func TestSymlinkEscape(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	os.WriteFile(outsideFile, []byte("s"), 0644)

	// Create symlink inside allowed dir pointing outside
	link := filepath.Join(allowed, "escape")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skip("symlinks not supported")
	}

	r, err := NewResolver([]string{allowed}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Resolve("/", link)
	if err == nil {
		t.Error("expected deny for symlink escaping allowed directory")
	}
}

func TestRelativePathResolution(t *testing.T) {
	tmp := t.TempDir()
	subDir := filepath.Join(tmp, "sub")
	os.MkdirAll(subDir, 0755)
	testFile := filepath.Join(subDir, "file.txt")
	os.WriteFile(testFile, []byte("f"), 0644)

	r, err := NewResolver(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := r.Resolve(subDir, "file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != testFile {
		t.Errorf("got %q, want %q", resolved, testFile)
	}
}

func TestClearErrorMessages(t *testing.T) {
	tmp := t.TempDir()
	r, err := NewResolver([]string{tmp}, []string{"**/.env"})
	if err != nil {
		t.Fatal(err)
	}

	// Outside allow
	_, err = r.Resolve("/", "/etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
		t.Errorf("expected 'outside allowed directories' error, got: %v", err)
	}

	// Deny match
	envFile := filepath.Join(tmp, ".env")
	os.WriteFile(envFile, []byte("x"), 0644)
	_, err = r.Resolve("/", envFile)
	if err == nil || !strings.Contains(err.Error(), "deny pattern") {
		t.Errorf("expected 'deny pattern' error, got: %v", err)
	}
}
