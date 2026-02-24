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

// View-before-edit codes
const (
	ErrFileNotViewed = "FILE_NOT_VIEWED"
)

// Grep tool codes
const (
	ErrGrepInvalidPattern    = "GREP_INVALID_PATTERN"
	ErrGrepInvalidOutputMode = "GREP_INVALID_OUTPUT_MODE"
)

// Glob tool codes
const (
	ErrGlobInvalidPattern = "GLOB_INVALID_PATTERN"
	ErrGlobInvalidType    = "GLOB_INVALID_TYPE"
)

// standardToolNames lists the MCP tool names available in standard mode.
var standardToolNames = map[string]struct{}{
	"bash":        {},
	"task_output": {},
	"view":        {},
	"str_replace": {},
	"create_file": {},
	"grep":        {},
	"glob":        {},
}

// anthropicToolNames lists the MCP tool names available in anthropic-compat mode.
var anthropicToolNames = map[string]struct{}{
	"bash":               {},
	"task_output":        {},
	"str_replace_editor": {},
	"grep":               {},
	"glob":               {},
}

// ValidateDisableTools checks that all tool names in the set are valid for the given mode.
func ValidateDisableTools(names map[string]struct{}, anthropicCompat bool) error {
	valid := standardToolNames
	if anthropicCompat {
		valid = anthropicToolNames
	}
	// In anthropic-compat mode, also accept the standard file tool names since
	// they map to str_replace_editor.
	for name := range names {
		if _, ok := valid[name]; ok {
			continue
		}
		if anthropicCompat {
			if _, ok := standardToolNames[name]; ok {
				continue
			}
		}
		validNames := make([]string, 0, len(valid))
		for n := range valid {
			validNames = append(validNames, n)
		}
		return fmt.Errorf("unknown tool name %q; valid tools: %v", name, validNames)
	}
	return nil
}

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
	DisableTools         map[string]struct{}
	MaxFileSize          int64
	DefaultTimeout       int
	Shell                string
	AnthropicCompat      bool
	BackgroundTaskTimeout int // background task safety-net timeout in seconds (0 = disabled)
	RequireViewBeforeEdit bool

	// RegisterSession is called on first bash/task_output invocation with the
	// SDK session ID. In HTTP mode this registers the Boris session in the
	// SessionRegistry for lifecycle cleanup. Nil in STDIO mode.
	RegisterSession func(sessionID string)
}

// toolDisabled reports whether the given tool name is in the DisableTools set.
func toolDisabled(cfg Config, name string) bool {
	if cfg.DisableTools == nil {
		return false
	}
	_, ok := cfg.DisableTools[name]
	return ok
}

