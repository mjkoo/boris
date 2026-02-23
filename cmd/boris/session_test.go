package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/mjkoo/boris/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// testServerConfig returns a serverConfig suitable for testing with
// per-connection session isolation.
func testServerConfig(t *testing.T, workdir string) serverConfig {
	t.Helper()
	resolver, err := pathscope.NewResolver(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return serverConfig{
		workdir:  workdir,
		resolver: resolver,
		impl: &mcp.Implementation{
			Name:    "boris-test",
			Version: "test",
		},
		toolsCfg: tools.Config{
			Shell:          "/bin/sh",
			DefaultTimeout: 30,
			MaxFileSize:    10 * 1024 * 1024,
		},
	}
}

// newTestHTTPServer creates an httptest.Server backed by a per-connection
// session model matching the production runHTTP logic. It registers cleanup
// via t.Cleanup so that the server is closed after all client sessions.
func newTestHTTPServer(t *testing.T, cfg serverConfig) *httptest.Server {
	t.Helper()
	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		server := mcp.NewServer(cfg.impl, nil)
		sess := session.New(cfg.workdir)
		tools.RegisterAll(server, cfg.resolver, sess, cfg.toolsCfg)
		return server
	}, nil)

	srv := httptest.NewServer(handler)
	// Register server cleanup first (LIFO: runs after client cleanups)
	t.Cleanup(func() { srv.Close() })
	return srv
}

// connectHTTPClient creates an MCP client connected to the given httptest.Server.
// The client session is closed via t.Cleanup (before server close due to LIFO).
func connectHTTPClient(t *testing.T, ctx context.Context, srv *httptest.Server) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "test",
	}, nil)
	clientSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: srv.URL,
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })
	return clientSession
}

// callBash calls the bash tool and returns the text output.
func callBash(t *testing.T, ctx context.Context, cs *mcp.ClientSession, command string) string {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "bash",
		Arguments: map[string]interface{}{
			"command": command,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(bash %q): %v", command, err)
	}
	return toolResultText(res)
}

func toolResultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}

// TestHTTPSessionIsolationCwd verifies that two HTTP clients connecting to the
// same boris server get independent working directories.
func TestHTTPSessionIsolationCwd(t *testing.T) {
	workdir := t.TempDir()
	cfg := testServerConfig(t, workdir)
	srv := newTestHTTPServer(t, cfg)

	ctx := context.Background()

	// Connect two separate clients
	clientA := connectHTTPClient(t, ctx, srv)
	clientB := connectHTTPClient(t, ctx, srv)

	// Client A: cd /var (using /var instead of /tmp because t.TempDir() is under /tmp)
	callBash(t, ctx, clientA, "cd /var")

	// Client A: verify cwd changed
	textA := callBash(t, ctx, clientA, "pwd")
	if !strings.Contains(textA, "/var") {
		t.Errorf("client A pwd should be /var, got: %s", textA)
	}

	// Client B: pwd should see initial workdir, not /var
	textB := callBash(t, ctx, clientB, "pwd")
	if !strings.Contains(textB, workdir) {
		t.Errorf("client B pwd should be initial workdir %q, got: %s", workdir, textB)
	}
	if strings.Contains(textB, "/var") {
		t.Errorf("client B should NOT see /var (client A's cwd), got: %s", textB)
	}
}

// TestHTTPSessionIsolationBackgroundTasks verifies that background task IDs are
// scoped per connection. Client A starts a background task, client B cannot
// retrieve it.
func TestHTTPSessionIsolationBackgroundTasks(t *testing.T) {
	workdir := t.TempDir()
	cfg := testServerConfig(t, workdir)
	srv := newTestHTTPServer(t, cfg)

	ctx := context.Background()

	clientA := connectHTTPClient(t, ctx, srv)
	clientB := connectHTTPClient(t, ctx, srv)

	// Client A: start a background task
	res, err := clientA.CallTool(ctx, &mcp.CallToolParams{
		Name: "bash",
		Arguments: map[string]interface{}{
			"command":           "sleep 1",
			"run_in_background": true,
		},
	})
	if err != nil {
		t.Fatalf("client A background task: %v", err)
	}
	text := toolResultText(res)
	taskID := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "task_id: ") {
			taskID = strings.TrimPrefix(line, "task_id: ")
			break
		}
	}
	if taskID == "" {
		t.Fatalf("no task_id in client A response: %s", text)
	}

	// Client B: try to retrieve client A's task
	res, err = clientB.CallTool(ctx, &mcp.CallToolParams{
		Name: "task_output",
		Arguments: map[string]interface{}{
			"task_id": taskID,
		},
	})
	if err != nil {
		t.Fatalf("client B task_output: %v", err)
	}
	text = toolResultText(res)
	if !res.IsError {
		t.Errorf("client B should NOT be able to access client A's task, got: %s", text)
	}
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' error, got: %s", text)
	}
}

