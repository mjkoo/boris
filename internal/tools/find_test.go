package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// findTestSetup creates a temp directory with a session and resolver.
func findTestSetup(t *testing.T) (string, *session.Session, *pathscope.Resolver) {
	t.Helper()
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, err := pathscope.NewResolver(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tmp, sess, resolver
}

func callFind(sess *session.Session, resolver *pathscope.Resolver, args FindArgs) (*mcp.CallToolResult, error) {
	handler := findHandler(sess, resolver)
	r, _, err := handler(context.Background(), nil, args)
	return r, err
}

func callFindCompat(sess *session.Session, resolver *pathscope.Resolver, args FindCompatArgs) (*mcp.CallToolResult, error) {
	handler := findCompatHandler(sess, resolver)
	r, _, err := handler(context.Background(), nil, args)
	return r, err
}

// --- 3.1: Simple extension pattern ---

func TestFindSimpleExtensionPattern(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main"), 0644)
	os.MkdirAll(filepath.Join(tmp, "internal", "tools"), 0755)
	os.WriteFile(filepath.Join(tmp, "internal", "tools", "grep.go"), []byte("package tools"), 0644)
	os.WriteFile(filepath.Join(tmp, "README.md"), []byte("# readme"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "main.go") {
		t.Errorf("expected main.go in results, got: %s", text)
	}
	if !strings.Contains(text, filepath.Join("internal", "tools", "grep.go")) {
		t.Errorf("expected internal/tools/grep.go in results, got: %s", text)
	}
	if strings.Contains(text, "README.md") {
		t.Errorf("README.md should not match *.go, got: %s", text)
	}
}

// --- 3.2: Recursive doublestar pattern ---

func TestFindRecursiveDoublestarPattern(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.MkdirAll(filepath.Join(tmp, "src", "utils"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "app.test.ts"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmp, "src", "utils", "helper.test.ts"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmp, "src", "app.ts"), []byte("code"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "**/*.test.ts"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "app.test.ts") {
		t.Errorf("expected app.test.ts, got: %s", text)
	}
	if !strings.Contains(text, filepath.Join("utils", "helper.test.ts")) {
		t.Errorf("expected utils/helper.test.ts, got: %s", text)
	}
	if strings.Contains(text, "app.ts\n") || strings.HasSuffix(text, "app.ts") {
		// Make sure app.ts without .test. is not matched
		// But be careful — app.test.ts contains "app.ts" as a substring
		lines := strings.Split(strings.TrimSpace(text), "\n")
		for _, line := range lines {
			base := filepath.Base(line)
			if base == "app.ts" {
				t.Errorf("app.ts should not match **/*.test.ts, got: %s", text)
			}
		}
	}
}

// --- 3.3: Directory-scoped search via path parameter ---

func TestFindDirectoryScopedSearch(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "internal", "tools"), 0755)
	os.MkdirAll(filepath.Join(tmp, "cmd"), 0755)
	os.WriteFile(filepath.Join(tmp, "internal", "tools", "grep.go"), []byte("package tools"), 0644)
	os.WriteFile(filepath.Join(tmp, "cmd", "main.go"), []byte("package main"), 0644)

	r, err := callFind(sess, resolver, FindArgs{
		Pattern: "*.go",
		Path:    "internal/tools",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "grep.go") {
		t.Errorf("expected grep.go in results, got: %s", text)
	}
	if strings.Contains(text, "main.go") {
		t.Errorf("main.go should not be in results (outside path), got: %s", text)
	}
}

// --- 3.4: Relative directory path in pattern ---

func TestFindRelativeDirectoryInPattern(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.MkdirAll(filepath.Join(tmp, "docs"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "README.md"), []byte("source"), 0644)
	os.WriteFile(filepath.Join(tmp, "docs", "README.md"), []byte("docs"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "src/**/*.md"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, filepath.Join("src", "README.md")) {
		t.Errorf("expected src/README.md, got: %s", text)
	}
	if strings.Contains(text, filepath.Join("docs", "README.md")) {
		t.Errorf("docs/README.md should not match src/**/*.md, got: %s", text)
	}
}

// --- 3.5: Brace expansion and character class ---

func TestFindBraceExpansion(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "app.ts"), []byte("code"), 0644)
	os.WriteFile(filepath.Join(tmp, "comp.tsx"), []byte("code"), 0644)
	os.WriteFile(filepath.Join(tmp, "style.css"), []byte("code"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.{ts,tsx}"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "app.ts") {
		t.Errorf("expected app.ts, got: %s", text)
	}
	if !strings.Contains(text, "comp.tsx") {
		t.Errorf("expected comp.tsx, got: %s", text)
	}
	if strings.Contains(text, "style.css") {
		t.Errorf("style.css should not match, got: %s", text)
	}
}

func TestFindCharacterClass(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "Makefile"), []byte("all:"), 0644)
	os.WriteFile(filepath.Join(tmp, "makefile"), []byte("all:"), 0644)
	os.WriteFile(filepath.Join(tmp, "notmatch"), []byte("x"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "[Mm]akefile"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "Makefile") {
		t.Errorf("expected Makefile, got: %s", text)
	}
	if !strings.Contains(text, "makefile") {
		t.Errorf("expected makefile, got: %s", text)
	}
	if strings.Contains(text, "notmatch") {
		t.Errorf("notmatch should not match, got: %s", text)
	}
}

// --- 3.6: Mtime sorting ---

func TestFindMtimeSorting(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "old.go"), []byte("old"), 0644)
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(filepath.Join(tmp, "old.go"), oldTime, oldTime)

	os.WriteFile(filepath.Join(tmp, "new.go"), []byte("new"), 0644)
	// new.go has current mtime (newer)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 results, got %d: %s", len(lines), text)
	}
	if lines[0] != "new.go" {
		t.Errorf("expected new.go first (newest), got: %s", lines[0])
	}
	if lines[1] != "old.go" {
		t.Errorf("expected old.go second (oldest), got: %s", lines[1])
	}
}

