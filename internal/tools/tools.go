package tools

import (
	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config holds configuration for tool registration.
type Config struct {
	NoBash         bool
	MaxFileSize    int64
	DefaultTimeout int
}

// RegisterAll registers all tools with the MCP server.
func RegisterAll(server *mcp.Server, resolver *pathscope.Resolver, sess *session.Session, cfg Config) {
	if !cfg.NoBash {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "bash",
			Description: "Execute a shell command. Commands run in a persistent session that tracks the working directory across calls.",
		}, bashHandler(sess, cfg.DefaultTimeout))
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "view",
		Description: "Read a file with line numbers, or list a directory (2 levels deep). Supports line ranges for large files.",
	}, viewHandler(sess, resolver, cfg.MaxFileSize))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "str_replace",
		Description: "Replace a unique string in a file. The old_str must appear exactly once.",
	}, strReplaceHandler(sess, resolver))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_file",
		Description: "Create a new file or overwrite an existing one. Creates parent directories as needed.",
	}, createFileHandler(sess, resolver, cfg.MaxFileSize))
}