// TestHTTPSessionCwdPersistence verifies that CWD changes persist within a
// single HTTP session across multiple tool calls. This demonstrates that the
// go-sdk reuses the same mcp.Server (and thus the same session.Session) for
// requests with the same Mcp-Session-Id — the same property that makes CWD
// survive across reconnects.
func TestHTTPSessionCwdPersistence(t *testing.T) {
	workdir := t.TempDir()
	cfg := testServerConfig(t, workdir)

	var mu sync.Mutex
	getServerCalls := 0
	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		mu.Lock()
		getServerCalls++
		mu.Unlock()
		server := mcp.NewServer(cfg.impl, nil)
		sess := session.New(cfg.workdir)
		tools.RegisterAll(server, cfg.resolver, sess, cfg.toolsCfg)
		return server
	}, nil)

	srv := httptest.NewServer(handler)
	t.Cleanup(func() { srv.Close() })

	ctx := context.Background()
	client := connectHTTPClient(t, ctx, srv)

	// cd to /var
	callBash(t, ctx, client, "cd /var")

	// pwd should reflect the cd
	text := callBash(t, ctx, client, "pwd")
	if !strings.Contains(text, "/var") {
		t.Errorf("pwd should show /var after cd, got: %s", text)
	}

	// cd again
	callBash(t, ctx, client, "cd /")

	text = callBash(t, ctx, client, "pwd")
	// Check that output contains a standalone "/" and not "/var"
	lines := strings.Split(text, "\n")
	foundRoot := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "/" {
			foundRoot = true
			break
		}
	}
	if !foundRoot {
		t.Errorf("pwd should show / after second cd, got: %s", text)
	}

	// getServer should have been called exactly once for this client
	mu.Lock()
	calls := getServerCalls
	mu.Unlock()
	if calls != 1 {
		t.Errorf("getServer should be called once per session, got %d calls", calls)
	}
}

// newTestHTTPServerWithMux creates an httptest.Server with the full mux
// (including /health) and optional bearer auth on /mcp, matching production
// runHTTP wiring.
func newTestHTTPServerWithMux(t *testing.T, cfg serverConfig, token string) *httptest.Server {
	t.Helper()
	var mcpHandler http.Handler = mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		server := mcp.NewServer(cfg.impl, nil)
		sess := session.New(cfg.workdir)
		tools.RegisterAll(server, cfg.resolver, sess, cfg.toolsCfg)
		return server
	}, nil)

	if token != "" {
		mcpHandler = bearerAuthMiddleware(token, mcpHandler)
	}

	mux := buildMux(mcpHandler)

	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })
	return srv
}

// TestHTTPAuthRequiredOnMCP verifies that when a token is set, /mcp requires
// auth but /health does not.
func TestHTTPAuthRequiredOnMCP(t *testing.T) {
	workdir := t.TempDir()
	cfg := testServerConfig(t, workdir)
	token := "test-secret-token"
	srv := newTestHTTPServerWithMux(t, cfg, token)

	// /mcp without auth → 401
	resp, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("/mcp without auth: status = %d, want 401", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var errBody map[string]string
	if err := json.Unmarshal(body, &errBody); err != nil {
		t.Fatalf("failed to decode 401 body: %v", err)
	}
	if errBody["error"] != "unauthorized" {
		t.Errorf("401 body error = %q, want %q", errBody["error"], "unauthorized")
	}

	// /mcp with wrong token → 401
	req, _ := http.NewRequest("POST", srv.URL+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp with wrong token: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("/mcp with wrong token: status = %d, want 401", resp2.StatusCode)
	}

	// /mcp with correct token → not 401 (will be some MCP response)
	req, _ = http.NewRequest("POST", srv.URL+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp with correct token: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode == http.StatusUnauthorized {
		t.Errorf("/mcp with correct token should not be 401, got %d", resp3.StatusCode)
	}

	// /health without auth → 200
	resp4, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("/health: status = %d, want 200", resp4.StatusCode)
	}
}

// newTestHTTPServerWithLifecycle creates a test HTTP server with full session
// lifecycle wiring (registry, EventStore, SessionTimeout) matching production
// runHTTP. The short timeout ensures tests don't wait long for idle cleanup.
func newTestHTTPServerWithLifecycle(t *testing.T, cfg serverConfig, sessionTimeout time.Duration) *httptest.Server {
	t.Helper()
	registry := session.NewRegistry()
	store := &session.SessionCleanupStore{Registry: registry}

	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		server := mcp.NewServer(cfg.impl, nil)
		sess := session.New(cfg.workdir)
		toolsCfg := cfg.toolsCfg
		toolsCfg.RegisterSession = func(sessionID string) {
			registry.Register(sessionID, sess)
		}
		tools.RegisterAll(server, cfg.resolver, sess, toolsCfg)
		return server
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout: sessionTimeout,
		EventStore:     store,
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(func() { srv.Close() })
	return srv
}

// TestHTTPSessionCleanupOnDelete verifies that when a client sends a DELETE
// (explicit session close), background tasks in that session are killed.
func TestHTTPSessionCleanupOnDelete(t *testing.T) {
	workdir := t.TempDir()
	cfg := testServerConfig(t, workdir)
	srv := newTestHTTPServerWithLifecycle(t, cfg, 10*time.Minute)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: srv.URL,
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}

	// Start a background task that writes its PID to a unique file
	pidFile := workdir + "/bg.pid"
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "bash",
		Arguments: map[string]interface{}{
			"command":           "echo $$ > " + pidFile + " && sleep 300",
			"run_in_background": true,
		},
	})
	if err != nil {
		t.Fatalf("start background task: %v", err)
	}
	text := toolResultText(res)
	if !strings.Contains(text, "task_id:") {
		t.Fatalf("expected task_id in response, got: %s", text)
	}

	// Wait for PID file to be written
	var pid string
	for i := 0; i < 20; i++ {
		data, err := os.ReadFile(pidFile)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			pid = strings.TrimSpace(string(data))
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pid == "" {
		t.Fatal("PID file was not written")
	}

	// Close the client session — sends DELETE, triggers cleanup chain
	clientSession.Close()

	// Give the cleanup chain time to run (SIGTERM → process exit)
	time.Sleep(2 * time.Second)

	// Verify the process is gone
	if _, err := os.Stat("/proc/" + pid); err == nil {
		t.Errorf("background task process (PID %s) should be killed after session close", pid)
	}
}