// --- 3.7: Relative paths ---

func TestFindRelativePaths(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "sub"), 0755)
	os.WriteFile(filepath.Join(tmp, "sub", "file.go"), []byte("package sub"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, tmp) {
		t.Errorf("result should contain relative paths, not absolute: %s", text)
	}
	if !strings.Contains(text, filepath.Join("sub", "file.go")) {
		t.Errorf("expected relative path sub/file.go, got: %s", text)
	}
}

// --- 3.8: No files found for zero matches ---

func TestFindNoFilesFoundZeroMatches(t *testing.T) {
	_, sess, resolver := findTestSetup(t)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.xyz"})
	if err != nil {
		t.Fatal(err)
	}
	if isErrorResult(r) {
		t.Error("zero matches should not be an error")
	}
	text := resultText(r)
	if text != "No files found" {
		t.Errorf("expected 'No files found', got: %s", text)
	}
}

// --- 3.9: No files found for non-existent path ---

func TestFindNoFilesFoundNonExistentPath(t *testing.T) {
	_, sess, resolver := findTestSetup(t)

	r, err := callFind(sess, resolver, FindArgs{
		Pattern: "*.go",
		Path:    "nonexistent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if isErrorResult(r) {
		t.Error("non-existent path should not be an error")
	}
	text := resultText(r)
	if text != "No files found" {
		t.Errorf("expected 'No files found', got: %s", text)
	}
}

// --- 3.10: Output truncation ---

func TestFindOutputTruncation(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	// Create many files with long names to exceed 30k chars
	os.MkdirAll(filepath.Join(tmp, "deep"), 0755)
	for i := 0; i < 1500; i++ {
		name := fmt.Sprintf("%s_%05d.txt", strings.Repeat("a", 25), i)
		os.WriteFile(filepath.Join(tmp, "deep", name), []byte("x"), 0644)
	}

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "truncated") {
		t.Errorf("expected truncation message, got length %d", len(text))
	}
	// Verify truncation happens at a complete line, not mid-path
	lastNewline := strings.LastIndex(text, "\n")
	lineBeforeTrunc := text[:lastNewline]
	lastLine := lineBeforeTrunc[strings.LastIndex(lineBeforeTrunc, "\n")+1:]
	if !strings.HasSuffix(lastLine, ".txt") {
		t.Errorf("truncation should happen at complete path boundary, last line: %s", lastLine)
	}
}

// --- 4.1: Directory symlink NOT followed ---

func TestFindDirectorySymlinkNotFollowed(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "real"), 0755)
	os.WriteFile(filepath.Join(tmp, "real", "inside.go"), []byte("package real"), 0644)
	os.Symlink(filepath.Join(tmp, "real"), filepath.Join(tmp, "linked_dir"))

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, filepath.Join("real", "inside.go")) {
		t.Errorf("expected real/inside.go, got: %s", text)
	}
	// Files inside the symlinked dir should NOT appear
	if strings.Contains(text, filepath.Join("linked_dir", "inside.go")) {
		t.Errorf("directory symlink should not be followed, got: %s", text)
	}
}

