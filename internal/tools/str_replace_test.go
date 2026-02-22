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

func TestStrReplaceSuccessful(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	os.WriteFile(file, []byte("hello world\nfoo bar\nbaz\n"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := strReplaceHandler(sess, resolver)

	result, _, err := handler(context.Background(), nil, StrReplaceArgs{
		Path:   file,
		OldStr: "foo bar",
		NewStr: "replaced",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "Replaced") {
		t.Errorf("expected confirmation, got: %s", text)
	}

	// Verify file content
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "replaced") {
		t.Errorf("file should contain 'replaced': %s", data)
	}
	if strings.Contains(string(data), "foo bar") {
		t.Error("file should not contain 'foo bar'")
	}
}

func TestStrReplaceNotFound(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	os.WriteFile(file, []byte("hello\n"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := strReplaceHandler(sess, resolver)

	result, _, err := handler(context.Background(), nil, StrReplaceArgs{
		Path:   file,
		OldStr: "nonexistent",
		NewStr: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("expected IsError for string not found")
	}
	if !strings.Contains(resultText(result), "not found") {
		t.Errorf("expected 'not found' error, got: %s", resultText(result))
	}
}

func TestStrReplaceMultipleOccurrences(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	os.WriteFile(file, []byte("aaa bbb aaa\n"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := strReplaceHandler(sess, resolver)

	result, _, err := handler(context.Background(), nil, StrReplaceArgs{
		Path:   file,
		OldStr: "aaa",
		NewStr: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("expected IsError for multiple occurrences")
	}
	if !strings.Contains(resultText(result), "2 occurrences") {
		t.Errorf("expected occurrence count in error, got: %s", resultText(result))
	}
}

func TestStrReplaceDeletion(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	os.WriteFile(file, []byte("keep DELETE keep\n"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := strReplaceHandler(sess, resolver)

	_, _, err := handler(context.Background(), nil, StrReplaceArgs{
		Path:   file,
		OldStr: " DELETE",
	})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(file)
	if string(data) != "keep keep\n" {
		t.Errorf("expected 'keep keep\\n', got %q", data)
	}
}

func TestStrReplaceAll(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	os.WriteFile(file, []byte("aaa bbb aaa ccc aaa\n"), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := strReplaceHandler(sess, resolver)

	t.Run("multiple replacements with count", func(t *testing.T) {
		result, _, err := handler(context.Background(), nil, StrReplaceArgs{
			Path:       file,
			OldStr:     "aaa",
			NewStr:     "XXX",
			ReplaceAll: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		if !strings.Contains(text, "3 occurrences") {
			t.Errorf("expected '3 occurrences', got: %s", text)
		}
		data, _ := os.ReadFile(file)
		if strings.Contains(string(data), "aaa") {
			t.Error("file should not contain 'aaa' after replace_all")
		}
		if strings.Count(string(data), "XXX") != 3 {
			t.Errorf("expected 3 XXX replacements, got: %s", data)
		}
	})

	t.Run("no match error", func(t *testing.T) {
		result, _, err := handler(context.Background(), nil, StrReplaceArgs{
			Path:       file,
			OldStr:     "nonexistent",
			NewStr:     "y",
			ReplaceAll: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !isErrorResult(result) {
			t.Error("expected IsError for no matches even with replace_all")
		}
	})

	t.Run("replace_all false preserves uniqueness check", func(t *testing.T) {
		// Write file with duplicates
		os.WriteFile(file, []byte("aaa bbb aaa\n"), 0644)
		result, _, err := handler(context.Background(), nil, StrReplaceArgs{
			Path:       file,
			OldStr:     "aaa",
			NewStr:     "x",
			ReplaceAll: false,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !isErrorResult(result) {
			t.Error("expected IsError for multiple occurrences without replace_all")
		}
	})
}

func TestStrReplaceEmptyOldStr(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.txt")
	original := "hello world\n"
	os.WriteFile(file, []byte(original), 0644)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := strReplaceHandler(sess, resolver)

	t.Run("replace_all true", func(t *testing.T) {
		result, _, err := handler(context.Background(), nil, StrReplaceArgs{
			Path:       file,
			OldStr:     "",
			NewStr:     "inserted",
			ReplaceAll: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !isErrorResult(result) {
			t.Error("expected IsError for empty old_str")
		}
		data, _ := os.ReadFile(file)
		if string(data) != original {
			t.Errorf("file should be unchanged, got %q", data)
		}
	})

	t.Run("replace_all false", func(t *testing.T) {
		result, _, err := handler(context.Background(), nil, StrReplaceArgs{
			Path:   file,
			OldStr: "",
			NewStr: "inserted",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !isErrorResult(result) {
			t.Error("expected IsError for empty old_str")
		}
		data, _ := os.ReadFile(file)
		if string(data) != original {
			t.Errorf("file should be unchanged, got %q", data)
		}
	})
}

func TestStrReplacePreservesPermissions(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test.sh")
	os.WriteFile(file, []byte("old content\n"), 0755)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := strReplaceHandler(sess, resolver)

	_, _, err := handler(context.Background(), nil, StrReplaceArgs{
		Path:   file,
		OldStr: "old content",
		NewStr: "new content",
	})
	if err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(file)
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected mode 0755, got %o", info.Mode().Perm())
	}
}

func TestStrReplacePathScoping(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver([]string{tmp}, nil)
	handler := strReplaceHandler(sess, resolver)

	result, _, err := handler(context.Background(), nil, StrReplaceArgs{
		Path:   "/etc/hostname",
		OldStr: "x",
		NewStr: "y",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("expected IsError for path scoping violation")
	}
}

func TestStrReplaceFileNotFound(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver(nil, nil)
	handler := strReplaceHandler(sess, resolver)

	result, _, err := handler(context.Background(), nil, StrReplaceArgs{
		Path:   filepath.Join(tmp, "nonexistent"),
		OldStr: "x",
		NewStr: "y",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isErrorResult(result) {
		t.Error("expected IsError for nonexistent file")
	}
}