// TestHTTPSessionCleanupOnIdleTimeout verifies that idle sessions are cleaned
// up by the SDK's SessionTimeout, which triggers our EventStore.SessionClosed.
func TestHTTPSessionCleanupOnIdleTimeout(t *testing.T) {
	workdir := t.TempDir()
	cfg := testServerConfig(t, workdir)
	// Use a very short timeout for testing
	srv := newTestHTTPServerWithLifecycle(t, cfg, 1*time.Second)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: srv.URL,
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}

	// Start a background task that writes its PID to a unique file
	pidFile := workdir + "/bg.pid"
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "bash",
		Arguments: map[string]interface{}{
			"command":           "echo $$ > " + pidFile + " && sleep 300",
			"run_in_background": true,
		},
	})
	if err != nil {
		t.Fatalf("start background task: %v", err)
	}
	text := toolResultText(res)
	if !strings.Contains(text, "task_id:") {
		t.Fatalf("expected task_id in response, got: %s", text)
	}

	// Wait for PID file to be written
	var pid string
	for i := 0; i < 20; i++ {
		data, err := os.ReadFile(pidFile)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			pid = strings.TrimSpace(string(data))
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pid == "" {
		t.Fatal("PID file was not written")
	}

	// Wait for the idle timeout (1s) plus buffer for cleanup (SIGTERM → exit)
	time.Sleep(4 * time.Second)

	// Verify the process is gone
	if _, err := os.Stat("/proc/" + pid); err == nil {
		t.Errorf("background task process (PID %s) should be killed after idle timeout", pid)
	}
}

// TestSTDIOSessionCleanup verifies that background tasks are killed when a
// STDIO session ends (simulated by using in-memory transport + sess.Close).
func TestSTDIOSessionCleanup(t *testing.T) {
	workdir := t.TempDir()
	cfg := testServerConfig(t, workdir)

	server := mcp.NewServer(cfg.impl, nil)
	sess := session.New(cfg.workdir)
	defer sess.Close()
	tools.RegisterAll(server, cfg.resolver, sess, cfg.toolsCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Use a unique marker file to track the background process
	marker := workdir + "/bg-alive"

	// Start a background task that writes a marker file
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "bash",
		Arguments: map[string]interface{}{
			"command":           "touch " + marker + " && sleep 300",
			"run_in_background": true,
		},
	})
	if err != nil {
		t.Fatalf("start background task: %v", err)
	}
	text := toolResultText(res)
	if !strings.Contains(text, "task_id:") {
		t.Fatalf("expected task_id in response, got: %s", text)
	}

	// Verify task is tracked
	if sess.TaskCount() != 1 {
		t.Fatalf("expected 1 task, got %d", sess.TaskCount())
	}

	// Wait for marker file to be created
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker file should exist: %v", err)
	}

	// Close the client session and explicitly close the Boris session
	// (simulating what defer sess.Close() does in runSTDIO)
	clientSession.Close()
	sess.Close()

	// After Close returns, all tasks should be cleaned up
	if sess.TaskCount() != 0 {
		t.Errorf("expected 0 tasks after close, got %d", sess.TaskCount())
	}

	// Verify AddTask is rejected after close
	err = sess.AddTask(&session.BackgroundTask{ID: "late", Done: make(chan struct{})})
	if err == nil {
		t.Error("expected error adding task to closed session")
	}
}

// TestHTTPNoAuthWhenNoToken verifies that without a token, /mcp is accessible
// without authentication.
func TestHTTPNoAuthWhenNoToken(t *testing.T) {
	workdir := t.TempDir()
	cfg := testServerConfig(t, workdir)
	srv := newTestHTTPServerWithMux(t, cfg, "")

	// /mcp without auth → should not be 401
	resp, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Errorf("/mcp without token configured should not be 401, got %d", resp.StatusCode)
	}

	// /health → 200
	resp2, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("/health: status = %d, want 200", resp2.StatusCode)
	}
}
