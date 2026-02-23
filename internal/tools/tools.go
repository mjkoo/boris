package tools

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mjkoo/boris/internal/pathscope"
	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Error code constants for structured error responses.
// Cross-tool codes
const (
	ErrInvalidInput = "INVALID_INPUT"
	ErrPathNotFound = "PATH_NOT_FOUND"
	ErrAccessDenied = "ACCESS_DENIED"
	ErrFileTooLarge = "FILE_TOO_LARGE"
	ErrIO           = "IO_ERROR"
)

// Bash tool codes
const (
	ErrBashEmptyCommand = "BASH_EMPTY_COMMAND"
	ErrBashStartFailed  = "BASH_START_FAILED"
	ErrBashTaskLimit    = "BASH_TASK_LIMIT"
	ErrBashTaskNotFound = "BASH_TASK_NOT_FOUND"
)

// Str_replace tool codes
const (
	ErrStrReplaceNotFound  = "STR_REPLACE_NOT_FOUND"
	ErrStrReplaceAmbiguous = "STR_REPLACE_AMBIGUOUS"
)

// Grep tool codes
const (
	ErrGrepInvalidPattern    = "GREP_INVALID_PATTERN"
	ErrGrepInvalidOutputMode = "GREP_INVALID_OUTPUT_MODE"
)

// Find tool codes
const (
	ErrFindInvalidPattern = "FIND_INVALID_PATTERN"
	ErrFindInvalidType    = "FIND_INVALID_TYPE"
)

// typeSchemas provides custom JSON schema mappings for named types.
var typeSchemas = map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[EditorCommand](): {
		Type: "string",
		Enum: []any{EditorCommandView, EditorCommandStrReplace, EditorCommandCreate},
	},
	reflect.TypeFor[ViewRange](): {
		Type:  "array",
		Items: &jsonschema.Schema{Type: "integer"},
	},
}

// toolErr returns a CallToolResult with IsError set to true.
// Use this for operational errors (file not found, invalid input, etc.)
// instead of returning Go errors, which are reserved for infrastructure failures.
// The code parameter must be one of the Err* constants defined above.
func toolErr(code string, msg string, args ...any) (*mcp.CallToolResult, any, error) {
	r := &mcp.CallToolResult{}
	text := fmt.Sprintf("[%s] %s", code, fmt.Sprintf(msg, args...))
	r.SetError(errors.New(text))
	return r, nil, nil
}

// Config holds configuration for tool registration.
type Config struct {
	NoBash          bool
	MaxFileSize     int64
	DefaultTimeout  int
	Shell           string
	AnthropicCompat bool
}