// RegisterAll registers all tools with the MCP server.
func RegisterAll(server *mcp.Server, resolver *pathscope.Resolver, sess *session.Session, cfg Config) {
	// Disabling bash also disables task_output
	if !toolDisabled(cfg, "bash") && !toolDisabled(cfg, "task_output") {
		bashDesc := "Executes a bash command with optional timeout. The working directory persists between calls. When run_in_background is true, the command runs asynchronously and returns a task_id for later retrieval via task_output."
		taskOutputDesc := "Retrieve output from a running or completed background bash command by task_id. Running tasks return current output with status: running. Completed tasks return final output, exit code, and are cleaned up after retrieval."
		if cfg.AnthropicCompat {
			bashDesc = `Executes a given bash command with optional timeout. Working directory persists between commands; shell state (everything else) does not. Timeout in milliseconds (default 120000, max 600000). Output truncated at 30000 characters.`

			taskOutputDesc = `Retrieves output from a running or completed background bash command. Takes a task_id returned by a background bash command. Running tasks return current output with status: running. Completed tasks return final output, exit code, and are cleaned up after retrieval.`
		}

		mcp.AddTool(server, &mcp.Tool{
			Name:        "bash",
			Description: bashDesc,
		}, bashHandler(sess, cfg))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "task_output",
			Description: taskOutputDesc,
		}, taskOutputHandler(sess, cfg))
	}

	if !toolDisabled(cfg, "grep") {
		if cfg.AnthropicCompat {
			mcp.AddTool(server, &mcp.Tool{
				Name: "grep",
				Description: `Search file contents using regex patterns. Supports full regex syntax.
- Filter files with glob parameter (e.g., "*.js", "**/*.tsx") or type parameter (e.g., "js", "py", "rust")
- Output modes: "content" shows matching lines, "files_with_matches" shows only file paths (default), "count" shows match counts
- Multiline matching: By default patterns match within single lines only. For cross-line patterns, use multiline: true`,
			}, grepCompatHandler(sess, resolver, cfg.MaxFileSize))
		} else {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "grep",
				Description: "Search file contents using regex patterns. Returns matching file paths (sorted by modification time), matching lines with context, or match counts.",
			}, grepHandler(sess, resolver, cfg.MaxFileSize))
		}
	}

	if !toolDisabled(cfg, "glob") {
		if cfg.AnthropicCompat {
			mcp.AddTool(server, &mcp.Tool{
				Name: "glob",
				Description: `- Fast file pattern matching tool that works with any codebase size
- Supports glob patterns like "**/*.js" or "src/**/*.ts"
- Returns matching file paths sorted by modification time
- Use this tool when you need to find files by name patterns`,
			}, globCompatHandler(sess, resolver))
		} else {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "glob",
				Description: "Find files by glob pattern. Returns matching file paths sorted by modification time (newest first). Supports doublestar patterns, brace expansion, and character classes. Respects .gitignore and skips .git/node_modules.",
			}, globHandler(sess, resolver))
		}
	}

	// In anthropic-compat mode, disabling any of view/str_replace/create_file
	// disables the combined str_replace_editor tool.
	if cfg.AnthropicCompat {
		editorDisabled := toolDisabled(cfg, "str_replace_editor") ||
			toolDisabled(cfg, "view") ||
			toolDisabled(cfg, "str_replace") ||
			toolDisabled(cfg, "create_file")
		if !editorDisabled {
			editorSchema, err := jsonschema.For[StrReplaceEditorArgs](&jsonschema.ForOptions{
				TypeSchemas: typeSchemas,
			})
			if err != nil {
				panic(fmt.Sprintf("failed to build str_replace_editor schema: %v", err))
			}
			mcp.AddTool(server, &mcp.Tool{
				Name: "str_replace_editor",
				Description: `View, create, and edit files. Commands:
- 'view': Read a file with line numbers, or list a directory. Supports optional view_range [start, end]. Lines longer than 2000 characters are truncated.
- 'str_replace': Replace a unique string in a file. old_str must appear exactly once unless replace_all is true. Omit new_str to delete.
- 'create': Create a new file or overwrite an existing one. Creates parent directories as needed.`,
				InputSchema: editorSchema,
			}, strReplaceEditorHandler(sess, resolver, cfg))
		}
	} else {
		if !toolDisabled(cfg, "view") {
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
			}, viewHandler(sess, resolver, cfg))
		}

		if !toolDisabled(cfg, "str_replace") {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "str_replace",
				Description: "Replace a unique string in a file. The old_str must appear exactly once unless replace_all is true. Omit new_str or set it to empty string to delete the matched text.",
			}, strReplaceHandler(sess, resolver, cfg))
		}

		if !toolDisabled(cfg, "create_file") {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "create_file",
				Description: "Create a new file or overwrite an existing one. Creates parent directories as needed.",
			}, createFileHandler(sess, resolver, cfg))
		}
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

func strReplaceEditorHandler(sess *session.Session, resolver *pathscope.Resolver, cfg Config) mcp.ToolHandlerFor[StrReplaceEditorArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args StrReplaceEditorArgs) (*mcp.CallToolResult, any, error) {
		switch args.Command {
		case EditorCommandView:
			return doView(sess, resolver, cfg, args.Path, args.ViewRange)
		case EditorCommandStrReplace:
			return doStrReplace(sess, resolver, cfg, args.Path, args.OldStr, args.NewStr, args.ReplaceAll)
		case EditorCommandCreate:
			return doCreateFile(sess, resolver, cfg, args.Path, args.FileText)
		default:
			return toolErr(ErrInvalidInput, "unknown command: %s (valid commands: view, str_replace, create)", args.Command)
		}
	}
}
