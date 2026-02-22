package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// grepTestSetup creates a temp directory with test files and returns the session, resolver, and cleanup.
func grepTestSetup(t *testing.T) (string, *session.Session, *pathscope.Resolver) {
	t.Helper()
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, err := pathscope.NewResolver(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tmp, sess, resolver
}

func callGrep(sess *session.Session, resolver *pathscope.Resolver, args GrepArgs) (*mcp.CallToolResult, error) {
	handler := grepHandler(sess, resolver)
	r, _, err := handler(context.Background(), nil, args)
	return r, err
}

func callGrepCompat(sess *session.Session, resolver *pathscope.Resolver, args GrepCompatArgs) (*mcp.CallToolResult, error) {
	handler := grepCompatHandler(sess, resolver)
	r, _, err := handler(context.Background(), nil, args)
	return r, err
}

// --- 3.1: Basic search tests ---

func TestGrepLiteralMatch(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("hello world\nfoo bar\nbaz\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "foo",
		Path:       "test.txt",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "foo bar") {
		t.Errorf("expected match for 'foo', got: %s", text)
	}
}

func TestGrepRegexMatch(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.go"), []byte("func main() {\n\tfmt.Println(\"hello\")\n}\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    `func\s+\w+\(`,
		Path:       "test.go",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "func main()") {
		t.Errorf("expected regex match, got: %s", text)
	}
}

func TestGrepInvalidRegex(t *testing.T) {
	_, sess, resolver := grepTestSetup(t)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "[invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(r) {
		t.Error("expected error for invalid regex")
	}
	if !hasErrorCode(r, ErrGrepInvalidPattern) {
		t.Errorf("expected error code %s, got: %s", ErrGrepInvalidPattern, resultText(r))
	}
}

func TestGrepEmptyPattern(t *testing.T) {
	_, sess, resolver := grepTestSetup(t)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "",
	})
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

// --- 3.2: Output mode tests ---

func TestGrepContentModeWithLineNumbers(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("line one\nfoo here\nline three\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "foo",
		Path:       "test.txt",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt:2:foo here") {
		t.Errorf("expected content with line numbers, got: %s", text)
	}
}

func TestGrepFilesWithMatchesMode(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("match here\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("no matches\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match here",
		OutputMode: "files_with_matches",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "a.txt") {
		t.Errorf("expected a.txt in results, got: %s", text)
	}
	if strings.Contains(text, "b.txt") {
		t.Errorf("b.txt should not be in results, got: %s", text)
	}
}

func TestGrepFilesWithMatchesMtimeSorted(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "old.txt"), []byte("match\n"), 0644)
	// Set old.txt to an old mtime
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(filepath.Join(tmp, "old.txt"), oldTime, oldTime)

	os.WriteFile(filepath.Join(tmp, "new.txt"), []byte("match\n"), 0644)
	// new.txt has current mtime (newer)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		OutputMode: "files_with_matches",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 files, got %d: %s", len(lines), text)
	}
	// Newest first
	if lines[0] != "new.txt" {
		t.Errorf("expected new.txt first (newest), got: %s", lines[0])
	}
	if lines[1] != "old.txt" {
		t.Errorf("expected old.txt second (oldest), got: %s", lines[1])
	}
}

func TestGrepCountMode(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("foo\nbar\nfoo\nbaz\nfoo\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "foo",
		Path:       "test.txt",
		OutputMode: "count",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if text != "test.txt:3" {
		t.Errorf("expected count of 3, got: %s", text)
	}
}

func TestGrepInvalidOutputMode(t *testing.T) {
	_, sess, resolver := grepTestSetup(t)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "foo",
		OutputMode: "summary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(r) {
		t.Error("expected error for invalid output_mode")
	}
	if !hasErrorCode(r, ErrGrepInvalidOutputMode) {
		t.Errorf("expected error code %s, got: %s", ErrGrepInvalidOutputMode, resultText(r))
	}
}

func TestGrepDefaultOutputMode(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	// No output_mode specified — should default to files_with_matches
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt") {
		t.Errorf("expected file path in default output, got: %s", text)
	}
	// Should NOT contain line numbers (that's content mode)
	if strings.Contains(text, ":1:") {
		t.Errorf("default mode should not include line numbers, got: %s", text)
	}
}

// --- 3.3: Context line tests ---