// RegisterAll registers all tools with the MCP server.
func RegisterAll(server *mcp.Server, resolver *pathscope.Resolver, sess *session.Session, cfg Config) {
	if !cfg.NoBash {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "bash",
			Description: "Executes a bash command with optional timeout. The working directory persists between calls. When run_in_background is true, the command runs asynchronously and returns a task_id for later retrieval via task_output.",
		}, bashHandler(sess, cfg))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "task_output",
			Description: "Retrieve output from a running or completed background bash command by task_id. Running tasks return current output with status: running. Completed tasks return final output, exit code, and are cleaned up after retrieval.",
		}, taskOutputHandler(sess))
	}

	// Register grep tool (always available, both modes, even with --no-bash)
	if cfg.AnthropicCompat {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "grep",
			Description: "Search file contents using regex patterns. Returns matching file paths (sorted by modification time), matching lines with context, or match counts.",
		}, grepCompatHandler(sess, resolver, cfg.MaxFileSize))
	} else {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "grep",
			Description: "Search file contents using regex patterns. Returns matching file paths (sorted by modification time), matching lines with context, or match counts.",
		}, grepHandler(sess, resolver, cfg.MaxFileSize))
	}

	// Register find/Glob tool (always available, both modes, even with --no-bash)
	if cfg.AnthropicCompat {
		mcp.AddTool(server, &mcp.Tool{
			Name: "Glob",
			Description: `- Fast file pattern matching tool that works with any codebase size
- Supports glob patterns like "**/*.js" or "src/**/*.ts"
- Returns matching file paths sorted by modification time
- Use this tool when you need to find files by name patterns
- When you are doing an open ended search that may require multiple rounds of globbing and grepping, use the Agent tool instead
- You can call multiple tools in a single response. It is always better to speculatively perform multiple searches in parallel if they are potentially useful.`,
		}, findCompatHandler(sess, resolver))
	} else {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "find",
			Description: "Find files by glob pattern. Returns matching file paths sorted by modification time (newest first). Supports doublestar patterns, brace expansion, and character classes. Respects .gitignore and skips .git/node_modules.",
		}, findHandler(sess, resolver))
	}

	if cfg.AnthropicCompat {
		editorSchema, err := jsonschema.For[StrReplaceEditorArgs](&jsonschema.ForOptions{
			TypeSchemas: typeSchemas,
		})
		if err != nil {
			panic(fmt.Sprintf("failed to build str_replace_editor schema: %v", err))
		}
		mcp.AddTool(server, &mcp.Tool{
			Name:        "str_replace_editor",
			Description: "View, create, and edit files. Use the 'command' parameter to select the operation: 'view' to read files/directories, 'str_replace' to replace text, 'create' to create or overwrite files.",
			InputSchema: editorSchema,
		}, strReplaceEditorHandler(sess, resolver, cfg.MaxFileSize))
	} else {
		viewSchema, err := jsonschema.For[ViewArgs](&jsonschema.ForOptions{
			TypeSchemas: typeSchemas,
		})
		if err != nil {
			panic(fmt.Sprintf("failed to build view schema: %v", err))
		}
		mcp.AddTool(server, &mcp.Tool{
			Name:        "view",
			Description: "Read a file from the filesystem with line numbers, or list a directory (2 levels deep). Supports line ranges for large files. Returns images as inline content. Lines longer than 2000 characters are truncated.",
			InputSchema: viewSchema,
		}, viewHandler(sess, resolver, cfg.MaxFileSize))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "str_replace",
			Description: "Replace a unique string in a file. The old_str must appear exactly once unless replace_all is true. Omit new_str or set it to empty string to delete the matched text.",
		}, strReplaceHandler(sess, resolver))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "create_file",
			Description: "Create a new file or overwrite an existing one. Creates parent directories as needed.",
		}, createFileHandler(sess, resolver, cfg.MaxFileSize))
	}
}

// EditorCommand is the command type for the combined str_replace_editor tool.
type EditorCommand string

const (
	EditorCommandView       EditorCommand = "view"
	EditorCommandStrReplace EditorCommand = "str_replace"
	EditorCommandCreate     EditorCommand = "create"
)

// StrReplaceEditorArgs is the input schema for the combined str_replace_editor tool.
type StrReplaceEditorArgs struct {
	Command    EditorCommand `json:"command" jsonschema:"the operation to perform: view, str_replace, or create"`
	Path       string        `json:"path" jsonschema:"file path"`
	ViewRange  ViewRange     `json:"view_range,omitempty" jsonschema:"optional line range [start end] (1-indexed, for view command)"`
	OldStr     string        `json:"old_str,omitempty" jsonschema:"the string to find (for str_replace command)"`
	NewStr     string        `json:"new_str,omitempty" jsonschema:"replacement string (for str_replace command)"`
	ReplaceAll bool          `json:"replace_all,omitempty" jsonschema:"replace all occurrences (for str_replace command)"`
	FileText   string        `json:"file_text,omitempty" jsonschema:"file content (for create command)"`
}

func strReplaceEditorHandler(sess *session.Session, resolver *pathscope.Resolver, maxFileSize int64) mcp.ToolHandlerFor[StrReplaceEditorArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args StrReplaceEditorArgs) (*mcp.CallToolResult, any, error) {
		switch args.Command {
		case EditorCommandView:
			return doView(sess, resolver, maxFileSize, args.Path, args.ViewRange)
		case EditorCommandStrReplace:
			return doStrReplace(sess, resolver, args.Path, args.OldStr, args.NewStr, args.ReplaceAll)
		case EditorCommandCreate:
			return doCreateFile(sess, resolver, maxFileSize, args.Path, args.FileText)
		default:
			return toolErr(ErrInvalidInput, "unknown command: %s (valid commands: view, str_replace, create)", args.Command)
		}
	}
}
