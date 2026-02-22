package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/mjkoo/boris/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestIntegrationToolLifecycle(t *testing.T) {
	tmp := t.TempDir()

	// Create server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "boris-test",
		Version: "test",
	}, nil)

	sess := session.New(tmp)
	resolver, err := pathscope.NewResolver([]string{tmp}, nil)
	if err != nil {
		t.Fatal(err)
	}

	tools.RegisterAll(server, resolver, sess, tools.Config{
		MaxFileSize:    10 * 1024 * 1024,
		DefaultTimeout: 30,
		Shell:          "/bin/sh",
	})

	// Connect via in-memory transport
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

	// Step 1: create_file
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_file",
		Arguments: map[string]interface{}{
			"path":    "hello.txt",
			"content": "hello world\nfoo bar\nbaz\n",
		},
	})
	if err != nil {
		t.Fatalf("create_file failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("create_file returned error: %s", contentText(res))
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(tmp, "hello.txt")); err != nil {
		t.Fatalf("file should exist: %v", err)
	}

	// Step 2: view
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "view",
		Arguments: map[string]interface{}{
			"path": "hello.txt",
		},
	})
	if err != nil {
		t.Fatalf("view failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("view returned error: %s", contentText(res))
	}
	text := contentText(res)
	if !strings.Contains(text, "hello world") {
		t.Errorf("view should show file content, got: %s", text)
	}
	if !strings.Contains(text, "1\t") {
		t.Errorf("view should show line numbers, got: %s", text)
	}

	// Step 3: str_replace
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace",
		Arguments: map[string]interface{}{
			"path":    "hello.txt",
			"old_str": "foo bar",
			"new_str": "REPLACED",
		},
	})
	if err != nil {
		t.Fatalf("str_replace failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("str_replace returned error: %s", contentText(res))
	}

	// Step 4: view again to verify change
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "view",
		Arguments: map[string]interface{}{
			"path": "hello.txt",
		},
	})
	if err != nil {
		t.Fatalf("view after replace failed: %v", err)
	}
	text = contentText(res)
	if !strings.Contains(text, "REPLACED") {
		t.Errorf("view should show replacement, got: %s", text)
	}
	if strings.Contains(text, "foo bar") {
		t.Error("view should not show old string")
	}

	// Step 5: bash
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "bash",
		Arguments: map[string]interface{}{
			"command": "cat hello.txt",
		},
	})
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("bash returned error: %s", contentText(res))
	}
	text = contentText(res)
	if !strings.Contains(text, "REPLACED") {
		t.Errorf("bash cat should show replaced content, got: %s", text)
	}

	// Step 6: create_file overwrites existing
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_file",
		Arguments: map[string]interface{}{
			"path":    "hello.txt",
			"content": "overwritten\n",
		},
	})
	if err != nil {
		t.Fatalf("create_file overwrite failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("create_file overwrite returned error: %s", contentText(res))
	}
	data, _ := os.ReadFile(filepath.Join(tmp, "hello.txt"))
	if string(data) != "overwritten\n" {
		t.Errorf("expected overwritten content, got: %s", data)
	}

	// Step 7: str_replace with replace_all
	os.WriteFile(filepath.Join(tmp, "multi.txt"), []byte("aaa bbb aaa ccc aaa\n"), 0644)
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace",
		Arguments: map[string]interface{}{
			"path":        "multi.txt",
			"old_str":     "aaa",
			"new_str":     "ZZZ",
			"replace_all": true,
		},
	})
	if err != nil {
		t.Fatalf("str_replace replace_all failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("str_replace replace_all returned error: %s", contentText(res))
	}
	data, _ = os.ReadFile(filepath.Join(tmp, "multi.txt"))
	if strings.Contains(string(data), "aaa") {
		t.Error("replace_all should have replaced all occurrences")
	}
}

func TestIntegrationAnthropicCompat(t *testing.T) {
	tmp := t.TempDir()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "boris-test",
		Version: "test",
	}, nil)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver([]string{tmp}, nil)

	tools.RegisterAll(server, resolver, sess, tools.Config{
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

	// create via combined tool
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace_editor",
		Arguments: map[string]interface{}{
			"command":   "create",
			"path":      "test.txt",
			"file_text": "hello world\n",
		},
	})
	if err != nil {
		t.Fatalf("create via combined tool failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("create returned error: %s", contentText(res))
	}

	// view via combined tool
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace_editor",
		Arguments: map[string]interface{}{
			"command": "view",
			"path":    "test.txt",
		},
	})
	if err != nil {
		t.Fatalf("view via combined tool failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("view returned error: %s", contentText(res))
	}
	text := contentText(res)
	if !strings.Contains(text, "hello world") {
		t.Errorf("expected content, got: %s", text)
	}

	// str_replace via combined tool
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace_editor",
		Arguments: map[string]interface{}{
			"command": "str_replace",
			"path":    "test.txt",
			"old_str": "hello world",
			"new_str": "REPLACED",
		},
	})
	if err != nil {
		t.Fatalf("str_replace via combined tool failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("str_replace returned error: %s", contentText(res))
	}

	// invalid command — rejected by schema validation at protocol level
	_, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace_editor",
		Arguments: map[string]interface{}{
			"command": "delete",
			"path":    "test.txt",
		},
	})
	if err == nil {
		t.Error("expected protocol error for invalid enum value")
	}

	// Verify split tools are NOT registered
	_, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "view",
		Arguments: map[string]interface{}{
			"path": "test.txt",
		},
	})
	if err == nil {
		t.Error("expected error for 'view' tool in anthropic-compat mode (should not exist)")
	}
}

func contentText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}