func TestGrepBeforeContext(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "line1\nline2\nline3\nmatch\nline5\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	cb := intPtr(2)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:       "match",
		Path:          "test.txt",
		OutputMode:    "content",
		ContextBefore: cb,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt-2-line2") {
		t.Errorf("expected before context line 2, got: %s", text)
	}
	if !strings.Contains(text, "test.txt-3-line3") {
		t.Errorf("expected before context line 3, got: %s", text)
	}
	if !strings.Contains(text, "test.txt:4:match") {
		t.Errorf("expected match line, got: %s", text)
	}
}

func TestGrepAfterContext(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "line1\nmatch\nline3\nline4\nline5\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	ca := intPtr(2)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:      "match",
		Path:         "test.txt",
		OutputMode:   "content",
		ContextAfter: ca,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt:2:match") {
		t.Errorf("expected match line, got: %s", text)
	}
	if !strings.Contains(text, "test.txt-3-line3") {
		t.Errorf("expected after context line 3, got: %s", text)
	}
	if !strings.Contains(text, "test.txt-4-line4") {
		t.Errorf("expected after context line 4, got: %s", text)
	}
}

func TestGrepContextShorthand(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "line1\nline2\nmatch\nline4\nline5\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	c := intPtr(1)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		Context:    c,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt-2-line2") {
		t.Errorf("expected before context, got: %s", text)
	}
	if !strings.Contains(text, "test.txt:3:match") {
		t.Errorf("expected match line, got: %s", text)
	}
	if !strings.Contains(text, "test.txt-4-line4") {
		t.Errorf("expected after context, got: %s", text)
	}
}

func TestGrepExplicitOverridesShorthand(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "line1\nline2\nline3\nmatch\nline5\nline6\nline7\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	c := intPtr(3)
	cb := intPtr(1)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:       "match",
		Path:          "test.txt",
		OutputMode:    "content",
		Context:       c,
		ContextBefore: cb,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	// context_before=1 overrides context=3 for before
	if strings.Contains(text, "test.txt-2-line2") {
		t.Errorf("should NOT show line2 (context_before=1 overrides context=3), got: %s", text)
	}
	if !strings.Contains(text, "test.txt-3-line3") {
		t.Errorf("expected context_before=1 shows line3, got: %s", text)
	}
	// context_after=3 from context shorthand
	if !strings.Contains(text, "test.txt-5-line5") {
		t.Errorf("expected after context line5, got: %s", text)
	}
	if !strings.Contains(text, "test.txt-7-line7") {
		t.Errorf("expected after context line7, got: %s", text)
	}
}

func TestGrepOverlappingContextMerge(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "line1\nmatch1\nline3\nmatch2\nline5\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	c := intPtr(1)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		Context:    c,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	// Lines 1-5 should be one contiguous block (no --)
	if strings.Contains(text, "--") {
		t.Errorf("overlapping context should merge (no --), got: %s", text)
	}
	// line3 should appear only once
	count := strings.Count(text, "line3")
	if count != 1 {
		t.Errorf("line3 should appear once, appeared %d times in: %s", count, text)
	}
}

func TestGrepContextAtFileBoundary(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "line1\nmatch\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	cb := intPtr(5)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:       "match",
		Path:          "test.txt",
		OutputMode:    "content",
		ContextBefore: cb,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	// Should clamp to start of file
	if !strings.Contains(text, "test.txt-1-line1") {
		t.Errorf("expected clamped context to show line1, got: %s", text)
	}
}

func TestGrepContextIgnoredOutsideContentMode(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	c := intPtr(3)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		OutputMode: "files_with_matches",
		Context:    c,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	// Should only contain file path
	if text != "test.txt" {
		t.Errorf("context should be ignored in files_with_matches mode, got: %s", text)
	}
}

func TestGrepContextLinesSeparator(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "line1\nmatch\nline3\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	c := intPtr(1)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		Context:    c,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	// Context lines should use all-hyphen separators
	if !strings.Contains(text, "test.txt-1-line1") {
		t.Errorf("context line should use - separators, got: %s", text)
	}
	if !strings.Contains(text, "test.txt-3-line3") {
		t.Errorf("context line should use - separators, got: %s", text)
	}
}

// --- 3.4: Separator tests ---

func TestGrepSeparatorBetweenFiles(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("match\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "--") {
		t.Errorf("expected -- separator between files, got: %s", text)
	}
}

