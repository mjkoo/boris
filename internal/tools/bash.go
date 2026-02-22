package tools

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mjkoo/boris/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxOutputChars = 30000

// BashArgs is the input schema for the bash tool.
type BashArgs struct {
	Command         string `json:"command" jsonschema:"the shell command to execute"`
	Timeout         int    `json:"timeout,omitempty" jsonschema:"timeout in milliseconds (default 120000, max 600000)"`
	RunInBackground bool   `json:"run_in_background,omitempty" jsonschema:"run command in background, returns a task_id"`
	Description     string `json:"description,omitempty" jsonschema:"optional human-readable description of what this command does"`
}

func bashHandler(sess *session.Session, cfg Config) mcp.ToolHandlerFor[BashArgs, any] {
	// Convert CLI --timeout (seconds) to milliseconds for the default.
	defaultTimeoutMs := cfg.DefaultTimeout * 1000

	return func(ctx context.Context, req *mcp.CallToolRequest, args BashArgs) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.Command) == "" {
			return toolErr("command must not be empty")
		}

		timeoutMs := args.Timeout
		if timeoutMs <= 0 {
			timeoutMs = defaultTimeoutMs
		}
		if timeoutMs > 600000 {
			timeoutMs = 600000
		}

		cwd := sess.Cwd()
		sentinel := sess.Sentinel()

		if args.RunInBackground {
			return runBackground(sess, cfg, cwd, args.Command)
		}

		return runForeground(ctx, req, sess, cfg, cwd, sentinel, args.Command, timeoutMs)
	}
}

func runForeground(ctx context.Context, req *mcp.CallToolRequest, sess *session.Session, cfg Config, cwd, sentinel, command string, timeoutMs int) (*mcp.CallToolResult, any, error) {
	wrappedCmd := fmt.Sprintf("cd %s && %s ; echo ; echo '%s' ; pwd",
		shellQuote(cwd), command, sentinel)

	cmd := exec.Command(cfg.Shell, "-c", wrappedCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Use pipes for streaming output
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return toolErr("failed to create stdout pipe: %v", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return toolErr("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return toolErr("failed to start command: %v", err)
	}

	pgid := cmd.Process.Pid
	var timedOut atomic.Bool
	timer := time.AfterFunc(time.Duration(timeoutMs)*time.Millisecond, func() {
		timedOut.Store(true)
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		time.AfterFunc(5*time.Second, func() {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		})
	})

	// Collect output via scanners, sending progress notifications
	var stdout, stderr bytes.Buffer
	var progressToken any
	if req != nil && req.Params != nil {
		progressToken = req.Params.GetProgressToken()
	}
	var lineCount atomic.Int64

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanAndNotify(ctx, req, stdoutPipe, &stdout, progressToken, &lineCount)
	}()
	go func() {
		defer wg.Done()
		scanAndNotify(ctx, req, stderrPipe, &stderr, progressToken, &lineCount)
	}()
	wg.Wait()

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

	// Parse sentinel from stdout to extract new cwd (before truncation)
	stdoutStr = parseSentinel(stdoutStr, sentinel, sess)

	// Truncate output
	stdoutStr = truncateOutput(stdoutStr)
	stderrStr = truncateOutput(stderrStr)

	// Build response
	var result strings.Builder
	if timedOut.Load() {
		fmt.Fprintf(&result, "Command timed out after %dms\n\n", timeoutMs)
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

// scanAndNotify reads from r line by line, writing to buf and optionally
// sending progress notifications for each line.
func scanAndNotify(ctx context.Context, req *mcp.CallToolRequest, r io.Reader, buf *bytes.Buffer, progressToken any, lineCount *atomic.Int64) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		buf.WriteByte('\n')

		if progressToken != nil && req.Session != nil {
			n := lineCount.Add(1)
			_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
				ProgressToken: progressToken,
				Progress:      float64(n),
				Message:       line,
			})
		}
	}
}

func runBackground(sess *session.Session, cfg Config, cwd, command string) (*mcp.CallToolResult, any, error) {
	// Generate a unique task ID
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return toolErr("failed to generate task ID: %v", err)
	}
	taskID := hex.EncodeToString(b)

	// No sentinel wrapping for background commands — they don't update cwd
	wrappedCmd := fmt.Sprintf("cd %s && %s", shellQuote(cwd), command)

	cmd := exec.Command(cfg.Shell, "-c", wrappedCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdoutBuf := &session.SyncBuffer{}
	stderrBuf := &session.SyncBuffer{}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		return toolErr("failed to start background command: %v", err)
	}

	task := &session.BackgroundTask{
		ID:     taskID,
		Cmd:    cmd,
		Stdout: stdoutBuf,
		Stderr: stderrBuf,
		Done:   make(chan struct{}),
	}

	if err := sess.AddTask(task); err != nil {
		// Kill the process we just started since we can't track it
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
		return toolErr("%v", err)
	}

	// Wait for completion in background goroutine
	go func() {
		defer close(task.Done)
		waitErr := cmd.Wait()
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				task.ExitCode = exitErr.ExitCode()
			}
		}
	}()

	text := fmt.Sprintf("task_id: %s\nCommand started in background.", taskID)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

// TaskOutputArgs is the input schema for the task_output tool.
type TaskOutputArgs struct {
	TaskID string `json:"task_id" jsonschema:"the task ID returned by a background bash command"`
}

func taskOutputHandler(sess *session.Session) mcp.ToolHandlerFor[TaskOutputArgs, any] {
	return func(_ context.Context, _ *mcp.CallToolRequest, args TaskOutputArgs) (*mcp.CallToolResult, any, error) {
		task, ok := sess.GetTask(args.TaskID)
		if !ok {
			return toolErr("task not found: %s", args.TaskID)
		}

		var result strings.Builder
		select {
		case <-task.Done:
			// Task completed
			stdoutStr := truncateOutput(task.Stdout.String())
			stderrStr := truncateOutput(task.Stderr.String())

			fmt.Fprintf(&result, "status: completed\nexit_code: %d\n", task.ExitCode)
			if stderrStr != "" {
				fmt.Fprintf(&result, "\nstderr:\n%s", stderrStr)
			}
			if stdoutStr != "" {
				fmt.Fprintf(&result, "\nstdout:\n%s", stdoutStr)
			}

			// Single-read semantics: clean up after retrieval
			sess.RemoveTask(args.TaskID)
		default:
			// Task still running
			stdoutStr := truncateOutput(task.Stdout.String())
			stderrStr := truncateOutput(task.Stderr.String())

			fmt.Fprintf(&result, "status: running\n")
			if stderrStr != "" {
				fmt.Fprintf(&result, "\nstderr:\n%s", stderrStr)
			}
			if stdoutStr != "" {
				fmt.Fprintf(&result, "\nstdout:\n%s", stdoutStr)
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result.String()}},
		}, nil, nil
	}
}

// parseSentinel finds the cwd sentinel in stdout, extracts the new working
// directory, updates the session, and returns stdout with sentinel lines stripped.
func parseSentinel(stdout, sentinel string, sess *session.Session) string {
	lines := strings.Split(stdout, "\n")

	sentinelIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] == sentinel {
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

// truncateOutput caps output at maxOutputChars characters.
func truncateOutput(s string) string {
	if len(s) <= maxOutputChars {
		return s
	}
	return s[:maxOutputChars] + fmt.Sprintf("\n\n[Truncated: output was %d characters, showing first %d]", len(s), maxOutputChars)
}

// shellQuote wraps a string in single quotes for safe shell embedding.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
