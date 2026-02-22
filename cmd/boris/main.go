package main

import (
	"context"
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
	NoBash          bool        `help:"Disable bash tool." env:"BORIS_NO_BASH"`
	MaxFileSize     string      `help:"Max file size for view/create." default:"10MB" env:"BORIS_MAX_FILE_SIZE"`
	AnthropicCompat bool        `help:"Expose combined str_replace_editor tool schema." env:"BORIS_ANTHROPIC_COMPAT"`
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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch cli.Transport {
	case "http":
		runHTTP(ctx, cfg, cli.Port)
	case "stdio":
		runSTDIO(ctx, cfg)
	}
}

func runHTTP(ctx context.Context, cfg serverConfig, port int) {
	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		server := mcp.NewServer(cfg.impl, nil)
		sess := session.New(cfg.workdir)
		tools.RegisterAll(server, cfg.resolver, sess, cfg.toolsCfg)
		return server
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

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
