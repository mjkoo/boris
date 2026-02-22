package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const cwdSentinel = "__BORIS_CWD__"

// BashArgs is the input schema for the bash tool.
type BashArgs struct {
	Command string `json:"command" jsonschema:"the shell command to execute"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"timeout in seconds"`
}

func bashHandler(sess *session.Session, defaultTimeout int) mcp.ToolHandlerFor[BashArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args BashArgs) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Command) == "" {
			return nil, nil, fmt.Errorf("command must not be empty")
		}

		timeout := args.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}

		cwd := sess.Cwd()
		// Wrap command with sentinel for cwd tracking.
		// Extra `echo` before sentinel ensures it starts on its own line.
		wrappedCmd := fmt.Sprintf("cd %s && %s ; echo ; echo '%s' ; pwd",
			shellQuote(cwd), args.Command, cwdSentinel)

		cmd := exec.Command("/bin/sh", "-c", wrappedCmd)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			return nil, nil, fmt.Errorf("failed to start command: %w", err)
		}

		pgid := cmd.Process.Pid
		var timedOut atomic.Bool
		timer := time.AfterFunc(time.Duration(timeout)*time.Second, func() {
			timedOut.Store(true)
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		})

		waitErr := cmd.Wait()
		timer.Stop()

		exitCode := 0
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}

		stdoutStr := stdout.String()
		stderrStr := stderr.String()

		// Parse sentinel from stdout to extract new cwd
		stdoutStr = parseSentinel(stdoutStr, sess)

		// Build response
		var result strings.Builder
		if timedOut.Load() {
			fmt.Fprintf(&result, "Command timed out after %d seconds\n\n", timeout)
		}
		fmt.Fprintf(&result, "exit_code: %d\n", exitCode)
		if stderrStr != "" {
			fmt.Fprintf(&result, "\nstderr:\n%s", stderrStr)
		}
		if stdoutStr != "" {
			fmt.Fprintf(&result, "\nstdout:\n%s", stdoutStr)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result.String()}},
		}, nil, nil
	}
}

// parseSentinel finds the cwd sentinel in stdout, extracts the new working
// directory, updates the session, and returns stdout with sentinel lines stripped.
func parseSentinel(stdout string, sess *session.Session) string {
	lines := strings.Split(stdout, "\n")

	sentinelIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] == cwdSentinel {
			sentinelIdx = i
			break
		}
	}

	if sentinelIdx < 0 {
		return stdout
	}

	// The line after sentinel is the pwd output
	if sentinelIdx+1 < len(lines) {
		newCwd := strings.TrimSpace(lines[sentinelIdx+1])
		if newCwd != "" {
			sess.SetCwd(newCwd)
		}
	}

	// Reconstruct output: everything before sentinel, excluding the
	// blank line we added before the sentinel echo.
	outputLines := lines[:sentinelIdx]
	// Remove trailing empty line from our extra `echo`
	for len(outputLines) > 0 && outputLines[len(outputLines)-1] == "" {
		outputLines = outputLines[:len(outputLines)-1]
	}

	if len(outputLines) == 0 {
		return ""
	}
	return strings.Join(outputLines, "\n") + "\n"
}

// shellQuote wraps a string in single quotes for safe shell embedding.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
