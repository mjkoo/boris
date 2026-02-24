package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/mjkoo/boris/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev" // overridden by -ldflags "-X main.version=..."

func versionInfo() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var revision string
	var modified bool
	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			revision = kv.Value
		case "vcs.modified":
			modified = kv.Value == "true"
		}
	}
	if revision == "" {
		return "dev"
	}
	v := "dev-" + revision[:min(12, len(revision))]
	if modified {
		v += "-dirty"
	}
	return v
}

// VersionFlag implements kong's BeforeApply hook to print version and exit.
type VersionFlag bool

func (v VersionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	fmt.Println(vars["version"])
	app.Exit(0)
	return nil
}

// CLI defines the command-line interface via kong struct tags.
type CLI struct {
	Version     VersionFlag `help:"Print version and exit." short:"v"`
	Port        int         `help:"Listen port (HTTP mode)." default:"8080" env:"BORIS_PORT"`
	Transport   string      `help:"Transport: http or stdio." default:"http" enum:"http,stdio" env:"BORIS_TRANSPORT"`
	Workdir     string      `help:"Initial working directory." default:"." env:"BORIS_WORKDIR"`
	Timeout     int         `help:"Default bash timeout in seconds." default:"120" env:"BORIS_TIMEOUT"`
	AllowDir    []string    `help:"Allowed directories (repeatable)." env:"BORIS_ALLOW_DIRS"`
	DenyDir     []string    `help:"Denied directories/patterns (repeatable)." env:"BORIS_DENY_DIRS"`
	Token           string      `help:"Bearer token for HTTP authentication." env:"BORIS_TOKEN"`
	GenerateToken   bool        `help:"Generate a random bearer token on startup." env:"BORIS_GENERATE_TOKEN"`
	DisableTools    []string    `help:"Tools to disable (repeatable)." env:"BORIS_DISABLE_TOOLS"`
	BackgroundTaskTimeout int   `help:"Background task safety-net timeout in seconds (0=disabled)." default:"0" env:"BORIS_BACKGROUND_TASK_TIMEOUT"`
	MaxFileSize     string      `help:"Max file size for view/create." default:"10MB" env:"BORIS_MAX_FILE_SIZE"`
	RequireViewBeforeEdit string `help:"Require files to be viewed before editing: auto, true, false." default:"auto" enum:"auto,true,false" env:"BORIS_REQUIRE_VIEW_BEFORE_EDIT"`
	AnthropicCompat bool        `help:"Expose combined str_replace_editor tool schema." env:"BORIS_ANTHROPIC_COMPAT"`
	LogLevel        string      `help:"Log level: debug, info, warn, error." default:"info" enum:"debug,info,warn,error" env:"BORIS_LOG_LEVEL"`
	LogFormat       string      `help:"Log format: text or json." default:"text" enum:"text,json" env:"BORIS_LOG_FORMAT"`
}

// Validate is called by kong after parsing to enforce flag constraints.
func (c *CLI) Validate() error {
	if c.Token != "" && c.GenerateToken {
		return fmt.Errorf("--token and --generate-token are mutually exclusive")
	}
	return nil
}

// serverConfig holds shared immutable values computed at startup.
// The getServer factory closure captures this struct and creates
// per-connection mcp.Server and session.Session instances.
type serverConfig struct {
	workdir    string
	resolver   *pathscope.Resolver
	impl       *mcp.Implementation
	toolsCfg   tools.Config
	serverOpts *mcp.ServerOptions
}

// generateToken returns a cryptographically random 64-character hex string
// (32 bytes of entropy).
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// bearerAuthMiddleware returns middleware that requires a valid
// Authorization: Bearer <token> header. Unauthenticated requests
// receive a 401 JSON response.
func bearerAuthMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"}); err != nil {
				slog.Debug("failed to write auth error response", "error", err)
			}
			return
		}
		provided := auth[len(prefix):]
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"}); err != nil {
				slog.Debug("failed to write auth error response", "error", err)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// parseLogLevel converts a log level string to a slog.Level.
func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", s)
	}
}