func TestFindDirectorySymlinkItselfNotReturned(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "real"), 0755)
	os.WriteFile(filepath.Join(tmp, "real", "file.txt"), []byte("content"), 0644)
	os.Symlink(filepath.Join(tmp, "real"), filepath.Join(tmp, "linked_dir"))

	// Even with **/* pattern, the directory symlink itself should not appear
	r, err := callFind(sess, resolver, FindArgs{Pattern: "**/*"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, "linked_dir") {
		t.Errorf("directory symlink itself should not appear in results, got: %s", text)
	}

	// Also verify with type=directory filter
	r, err = callFind(sess, resolver, FindArgs{Pattern: "**/*", Type: "directory"})
	if err != nil {
		t.Fatal(err)
	}
	text = resultText(r)
	if strings.Contains(text, "linked_dir") {
		t.Errorf("directory symlink should not appear even with type=directory, got: %s", text)
	}
}

// --- 4.2: File symlink IS included ---

func TestFindFileSymlinkIncluded(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "real.go"), []byte("package main"), 0644)
	os.Symlink(filepath.Join(tmp, "real.go"), filepath.Join(tmp, "link.go"))

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "link.go") {
		t.Errorf("file symlink should be included, got: %s", text)
	}
	if !strings.Contains(text, "real.go") {
		t.Errorf("real file should be included, got: %s", text)
	}
}

// --- 4.3: Broken symlink silently skipped ---

func TestFindBrokenSymlinkSkipped(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "valid.go"), []byte("package main"), 0644)
	os.Symlink(filepath.Join(tmp, "nonexistent"), filepath.Join(tmp, "broken.go"))

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "valid.go") {
		t.Errorf("valid file should be found, got: %s", text)
	}
	if strings.Contains(text, "broken.go") {
		t.Errorf("broken symlink should be skipped, got: %s", text)
	}
	if isErrorResult(r) {
		t.Error("broken symlink should not cause an error")
	}
}

// --- 5.1: .git directory skipped ---

func TestFindGitDirSkipped(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, ".git"), 0755)
	os.WriteFile(filepath.Join(tmp, ".git", "HEAD"), []byte("ref"), 0644)
	os.WriteFile(filepath.Join(tmp, "src.go"), []byte("package main"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "**/*"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, ".git") {
		t.Errorf(".git should be skipped, got: %s", text)
	}
	if !strings.Contains(text, "src.go") {
		t.Errorf("src.go should be found, got: %s", text)
	}
}

// --- 5.2: node_modules directory skipped ---

func TestFindNodeModulesSkipped(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "node_modules"), 0755)
	os.WriteFile(filepath.Join(tmp, "node_modules", "pkg.js"), []byte("module"), 0644)
	os.WriteFile(filepath.Join(tmp, "app.js"), []byte("app"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.js"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, "node_modules") {
		t.Errorf("node_modules should be skipped, got: %s", text)
	}
	if !strings.Contains(text, "app.js") {
		t.Errorf("app.js should be found, got: %s", text)
	}
}

// --- 5.3: Gitignore patterns respected ---

