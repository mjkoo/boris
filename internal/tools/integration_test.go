package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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
	t.Cleanup(sess.Close)
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

func TestIntegrationRegistrationCallback(t *testing.T) {
	tmp := t.TempDir()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "boris-test",
		Version: "test",
	}, nil)

	sess := session.New(tmp)
	t.Cleanup(sess.Close)
	resolver, err := pathscope.NewResolver([]string{tmp}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var regCount atomic.Int32
	cfg := tools.Config{
		MaxFileSize:    10 * 1024 * 1024,
		DefaultTimeout: 30,
		Shell:          "/bin/sh",
		RegisterSession: func(sessionID string) {
			regCount.Add(1)
		},
	}
	tools.RegisterAll(server, resolver, sess, cfg)

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

	// First bash call — callback should fire.
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "bash",
		Arguments: map[string]interface{}{"command": "echo hello"},
	})
	if err != nil {
		t.Fatalf("bash call failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("bash returned error: %s", contentText(res))
	}
	if got := regCount.Load(); got != 1 {
		t.Errorf("after first bash call: regCount = %d, want 1", got)
	}

	// Second bash call — callback should NOT fire again (sync.Once).
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "bash",
		Arguments: map[string]interface{}{"command": "echo again"},
	})
	if err != nil {
		t.Fatalf("second bash call failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("second bash returned error: %s", contentText(res))
	}
	if got := regCount.Load(); got != 1 {
		t.Errorf("after second bash call: regCount = %d, want 1", got)
	}

	// First task_output call — separate regOnce, callback fires again.
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "task_output",
		Arguments: map[string]interface{}{"task_id": "nonexistent"},
	})
	if err != nil {
		t.Fatalf("task_output call failed: %v", err)
	}
	// task_output returns an error for unknown task_id, but the callback
	// should still have fired.
	if got := regCount.Load(); got != 2 {
		t.Errorf("after task_output call: regCount = %d, want 2", got)
	}
}

func TestIntegrationAnthropicCompatViewBeforeEdit(t *testing.T) {
	tmp := t.TempDir()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "boris-test",
		Version: "test",
	}, nil)

	sess := session.New(tmp)
	resolver, _ := pathscope.NewResolver([]string{tmp}, nil)

	tools.RegisterAll(server, resolver, sess, tools.Config{
		MaxFileSize:           10 * 1024 * 1024,
		DefaultTimeout:        30,
		Shell:                 "/bin/sh",
		AnthropicCompat:       true,
		RequireViewBeforeEdit: true,
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

	// Create a file via str_replace_editor create (new file, no view needed)
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace_editor",
		Arguments: map[string]interface{}{
			"command":   "create",
			"path":      "test.txt",
			"file_text": "hello world\n",
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("create returned error: %s", contentText(res))
	}

	// Try str_replace without viewing — should fail
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace_editor",
		Arguments: map[string]interface{}{
			"command": "str_replace",
			"path":    "test.txt",
			"old_str": "hello",
			"new_str": "goodbye",
		},
	})
	if err != nil {
		t.Fatalf("str_replace call failed: %v", err)
	}
	if !res.IsError {
		t.Error("str_replace should fail without prior view")
	}
	if !strings.Contains(contentText(res), "FILE_NOT_VIEWED") {
		t.Errorf("expected FILE_NOT_VIEWED error, got: %s", contentText(res))
	}

	// View via str_replace_editor view command
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace_editor",
		Arguments: map[string]interface{}{
			"command": "view",
			"path":    "test.txt",
		},
	})
	if err != nil {
		t.Fatalf("view failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("view returned error: %s", contentText(res))
	}

	// Now str_replace should succeed
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace_editor",
		Arguments: map[string]interface{}{
			"command": "str_replace",
			"path":    "test.txt",
			"old_str": "hello",
			"new_str": "goodbye",
		},
	})
	if err != nil {
		t.Fatalf("str_replace after view failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("str_replace should succeed after view, got: %s", contentText(res))
	}
}