func TestGrepSeparatorNonAdjacentSameFile(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "match1\nfiller\nfiller\nfiller\nmatch2\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "--") {
		t.Errorf("expected -- separator between non-adjacent matches, got: %s", text)
	}
}

func TestGrepNoSeparatorAdjacentSameFile(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "match1\nmatch2\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, "--") {
		t.Errorf("should not have -- separator for adjacent matches, got: %s", text)
	}
}

// --- 3.5: File filtering tests ---

func TestGrepIncludeFilter(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.py"), []byte("import os\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "test.js"), []byte("import os\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "test.go"), []byte("import os\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "import",
		Include: "*.py",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.py") {
		t.Errorf("expected test.py in results, got: %s", text)
	}
	if strings.Contains(text, "test.js") {
		t.Errorf("test.js should be excluded, got: %s", text)
	}
	if strings.Contains(text, "test.go") {
		t.Errorf("test.go should be excluded, got: %s", text)
	}
}

func TestGrepIncludeWithBraceExpansion(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "app.ts"), []byte("import x\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "comp.tsx"), []byte("import x\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "style.css"), []byte("import x\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "import",
		Include: "*.{ts,tsx}",
	})
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
		t.Errorf("style.css should be excluded, got: %s", text)
	}
}

func TestGrepIncludeWithPathGlob(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "src", "utils"), 0755)
	os.MkdirAll(filepath.Join(tmp, "tests"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "utils", "helper.py"), []byte("import os\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "tests", "test.py"), []byte("import os\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "import",
		Include: "src/**/*.py",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "helper.py") {
		t.Errorf("expected src/utils/helper.py to match include 'src/**/*.py', got: %s", text)
	}
	if strings.Contains(text, "test.py") {
		t.Errorf("tests/test.py should NOT match include 'src/**/*.py', got: %s", text)
	}
}

func TestGrepTypeFilter(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "app.ts"), []byte("code\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "comp.tsx"), []byte("code\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "helper.mts"), []byte("code\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "style.css"), []byte("code\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "code",
		Type:    "ts",
	})
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
	if !strings.Contains(text, "helper.mts") {
		t.Errorf("expected helper.mts, got: %s", text)
	}
	if strings.Contains(text, "style.css") {
		t.Errorf("style.css should not match ts type, got: %s", text)
	}
}

func TestGrepTypeAndIncludeCombined(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "app.js"), []byte("code\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "lib.mjs"), []byte("code\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "util.cjs"), []byte("code\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "code",
		Type:    "js",
		Include: "*.mjs",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "lib.mjs") {
		t.Errorf("expected lib.mjs (matches both type:js and include:*.mjs), got: %s", text)
	}
	if strings.Contains(text, "app.js") {
		t.Errorf("app.js should not match include:*.mjs, got: %s", text)
	}
}

func TestGrepInvalidType(t *testing.T) {
	_, sess, resolver := grepTestSetup(t)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "code",
		Type:    "brainfuck",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(r) {
		t.Error("expected error for invalid type")
	}
	if !hasErrorCode(r, ErrInvalidInput) {
		t.Errorf("expected error code %s, got: %s", ErrInvalidInput, resultText(r))
	}
}

