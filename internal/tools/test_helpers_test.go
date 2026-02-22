package tools

import (
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// resultText extracts the text from a CallToolResult's first content block.
func resultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}

// isErrorResult returns true if the CallToolResult has IsError set.
func isErrorResult(r *mcp.CallToolResult) bool {
	return r != nil && r.IsError
}

// hasErrorCode returns true if the result is an error with the given code prefix.
func hasErrorCode(r *mcp.CallToolResult, code string) bool {
	return isErrorResult(r) && strings.HasPrefix(resultText(r), "["+code+"]")
}

// testConfig returns a Config suitable for testing.
func testConfig() Config {
	return Config{
		Shell:          "/bin/sh",
		DefaultTimeout: 120,
		MaxFileSize:    10 * 1024 * 1024,
	}
}