func TestFindGitignoreRespected(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("*.log\ndist/\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "dist"), 0755)
	os.WriteFile(filepath.Join(tmp, "dist", "bundle.js"), []byte("bundled"), 0644)
	os.WriteFile(filepath.Join(tmp, "app.log"), []byte("log"), 0644)
	os.WriteFile(filepath.Join(tmp, "src.go"), []byte("package main"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "**/*"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, "dist") {
		t.Errorf("dist/ should be ignored, got: %s", text)
	}
	if strings.Contains(text, "app.log") {
		t.Errorf("*.log should be ignored, got: %s", text)
	}
	if !strings.Contains(text, "src.go") {
		t.Errorf("src.go should be found, got: %s", text)
	}
}

// --- 5.4: Negated gitignore pattern ---

func TestFindGitignoreNegation(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("*.log\n!important.log\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "app.log"), []byte("log"), 0644)
	os.WriteFile(filepath.Join(tmp, "important.log"), []byte("important"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "*.log"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "important.log") {
		t.Errorf("important.log should be included (negation), got: %s", text)
	}
	if strings.Contains(text, "app.log") {
		t.Errorf("app.log should be excluded, got: %s", text)
	}
}

// --- 5.5: Hidden files included ---

func TestFindHiddenFilesIncluded(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, ".github", "workflows"), 0755)
	os.WriteFile(filepath.Join(tmp, ".github", "workflows", "ci.yml"), []byte("on: push"), 0644)
	os.WriteFile(filepath.Join(tmp, ".dockerignore"), []byte("node_modules"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "**/*"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "ci.yml") {
		t.Errorf("expected .github/workflows/ci.yml, got: %s", text)
	}
	if !strings.Contains(text, ".dockerignore") {
		t.Errorf("expected .dockerignore, got: %s", text)
	}
}

// --- 5.6: Path scoping: denied paths silently excluded ---

func TestFindDeniedPathsExcluded(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, err := pathscope.NewResolver([]string{tmp}, []string{"**/.env"})
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(tmp, ".env"), []byte("SECRET=val"), 0644)
	os.WriteFile(filepath.Join(tmp, "src.go"), []byte("package main"), 0644)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "**/*"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, ".env") {
		t.Errorf(".env should be denied, got: %s", text)
	}
	if !strings.Contains(text, "src.go") {
		t.Errorf("src.go should be found, got: %s", text)
	}
}

// --- 5.7: Path scoping: search root outside allowed dirs ---

func TestFindSearchRootOutsideAllowedDirs(t *testing.T) {
	tmp := t.TempDir()
	allowed := t.TempDir()
	sess := session.New(tmp)
	resolver, err := pathscope.NewResolver([]string{allowed}, nil)
	if err != nil {
		t.Fatal(err)
	}

	r, err := callFind(sess, resolver, FindArgs{
		Pattern: "*.go",
		Path:    tmp, // outside allow list
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(r) {
		t.Error("expected error for path outside allow list")
	}
	if !hasErrorCode(r, ErrAccessDenied) {
		t.Errorf("expected error code %s, got: %s", ErrAccessDenied, resultText(r))
	}
}

// --- 6.1: Empty pattern returns IsError ---

func TestFindEmptyPatternError(t *testing.T) {
	_, sess, resolver := findTestSetup(t)

	r, err := callFind(sess, resolver, FindArgs{Pattern: ""})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(r) {
		t.Error("expected error for empty pattern")
	}
	if !hasErrorCode(r, ErrInvalidInput) {
		t.Errorf("expected error code %s, got: %s", ErrInvalidInput, resultText(r))
	}
}

// --- 6.2: Malformed pattern returns IsError ---

func TestFindMalformedPatternError(t *testing.T) {
	_, sess, resolver := findTestSetup(t)

	r, err := callFind(sess, resolver, FindArgs{Pattern: "[invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(r) {
		t.Error("expected error for malformed pattern")
	}
	if !hasErrorCode(r, ErrFindInvalidPattern) {
		t.Errorf("expected error code %s, got: %s", ErrFindInvalidPattern, resultText(r))
	}
}

// --- 6.3: Type filter "file" returns only files ---

func TestFindTypeFilterFile(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmp, "file.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmp, "subdir", "nested.go"), []byte("package sub"), 0644)

	r, err := callFind(sess, resolver, FindArgs{
		Pattern: "**/*",
		Type:    "file",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "file.go") {
		t.Errorf("expected file.go, got: %s", text)
	}
	if !strings.Contains(text, filepath.Join("subdir", "nested.go")) {
		t.Errorf("expected subdir/nested.go, got: %s", text)
	}
	// "subdir" should not appear as a standalone result
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for _, line := range lines {
		if line == "subdir" {
			t.Errorf("directories should not appear with type=file, got: %s", text)
		}
	}
}

// --- 6.4: Type filter "directory" returns only directories ---

func TestFindTypeFilterDirectory(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "subdir"), 0755)
	os.MkdirAll(filepath.Join(tmp, "another"), 0755)
	os.WriteFile(filepath.Join(tmp, "file.go"), []byte("package main"), 0644)

	r, err := callFind(sess, resolver, FindArgs{
		Pattern: "**/*",
		Type:    "directory",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "subdir") {
		t.Errorf("expected subdir, got: %s", text)
	}
	if !strings.Contains(text, "another") {
		t.Errorf("expected another, got: %s", text)
	}
	if strings.Contains(text, "file.go") {
		t.Errorf("files should not appear with type=directory, got: %s", text)
	}
}

// --- 6.5: Invalid type value returns IsError ---

func TestFindInvalidTypeError(t *testing.T) {
	_, sess, resolver := findTestSetup(t)

	r, err := callFind(sess, resolver, FindArgs{
		Pattern: "*.go",
		Type:    "symlink",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(r) {
		t.Error("expected error for invalid type")
	}
	if !hasErrorCode(r, ErrFindInvalidType) {
		t.Errorf("expected error code %s, got: %s", ErrFindInvalidType, resultText(r))
	}
}

// --- Compat handler unit test ---

func TestFindCompatHandlerProducesSameResults(t *testing.T) {
	tmp, sess, resolver := findTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "app.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmp, "readme.md"), []byte("# readme"), 0644)

	normalR, err := callFind(sess, resolver, FindArgs{Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	compatR, err := callFindCompat(sess, resolver, FindCompatArgs{Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}

	normalText := resultText(normalR)
	compatText := resultText(compatR)
	if normalText != compatText {
		t.Errorf("normal and compat should produce identical results\nnormal: %s\ncompat: %s", normalText, compatText)
	}
}

// --- 7.1: Integration test: find in default mode tool list ---

func TestIntegrationFindInDefaultToolList(t *testing.T) {
	tmp := t.TempDir()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "boris-test",
		Version: "test",
	}, nil)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver([]string{tmp}, nil)

	RegisterAll(server, resolver, sess, Config{
		MaxFileSize:    10 * 1024 * 1024,
		DefaultTimeout: 30,
		Shell:          "/bin/sh",
	})

	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	toolList, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	toolNames := make(map[string]bool)
	for _, tool := range toolList.Tools {
		toolNames[tool.Name] = true
	}
	if !toolNames["find"] {
		t.Error("find tool should be in default mode tool list")
	}
	if toolNames["Glob"] {
		t.Error("Glob tool should NOT be in default mode tool list")
	}
}

// --- 7.2: Integration test: Glob in compat mode ---

func TestIntegrationGlobInCompatToolList(t *testing.T) {
	tmp := t.TempDir()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "boris-test",
		Version: "test",
	}, nil)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver([]string{tmp}, nil)

	RegisterAll(server, resolver, sess, Config{
		MaxFileSize:     10 * 1024 * 1024,
		DefaultTimeout:  30,
		Shell:           "/bin/sh",
		AnthropicCompat: true,
	})

	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	toolList, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	toolNames := make(map[string]bool)
	for _, tool := range toolList.Tools {
		toolNames[tool.Name] = true
	}
	if !toolNames["Glob"] {
		t.Error("Glob tool should be in compat mode tool list")
	}
	if toolNames["find"] {
		t.Error("find tool should NOT be in compat mode tool list")
	}
}

// --- 7.3: Integration test: Glob schema has only pattern and path ---

func TestIntegrationGlobSchemaNoType(t *testing.T) {
	tmp := t.TempDir()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "boris-test",
		Version: "test",
	}, nil)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver([]string{tmp}, nil)

	RegisterAll(server, resolver, sess, Config{
		MaxFileSize:     10 * 1024 * 1024,
		DefaultTimeout:  30,
		Shell:           "/bin/sh",
		AnthropicCompat: true,
	})

	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	toolList, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, tool := range toolList.Tools {
		if tool.Name == "Glob" {
			schemaMap, ok := tool.InputSchema.(map[string]interface{})
			if !ok {
				t.Fatal("Glob tool should have input schema map")
			}
			props, ok := schemaMap["properties"].(map[string]interface{})
			if !ok {
				t.Fatal("expected properties in Glob schema")
			}
			if _, ok := props["pattern"]; !ok {
				t.Error("Glob schema should have 'pattern' parameter")
			}
			if _, ok := props["path"]; !ok {
				t.Error("Glob schema should have 'path' parameter")
			}
			if _, ok := props["type"]; ok {
				t.Error("Glob schema should NOT have 'type' parameter in compat mode")
			}
			return
		}
	}
	t.Error("Glob tool not found in tool list")
}

// --- 7.4: Update existing integration tests ---
// The existing integration tests in integration_test.go need updating.
// These tests verify the new tool appears in tool lists.
// Tests for exact tool list contents are handled by TestIntegrationFindInDefaultToolList
// and TestIntegrationGlobInCompatToolList above.