func TestGrepBinaryFilesSkipped(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	// Create a binary file (starts with PNG header, includes NUL byte)
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	binaryData = append(binaryData, []byte("match should not appear")...)
	os.WriteFile(filepath.Join(tmp, "image.png"), binaryData, 0644)
	os.WriteFile(filepath.Join(tmp, "text.txt"), []byte("match here\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, "image.png") {
		t.Errorf("binary file should be skipped, got: %s", text)
	}
	if !strings.Contains(text, "text.txt") {
		t.Errorf("text file should be found, got: %s", text)
	}
}

func TestGrepBinaryNulByteDetection(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	// A file with a NUL byte in the header should be treated as binary and skipped,
	// even if MIME detection would say "text/plain" or "application/octet-stream".
	data := []byte("match here\x00 and more text")
	os.WriteFile(filepath.Join(tmp, "mixed.dat"), data, 0644)
	os.WriteFile(filepath.Join(tmp, "text.txt"), []byte("match here\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, "mixed.dat") {
		t.Errorf("file with NUL byte should be skipped as binary, got: %s", text)
	}
	if !strings.Contains(text, "text.txt") {
		t.Errorf("text file should be found, got: %s", text)
	}
}

func TestGrepGitAndNodeModulesSkipped(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, ".git"), 0755)
	os.WriteFile(filepath.Join(tmp, ".git", "HEAD"), []byte("match\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "node_modules"), 0755)
	os.WriteFile(filepath.Join(tmp, "node_modules", "pkg.js"), []byte("match\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "src.txt"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, ".git") {
		t.Errorf(".git should be skipped, got: %s", text)
	}
	if strings.Contains(text, "node_modules") {
		t.Errorf("node_modules should be skipped, got: %s", text)
	}
	if !strings.Contains(text, "src.txt") {
		t.Errorf("src.txt should be found, got: %s", text)
	}
}

// --- 3.6: Gitignore tests ---

func TestGrepGitignoreFilesSkipped(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("*.log\ndist/\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "dist"), 0755)
	os.WriteFile(filepath.Join(tmp, "dist", "bundle.js"), []byte("match\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "app.log"), []byte("match\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "src.txt"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
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
	if !strings.Contains(text, "src.txt") {
		t.Errorf("src.txt should be found, got: %s", text)
	}
}

func TestGrepNestedGitignoreOverridesParent(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("*.log\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", ".gitignore"), []byte("!debug.log\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "src", "debug.log"), []byte("match\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "root.log"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	// debug.log should be searched (negation overrides parent)
	if !strings.Contains(text, "debug.log") {
		t.Errorf("src/debug.log should be found (negation overrides parent), got: %s", text)
	}
	// root.log should still be ignored
	if strings.Contains(text, "root.log") {
		t.Errorf("root.log should be ignored, got: %s", text)
	}
}

func TestGrepNoGitignoreSearchesAll(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "app.log"), []byte("match\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "src.txt"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	// Without gitignore, all files should be searched
	if !strings.Contains(text, "app.log") {
		t.Errorf("app.log should be found (no gitignore), got: %s", text)
	}
	if !strings.Contains(text, "src.txt") {
		t.Errorf("src.txt should be found, got: %s", text)
	}
}

// --- 3.7: Symlink tests ---

func TestGrepSymlinkedDirectorySearched(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	// Create a real directory with a file
	os.MkdirAll(filepath.Join(tmp, "real"), 0755)
	os.WriteFile(filepath.Join(tmp, "real", "file.txt"), []byte("match\n"), 0644)
	// Create a symlink to the directory
	os.Symlink(filepath.Join(tmp, "real"), filepath.Join(tmp, "link"))

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	// Should find match in both real and linked paths
	if !strings.Contains(text, "file.txt") {
		t.Errorf("expected file.txt in symlinked dir, got: %s", text)
	}
}

func TestGrepCircularSymlinkDetected(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "a"), 0755)
	os.WriteFile(filepath.Join(tmp, "a", "file.txt"), []byte("match\n"), 0644)
	// Create circular symlink: a/b -> ../ (points back to tmp)
	os.Symlink(tmp, filepath.Join(tmp, "a", "b"))

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should not hang and should find the file
	text := resultText(r)
	if !strings.Contains(text, "file.txt") {
		t.Errorf("should find file.txt despite circular symlink, got: %s", text)
	}
}

func TestGrepSymlinkedFileSearched(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "real.txt"), []byte("match\n"), 0644)
	os.Symlink(filepath.Join(tmp, "real.txt"), filepath.Join(tmp, "link.txt"))

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "link.txt",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "match") {
		t.Errorf("should search symlinked file, got: %s", text)
	}
}

// --- 3.8: Path scoping tests ---

func TestGrepSearchRootOutsideAllowList(t *testing.T) {
	tmp := t.TempDir()
	allowed := t.TempDir()
	sess := session.New(tmp)
	resolver, err := pathscope.NewResolver([]string{allowed}, nil)
	if err != nil {
		t.Fatal(err)
	}

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "anything",
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

func TestGrepDeniedFilesSkippedDuringTraversal(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, err := pathscope.NewResolver([]string{tmp}, []string{"**/.env"})
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(tmp, ".env"), []byte("SECRET=match\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "src.txt"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if strings.Contains(text, ".env") {
		t.Errorf(".env should be skipped (denied), got: %s", text)
	}
	if !strings.Contains(text, "src.txt") {
		t.Errorf("src.txt should be found, got: %s", text)
	}
}

func TestGrepNoScopingWhenNoAllowDir(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt") {
		t.Errorf("should find files with no scoping, got: %s", text)
	}
}

func TestGrepFilesWithMatchesOffsetAfterMtimeSort(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)

	// Create 3 files with different mtimes.
	// After mtime sort (newest first), order should be: c.txt, b.txt, a.txt
	// With offset=1, we should skip c.txt (newest) and get b.txt, a.txt
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("match\n"), 0644)
	os.Chtimes(filepath.Join(tmp, "a.txt"), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("match\n"), 0644)
	os.Chtimes(filepath.Join(tmp, "b.txt"), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))

	os.WriteFile(filepath.Join(tmp, "c.txt"), []byte("match\n"), 0644)
	os.Chtimes(filepath.Join(tmp, "c.txt"), time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		OutputMode: "files_with_matches",
		Offset:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 files after offset=1, got %d: %s", len(lines), text)
	}
	// After sort (newest first) and offset=1: should skip c.txt, show b.txt then a.txt
	if lines[0] != "b.txt" {
		t.Errorf("expected b.txt first after offset, got: %s", lines[0])
	}
	if lines[1] != "a.txt" {
		t.Errorf("expected a.txt second after offset, got: %s", lines[1])
	}
}

