package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mjkoo/boris/internal/pathscope"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"10MB", 10 * 1024 * 1024},
		{"10mb", 10 * 1024 * 1024},
		{"10Mb", 10 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"512KB", 512 * 1024},
		{"100B", 100},
		{"4096", 4096},
		{"  10MB  ", 10 * 1024 * 1024},
		{"0MB", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseSize(tt.input)
			if err != nil {
				t.Fatalf("parseSize(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCLIValidate(t *testing.T) {
	tests := []struct {
		name    string
		cli     CLI
		wantErr bool
	}{
		{
			name:    "neither flag",
			cli:     CLI{},
			wantErr: false,
		},
		{
			name:    "token only",
			cli:     CLI{Token: "secret"},
			wantErr: false,
		},
		{
			name:    "generate-token only",
			cli:     CLI{GenerateToken: true},
			wantErr: false,
		},
		{
			name:    "both flags error",
			cli:     CLI{Token: "secret", GenerateToken: true},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cli.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	tok, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() error: %v", err)
	}
	if len(tok) != 64 {
		t.Errorf("expected 64-char token, got %d chars", len(tok))
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(tok) {
		t.Errorf("token is not valid hex: %q", tok)
	}

	// Uniqueness: two calls should produce different tokens
	tok2, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() second call error: %v", err)
	}
	if tok == tok2 {
		t.Error("two generateToken() calls produced identical tokens")
	}
}

func TestBearerAuthMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := bearerAuthMiddleware("test-token", inner)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"valid token", "Bearer test-token", http.StatusOK},
		{"missing header", "", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong-token", http.StatusUnauthorized},
		{"malformed scheme", "Basic dXNlcjpwYXNz", http.StatusUnauthorized},
		{"lowercase bearer", "bearer test-token", http.StatusOK},
		{"uppercase BEARER", "BEARER test-token", http.StatusOK},
		{"mixed case bEaReR", "bEaReR test-token", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/mcp", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusUnauthorized {
				var body map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode 401 body: %v", err)
				}
				if body["error"] != "unauthorized" {
					t.Errorf("error body = %q, want %q", body["error"], "unauthorized")
				}
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"DEBUG", slog.LevelDebug},
		{"Info", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseLogLevel(tt.input)
			if err != nil {
				t.Fatalf("parseLogLevel(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}

	t.Run("invalid", func(t *testing.T) {
		_, err := parseLogLevel("verbose")
		if err == nil {
			t.Error("parseLogLevel(\"verbose\") expected error, got nil")
		}
	})
}

func TestCORSPreflightReturns204(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := corsMiddleware(inner)

	req := httptest.NewRequest("OPTIONS", "/mcp", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO = %q, want *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods not set")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("Access-Control-Allow-Headers not set")
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "Mcp-Session-Id" {
		t.Errorf("Access-Control-Expose-Headers = %q, want Mcp-Session-Id", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("Access-Control-Max-Age = %q, want 86400", got)
	}
}

func TestCORSHeadersOnNormalRequest(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := corsMiddleware(inner)

	req := httptest.NewRequest("POST", "/mcp", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO = %q, want * on normal request", got)
	}
}

func TestCORSPreflightNotBlockedByAuth(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply auth inside, CORS outside (same order as production)
	handler := bearerAuthMiddleware("secret-token", inner)
	handler = corsMiddleware(handler)

	req := httptest.NewRequest("OPTIONS", "/mcp", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Preflight should succeed (204), not be blocked by auth (401)
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight with auth middleware: status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestHealthEndpointGetsCORSHeaders(t *testing.T) {
	mux := buildMux(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler := corsMiddleware(mux)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("/health ACAO = %q, want *", got)
	}
}

func TestGracefulShutdown(t *testing.T) {
	mux := buildMux(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Pick a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{Addr: addr, Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
		close(shutdownDone)
	}()

	go srv.ListenAndServe()

	// Wait for server to be ready
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Trigger shutdown
	cancel()
	select {
	case <-shutdownDone:
		// Shutdown completed cleanly
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not complete within timeout")
	}

	// New connections should be refused after shutdown
	_, err = http.Get(fmt.Sprintf("http://%s/health", addr))
	if err == nil {
		t.Error("expected connection refused after shutdown")
	}
}

func TestBuildInstructions(t *testing.T) {
	t.Run("workdir only", func(t *testing.T) {
		r, err := pathscope.NewResolver(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		got := buildInstructions("/workspace", r)
		want := "Working directory: /workspace"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("workdir + allow dirs", func(t *testing.T) {
		tmp1 := t.TempDir()
		tmp2 := t.TempDir()
		r, err := pathscope.NewResolver([]string{tmp1, tmp2}, nil)
		if err != nil {
			t.Fatal(err)
		}
		got := buildInstructions("/workspace", r)
		wantPrefix := "Working directory: /workspace\nAllowed directories: "
		if !strings.HasPrefix(got, wantPrefix) {
			t.Errorf("got %q, want prefix %q", got, wantPrefix)
		}
		if strings.Contains(got, "Denied patterns") {
			t.Error("should not contain Denied patterns line")
		}
	})

	t.Run("workdir + deny patterns", func(t *testing.T) {
		r, err := pathscope.NewResolver(nil, []string{"**/.env", "**/.git"})
		if err != nil {
			t.Fatal(err)
		}
		got := buildInstructions("/workspace", r)
		want := "Working directory: /workspace\nDenied patterns: **/.env, **/.git"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("all three", func(t *testing.T) {
		tmp := t.TempDir()
		r, err := pathscope.NewResolver([]string{tmp}, []string{"**/.env"})
		if err != nil {
			t.Fatal(err)
		}
		got := buildInstructions("/workspace", r)
		if !strings.HasPrefix(got, "Working directory: /workspace\n") {
			t.Errorf("missing workdir line: %q", got)
		}
		if !strings.Contains(got, "Allowed directories: ") {
			t.Error("missing allowed directories line")
		}
		if !strings.Contains(got, "Denied patterns: **/.env") {
			t.Error("missing denied patterns line")
		}
	})
}

func TestParseSizeErrors(t *testing.T) {
	tests := []string{
		"",
		"MB",
		"abc",
		"ten megabytes",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseSize(input)
			if err == nil {
				t.Errorf("parseSize(%q) expected error, got nil", input)
			}
		})
	}
}