// buildInstructions creates the MCP server instructions string from
// the working directory and path scoping configuration.
func buildInstructions(workdir string, resolver *pathscope.Resolver) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Working directory: %s", workdir)
	if dirs := resolver.AllowDirs(); len(dirs) > 0 {
		fmt.Fprintf(&b, "\nAllowed directories: %s", strings.Join(dirs, ", "))
	}
	if patterns := resolver.DenyPatterns(); len(patterns) > 0 {
		fmt.Fprintf(&b, "\nDenied patterns: %s", strings.Join(patterns, ", "))
	}
	return b.String()
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("boris"),
		kong.Description("Coding agent tools as a MCP server."),
		kong.Vars{"version": versionInfo()},
	)

	// Initialize structured logging
	logLevel, err := parseLogLevel(cli.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --log-level: %v\n", err)
		os.Exit(1)
	}
	var logHandler slog.Handler
	opts := &slog.HandlerOptions{Level: logLevel}
	switch cli.LogFormat {
	case "json":
		logHandler = slog.NewJSONHandler(os.Stderr, opts)
	default:
		logHandler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(logHandler))

	maxFileSize, err := parseSize(cli.MaxFileSize)
	if err != nil {
		slog.Error("invalid --max-file-size", "error", err)
		os.Exit(1)
	}

	// Resolve workdir
	workdir, err := filepath.Abs(cli.Workdir)
	if err != nil {
		slog.Error("invalid --workdir", "error", err)
		os.Exit(1)
	}
	workdir, err = filepath.EvalSymlinks(workdir)
	if err != nil {
		slog.Error("invalid --workdir", "error", err)
		os.Exit(1)
	}

	// Detect shell
	shell := "/bin/sh"
	if _, err := os.Stat("/bin/bash"); err == nil {
		shell = "/bin/bash"
	}
	slog.Info("using shell", "shell", shell)

	// Create path resolver
	resolver, err := pathscope.NewResolver(cli.AllowDir, cli.DenyDir)
	if err != nil {
		slog.Error("invalid path scoping config", "error", err)
		os.Exit(1)
	}

	// Build DisableTools set from CLI flag
	disableTools := make(map[string]struct{}, len(cli.DisableTools))
	for _, name := range cli.DisableTools {
		disableTools[name] = struct{}{}
	}
	if err := tools.ValidateDisableTools(disableTools, cli.AnthropicCompat); err != nil {
		slog.Error("invalid --disable-tools", "error", err)
		os.Exit(1)
	}

	// Resolve --require-view-before-edit: "auto" → true
	requireViewBeforeEdit := cli.RequireViewBeforeEdit == "true" || cli.RequireViewBeforeEdit == "auto"

	cfg := serverConfig{
		workdir:  workdir,
		resolver: resolver,
		impl: &mcp.Implementation{
			Name:    "boris",
			Version: versionInfo(),
		},
		toolsCfg: tools.Config{
			DisableTools:          disableTools,
			MaxFileSize:           maxFileSize,
			DefaultTimeout:        cli.Timeout,
			Shell:                 shell,
			AnthropicCompat:       cli.AnthropicCompat,
			BackgroundTaskTimeout: cli.BackgroundTaskTimeout,
			RequireViewBeforeEdit: requireViewBeforeEdit,
		},
		serverOpts: &mcp.ServerOptions{
			Instructions: buildInstructions(workdir, resolver),
		},
	}

	// Resolve bearer token
	var token string
	switch {
	case cli.Token != "":
		token = cli.Token
	case cli.GenerateToken:
		var err error
		token, err = generateToken()
		if err != nil {
			slog.Error("failed to generate token", "error", err)
			os.Exit(1)
		}
		slog.Info("generated bearer token", "token", token)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch cli.Transport {
	case "http":
		runHTTP(ctx, cfg, cli.Port, token)
	case "stdio":
		runSTDIO(ctx, cfg)
	}
}

// corsMiddleware adds permissive CORS headers for browser-based MCP clients.
// Non-browser clients ignore these headers, so there's no downside.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Session-Id")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// buildMux creates the HTTP mux with /mcp and /health routes.
func buildMux(mcpHandler http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Debug("failed to write health response", "error", err)
		}
	})
	return mux
}

func runHTTP(ctx context.Context, cfg serverConfig, port int, token string) {
	registry := session.NewRegistry()
	store := &session.SessionCleanupStore{Registry: registry}

	var mcpHandler http.Handler = mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		server := mcp.NewServer(cfg.impl, cfg.serverOpts)
		sess := session.New(cfg.workdir)
		toolsCfg := cfg.toolsCfg
		toolsCfg.RegisterSession = func(sessionID string) {
			registry.Register(sessionID, sess)
		}
		tools.RegisterAll(server, cfg.resolver, sess, toolsCfg)
		return server
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout: 10 * time.Minute,
		EventStore:     store,
	})

	if token != "" {
		mcpHandler = bearerAuthMiddleware(token, mcpHandler)
	}
	mux := buildMux(mcpHandler)

	addr := fmt.Sprintf(":%d", port)
	slog.Info("boris listening", "addr", addr, "transport", "http")

	srv := &http.Server{Addr: addr, Handler: corsMiddleware(mux)}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown error", "error", err)
		}
		// Clean up any sessions not yet closed by the SDK, killing orphan
		// background processes that would otherwise survive server shutdown.
		registry.CloseAll()
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func runSTDIO(ctx context.Context, cfg serverConfig) {
	slog.Info("boris running", "transport", "stdio")

	server := mcp.NewServer(cfg.impl, cfg.serverOpts)
	sess := session.New(cfg.workdir)
	defer sess.Close()
	tools.RegisterAll(server, cfg.resolver, sess, cfg.toolsCfg)

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

// parseSize parses a human-readable size string (e.g., "10MB", "1GB").
func parseSize(s string) (int64, error) {
	upper := strings.ToUpper(strings.TrimSpace(s))
	var multiplier int64 = 1
	switch {
	case strings.HasSuffix(upper, "GB"):
		multiplier = 1024 * 1024 * 1024
		upper = strings.TrimSuffix(upper, "GB")
	case strings.HasSuffix(upper, "MB"):
		multiplier = 1024 * 1024
		upper = strings.TrimSuffix(upper, "MB")
	case strings.HasSuffix(upper, "KB"):
		multiplier = 1024
		upper = strings.TrimSuffix(upper, "KB")
	case strings.HasSuffix(upper, "B"):
		upper = strings.TrimSuffix(upper, "B")
	}
	var val int64
	if _, err := fmt.Sscanf(upper, "%d", &val); err != nil {
		return 0, fmt.Errorf("cannot parse %q as size", s)
	}
	return val * multiplier, nil
}