// --- 3.9: Pagination tests ---

func TestGrepHeadLimitCountsAllOutputLines(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	// With context=1 around a match at line 3, output would be:
	// line2 (context), line3 (match), line4 (context) = 3 output lines
	// head_limit=2 should cap total output lines to 2 (not 2 match lines)
	content := "line1\nline2\nmatch\nline4\nline5\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	c := intPtr(1)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		Context:    c,
		HeadLimit:  2,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 output lines with head_limit=2 (counting context), got %d: %s", len(lines), text)
	}
}

func TestGrepHeadLimitTruncates(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	var content strings.Builder
	for i := 0; i < 10; i++ {
		content.WriteString("match\n")
	}
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content.String()), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		HeadLimit:  3,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines with head_limit=3, got %d: %s", len(lines), text)
	}
}

func TestGrepUnlimitedByDefault(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	var content strings.Builder
	for i := 0; i < 50; i++ {
		content.WriteString("match\n")
	}
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content.String()), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 50 {
		t.Errorf("expected all 50 lines with no limit, got %d", len(lines))
	}
}

func TestGrepOffsetSkipsResults(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	var content strings.Builder
	for i := 1; i <= 10; i++ {
		content.WriteString("match\n")
	}
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content.String()), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		HeadLimit:  3,
		Offset:     5,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines with offset=5 head_limit=3, got %d: %s", len(lines), text)
	}
	// Should start from line 6
	if !strings.Contains(lines[0], ":6:") {
		t.Errorf("expected first result at line 6, got: %s", lines[0])
	}
}

func TestGrepOffsetExceedsTotalReturnsEmpty(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		Offset:     100,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if text != "" {
		t.Errorf("expected empty result with offset exceeding total, got: %s", text)
	}
}

// --- 3.10: Case-insensitive tests ---

func TestGrepCaseInsensitive(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("Error\nerror\nERROR\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:         "error",
		Path:            "test.txt",
		OutputMode:      "content",
		CaseInsensitive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected case-insensitive match for Error, got: %s", text)
	}
	if !strings.Contains(text, "ERROR") {
		t.Errorf("expected case-insensitive match for ERROR, got: %s", text)
	}
}