func TestIntegrationViewBeforeEditFlow(t *testing.T) {
	tmp := t.TempDir()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "boris-test",
		Version: "test",
	}, nil)

	sess := session.New(tmp)
	t.Cleanup(sess.Close)
	resolver, _ := pathscope.NewResolver([]string{tmp}, nil)

	tools.RegisterAll(server, resolver, sess, tools.Config{
		MaxFileSize:           10 * 1024 * 1024,
		DefaultTimeout:        30,
		Shell:                 "/bin/sh",
		RequireViewBeforeEdit: true,
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

	// Create a file (new file, no view needed)
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_file",
		Arguments: map[string]interface{}{
			"path":    "test.txt",
			"content": "hello world\nfoo bar\n",
		},
	})
	if err != nil {
		t.Fatalf("create_file failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("create_file returned error: %s", contentText(res))
	}

	// str_replace without view — should fail
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace",
		Arguments: map[string]interface{}{
			"path":    "test.txt",
			"old_str": "foo bar",
			"new_str": "REPLACED",
		},
	})
	if err != nil {
		t.Fatalf("str_replace call failed: %v", err)
	}
	if !res.IsError {
		t.Error("str_replace should fail without prior view")
	}
	if !strings.Contains(contentText(res), "FILE_NOT_VIEWED") {
		t.Errorf("expected FILE_NOT_VIEWED error, got: %s", contentText(res))
	}

	// View the file
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "view",
		Arguments: map[string]interface{}{
			"path": "test.txt",
		},
	})
	if err != nil {
		t.Fatalf("view failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("view returned error: %s", contentText(res))
	}

	// str_replace after view — should succeed
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "str_replace",
		Arguments: map[string]interface{}{
			"path":    "test.txt",
			"old_str": "foo bar",
			"new_str": "REPLACED",
		},
	})
	if err != nil {
		t.Fatalf("str_replace after view failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("str_replace should succeed after view, got: %s", contentText(res))
	}
}

func TestIntegrationDisableTools(t *testing.T) {
	t.Run("disable single tool", func(t *testing.T) {
		tmp := t.TempDir()

		server := mcp.NewServer(&mcp.Implementation{
			Name:    "boris-test",
			Version: "test",
		}, nil)

		sess := session.New(tmp)
		t.Cleanup(sess.Close)
		resolver, _ := pathscope.NewResolver([]string{tmp}, nil)

		tools.RegisterAll(server, resolver, sess, tools.Config{
			MaxFileSize:    10 * 1024 * 1024,
			DefaultTimeout: 30,
			Shell:          "/bin/sh",
			DisableTools:   map[string]struct{}{"bash": {}},
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
		if toolNames["bash"] {
			t.Error("bash should be disabled")
		}
		if toolNames["task_output"] {
			t.Error("task_output should be disabled when bash is disabled")
		}
		if !toolNames["view"] {
			t.Error("view should still be available")
		}
		if !toolNames["grep"] {
			t.Error("grep should still be available")
		}
	})

	t.Run("disable multiple tools", func(t *testing.T) {
		tmp := t.TempDir()

		server := mcp.NewServer(&mcp.Implementation{
			Name:    "boris-test",
			Version: "test",
		}, nil)

		sess := session.New(tmp)
		t.Cleanup(sess.Close)
		resolver, _ := pathscope.NewResolver([]string{tmp}, nil)

		tools.RegisterAll(server, resolver, sess, tools.Config{
			MaxFileSize:    10 * 1024 * 1024,
			DefaultTimeout: 30,
			Shell:          "/bin/sh",
			DisableTools:   map[string]struct{}{"bash": {}, "create_file": {}},
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
		if toolNames["bash"] {
			t.Error("bash should be disabled")
		}
		if toolNames["create_file"] {
			t.Error("create_file should be disabled")
		}
		if !toolNames["view"] {
			t.Error("view should still be available")
		}
		if !toolNames["str_replace"] {
			t.Error("str_replace should still be available")
		}
	})

	t.Run("anthropic-compat disable mapping", func(t *testing.T) {
		tmp := t.TempDir()

		server := mcp.NewServer(&mcp.Implementation{
			Name:    "boris-test",
			Version: "test",
		}, nil)

		sess := session.New(tmp)
		t.Cleanup(sess.Close)
		resolver, _ := pathscope.NewResolver([]string{tmp}, nil)

		tools.RegisterAll(server, resolver, sess, tools.Config{
			MaxFileSize:     10 * 1024 * 1024,
			DefaultTimeout:  30,
			Shell:           "/bin/sh",
			AnthropicCompat: true,
			DisableTools:    map[string]struct{}{"view": {}},
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
		if toolNames["str_replace_editor"] {
			t.Error("str_replace_editor should be disabled when view is disabled in anthropic-compat mode")
		}
		if !toolNames["bash"] {
			t.Error("bash should still be available")
		}
	})

	t.Run("unknown tool name validation", func(t *testing.T) {
		err := tools.ValidateDisableTools(
			map[string]struct{}{"nonexistent": {}},
			false,
		)
		if err == nil {
			t.Error("expected error for unknown tool name")
		}
		if !strings.Contains(err.Error(), "nonexistent") {
			t.Errorf("error should mention the unknown name, got: %v", err)
		}
	})

	t.Run("valid tool names accepted", func(t *testing.T) {
		err := tools.ValidateDisableTools(
			map[string]struct{}{"bash": {}, "grep": {}},
			false,
		)
		if err != nil {
			t.Errorf("expected no error for valid tool names, got: %v", err)
		}
	})

	t.Run("anthropic-compat accepts standard names", func(t *testing.T) {
		err := tools.ValidateDisableTools(
			map[string]struct{}{"view": {}},
			true,
		)
		if err != nil {
			t.Errorf("expected no error for standard name in anthropic-compat mode, got: %v", err)
		}
	})
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
