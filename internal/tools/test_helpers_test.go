package tools

import (
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