func TestGrepCaseSensitiveByDefault(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("Error\nerror\nERROR\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "Error",
		Path:       "test.txt",
		OutputMode: "count",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if text != "test.txt:1" {
		t.Errorf("expected count 1 (case-sensitive), got: %s", text)
	}
}

// --- 3.11: Multiline tests ---

func TestGrepMultilineSpansLines(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "type Foo struct {\n\tName string\n}\n"
	os.WriteFile(filepath.Join(tmp, "test.go"), []byte(content), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    `struct \{.*?\}`,
		Path:       "test.go",
		OutputMode: "content",
		Multiline:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "struct {") {
		t.Errorf("expected multiline match, got: %s", text)
	}
	if !strings.Contains(text, "Name string") {
		t.Errorf("expected multiline match to span lines, got: %s", text)
	}
}

func TestGrepMultilineDisabledByDefault(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "foo\nbar\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "foo.*bar",
		Path:       "test.txt",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if text != "" {
		t.Errorf("expected no match without multiline, got: %s", text)
	}
}

func TestGrepMultilineFilesWithMatches(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "func main() {\n\treturn\n}\n"
	os.WriteFile(filepath.Join(tmp, "test.go"), []byte(content), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    `func.*\n.*return`,
		Path:       "test.go",
		OutputMode: "files_with_matches",
		Multiline:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if text != "test.go" {
		t.Errorf("expected test.go in multiline files_with_matches, got: %s", text)
	}
}

func TestGrepMultilineCountMode(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	// Pattern "ab" matches 3 times but across only 2 lines (lines 1 and 2).
	// In count mode with multiline, searchFileMultiline should report matching
	// line count (2), not regex match count (3). This bug manifests via the
	// directory search path (searchFile → searchFileMultiline).
	content := "ab ab\nab\nno match\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	// Search directory (not single file) to exercise searchFileMultiline path
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "ab",
		OutputMode: "count",
		Multiline:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if text != "test.txt:2" {
		t.Errorf("expected count of 2 (matching lines), got: %s", text)
	}
}

// --- 3.12: Line numbers tests ---

func TestGrepLineNumbersDefaultTrue(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt:1:match") {
		t.Errorf("expected line numbers by default, got: %s", text)
	}
}

func TestGrepLineNumbersFalse(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	ln := boolPtr(false)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:     "match",
		Path:        "test.txt",
		OutputMode:  "content",
		LineNumbers: ln,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if text != "test.txt:match" {
		t.Errorf("expected no line numbers, got: %s", text)
	}
}

func TestGrepLineNumbersIgnoredOutsideContent(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	ln := boolPtr(false)
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:     "match",
		OutputMode:  "files_with_matches",
		LineNumbers: ln,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	// Should be just file path
	if text != "test.txt" {
		t.Errorf("line_numbers should be ignored outside content mode, got: %s", text)
	}
}

// --- 3.13: Path handling tests ---

func TestGrepRelativePathResolved(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "main.go"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "match",
		Path:    "src",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "main.go") {
		t.Errorf("expected main.go in relative path search, got: %s", text)
	}
}

func TestGrepAbsolutePathUsedDirectly(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	absPath := filepath.Join(tmp, "test.txt")
	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       absPath,
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "match") {
		t.Errorf("expected match in absolute path search, got: %s", text)
	}
	// Single file search: output path should be as-provided
	if !strings.HasPrefix(text, absPath) {
		t.Errorf("expected path as-provided in output, got: %s", text)
	}
}

func TestGrepSingleFileSearch(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt:1:match") {
		t.Errorf("expected single file search result, got: %s", text)
	}
}

func TestGrepNonexistentPath(t *testing.T) {
	_, sess, resolver := grepTestSetup(t)

	r, err := callGrep(sess, resolver, GrepArgs{
		Pattern: "anything",
		Path:    "nonexistent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(r) {
		t.Error("expected error for nonexistent path")
	}
	if !hasErrorCode(r, ErrPathNotFound) {
		t.Errorf("expected error code %s, got: %s", ErrPathNotFound, resultText(r))
	}
}

// --- 3.14: Anthropic compat parameter tests ---

func TestGrepCompatGlob(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.go"), []byte("match\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "test.py"), []byte("match\n"), 0644)

	r, err := callGrepCompat(sess, resolver, GrepCompatArgs{
		Pattern: "match",
		Glob:    "*.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.go") {
		t.Errorf("expected test.go with compat glob, got: %s", text)
	}
	if strings.Contains(text, "test.py") {
		t.Errorf("test.py should be excluded with compat glob, got: %s", text)
	}
}

func TestGrepCompatCaseInsensitive(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("Error\nerror\n"), 0644)

	r, err := callGrepCompat(sess, resolver, GrepCompatArgs{
		Pattern:    "error",
		Path:       "test.txt",
		OutputMode: "content",
		I:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected case-insensitive match with -i, got: %s", text)
	}
}

func TestGrepCompatLineNumbers(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("match\n"), 0644)

	ln := boolPtr(false)
	r, err := callGrepCompat(sess, resolver, GrepCompatArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		N:          ln,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if text != "test.txt:match" {
		t.Errorf("expected no line numbers with -n=false, got: %s", text)
	}
}

