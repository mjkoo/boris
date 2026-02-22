package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

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
	NoBash          bool        `help:"Disable bash tool." env:"BORIS_NO_BASH"`
	MaxFileSize     string      `help:"Max file size for view/create." default:"10MB" env:"BORIS_MAX_FILE_SIZE"`
	AnthropicCompat bool        `help:"Expose combined str_replace_editor tool schema." env:"BORIS_ANTHROPIC_COMPAT"`
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
	workdir  string
	resolver *pathscope.Resolver
	impl     *mcp.Implementation
	toolsCfg tools.Config
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
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		provided := auth[len(prefix):]
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("boris"),
		kong.Description("Coding agent tools as a MCP server."),
		kong.Vars{"version": versionInfo()},
	)

	maxFileSize, err := parseSize(cli.MaxFileSize)
	if err != nil {
		log.Fatalf("invalid --max-file-size: %v", err)
	}

	// Resolve workdir
	workdir, err := filepath.Abs(cli.Workdir)
	if err != nil {
		log.Fatalf("invalid --workdir: %v", err)
	}
	workdir, err = filepath.EvalSymlinks(workdir)
	if err != nil {
		log.Fatalf("invalid --workdir: %v", err)
	}

	// Detect shell
	shell := "/bin/sh"
	if _, err := os.Stat("/bin/bash"); err == nil {
		shell = "/bin/bash"
	}
	log.Printf("using shell: %s", shell)

	// Create path resolver
	resolver, err := pathscope.NewResolver(cli.AllowDir, cli.DenyDir)
	if err != nil {
		log.Fatalf("invalid path scoping config: %v", err)
	}

	cfg := serverConfig{
		workdir:  workdir,
		resolver: resolver,
		impl: &mcp.Implementation{
			Name:    "boris",
			Version: versionInfo(),
		},
		toolsCfg: tools.Config{
			NoBash:          cli.NoBash,
			MaxFileSize:     maxFileSize,
			DefaultTimeout:  cli.Timeout,
			Shell:           shell,
			AnthropicCompat: cli.AnthropicCompat,
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
			log.Fatalf("failed to generate token: %v", err)
		}
		fmt.Fprintf(os.Stderr, "bearer token: %s\n", token)
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

// buildMux creates the HTTP mux with /mcp and /health routes.
func buildMux(mcpHandler http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	return mux
}

func runHTTP(ctx context.Context, cfg serverConfig, port int, token string) {
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

	addr := fmt.Sprintf(":%d", port)
	log.Printf("boris listening on %s (HTTP)", addr)

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func runSTDIO(ctx context.Context, cfg serverConfig) {
	log.SetOutput(os.Stderr) // Keep stdout clean for MCP
	log.Println("boris running on stdio")

	server := mcp.NewServer(cfg.impl, nil)
	sess := session.New(cfg.workdir)
	tools.RegisterAll(server, cfg.resolver, sess, cfg.toolsCfg)

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
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