func TestGrepCompatContextParams(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "line1\nline2\nmatch\nline4\nline5\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	b := intPtr(1)
	a := intPtr(1)
	r, err := callGrepCompat(sess, resolver, GrepCompatArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		B:          b,
		A:          a,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt-2-line2") {
		t.Errorf("expected -B context, got: %s", text)
	}
	if !strings.Contains(text, "test.txt:3:match") {
		t.Errorf("expected match line, got: %s", text)
	}
	if !strings.Contains(text, "test.txt-4-line4") {
		t.Errorf("expected -A context, got: %s", text)
	}
}

func TestGrepCompatCShorthand(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	content := "line1\nmatch\nline3\n"
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(content), 0644)

	c := intPtr(1)
	r, err := callGrepCompat(sess, resolver, GrepCompatArgs{
		Pattern:    "match",
		Path:       "test.txt",
		OutputMode: "content",
		C:          c,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "test.txt-1-line1") {
		t.Errorf("expected -C context before, got: %s", text)
	}
	if !strings.Contains(text, "test.txt-3-line3") {
		t.Errorf("expected -C context after, got: %s", text)
	}
}

func TestGrepNormalAndCompatProduceSameResults(t *testing.T) {
	tmp, sess, resolver := grepTestSetup(t)
	os.WriteFile(filepath.Join(tmp, "test.go"), []byte("Error here\nerror there\n"), 0644)

	cb := intPtr(0)
	ca := intPtr(0)
	normalR, err := callGrep(sess, resolver, GrepArgs{
		Pattern:         "error",
		Path:            "test.go",
		Include:         "*.go",
		OutputMode:      "content",
		CaseInsensitive: true,
		ContextBefore:   cb,
		ContextAfter:    ca,
	})
	if err != nil {
		t.Fatal(err)
	}

	b := intPtr(0)
	a := intPtr(0)
	compatR, err := callGrepCompat(sess, resolver, GrepCompatArgs{
		Pattern:    "error",
		Path:       "test.go",
		Glob:       "*.go",
		OutputMode: "content",
		I:          true,
		B:          b,
		A:          a,
	})
	if err != nil {
		t.Fatal(err)
	}

	normalText := resultText(normalR)
	compatText := resultText(compatR)
	if normalText != compatText {
		t.Errorf("normal and compat should produce identical results\nnormal: %s\ncompat: %s", normalText, compatText)
	}
}

// --- 3.15: Integration tests ---

func TestIntegrationGrepInToolList(t *testing.T) {
	tmp := t.TempDir()

	// Test split mode
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
	if !toolNames["grep"] {
		t.Error("grep tool should be in split mode tool list")
	}
}

func TestIntegrationGrepInAnthropicCompatToolList(t *testing.T) {
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
	if !toolNames["grep"] {
		t.Error("grep tool should be in anthropic-compat tool list")
	}
	if !toolNames["str_replace_editor"] {
		t.Error("str_replace_editor should be in anthropic-compat tool list")
	}

	// Check compat schema uses compat parameter names
	for _, tool := range toolList.Tools {
		if tool.Name == "grep" {
			schemaMap, ok := tool.InputSchema.(map[string]interface{})
			if !ok {
				t.Fatal("grep tool should have input schema map")
			}
			props, ok := schemaMap["properties"].(map[string]interface{})
			if !ok {
				t.Fatal("expected properties in grep schema")
			}
			if _, ok := props["glob"]; !ok {
				t.Error("compat mode should have 'glob' parameter")
			}
			if _, ok := props["-i"]; !ok {
				t.Error("compat mode should have '-i' parameter")
			}
			if _, ok := props["-n"]; !ok {
				t.Error("compat mode should have '-n' parameter")
			}
			if _, ok := props["-A"]; !ok {
				t.Error("compat mode should have '-A' parameter")
			}
			if _, ok := props["-B"]; !ok {
				t.Error("compat mode should have '-B' parameter")
			}
			if _, ok := props["-C"]; !ok {
				t.Error("compat mode should have '-C' parameter")
			}
		}
	}
}

func TestIntegrationGrepWithNoBash(t *testing.T) {
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
		NoBash:         true,
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
	if !toolNames["grep"] {
		t.Error("grep tool should be available with --no-bash")
	}
	if toolNames["bash"] {
		t.Error("bash tool should NOT be available with --no-bash")
	}
}

// Helper functions
func intPtr(v int) *int   { return &v }
func boolPtr(v bool) *bool { return &v }
