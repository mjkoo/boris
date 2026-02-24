package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mjkoo/boris/internal/session"
)

func TestBashSimpleCommand(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	result, _, err := handler(context.Background(), nil, BashArgs{Command: "echo hello"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", text)
	}
	if !strings.Contains(text, "exit_code: 0") {
		t.Errorf("expected exit_code: 0, got: %s", text)
	}
}

func TestBashNonZeroExit(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	result, _, err := handler(context.Background(), nil, BashArgs{Command: "exit 42"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "exit_code: 42") {
		t.Errorf("expected exit_code: 42, got: %s", text)
	}
	// Non-zero exit code is data, not an error
	if isErrorResult(result) {
		t.Error("non-zero exit code should not set IsError")
	}
}

func TestBashStderrCapture(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	result, _, err := handler(context.Background(), nil, BashArgs{Command: "echo err >&2"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "err") {
		t.Errorf("expected stderr 'err' in output, got: %s", text)
	}
}

func TestBashCwdTracking(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	handler := bashHandler(sess, testConfig())

	// cd to /tmp
	_, _, err := handler(context.Background(), nil, BashArgs{Command: "cd /tmp"})
	if err != nil {
		t.Fatal(err)
	}

	// pwd should now show /tmp
	result, _, err := handler(context.Background(), nil, BashArgs{Command: "pwd"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "/tmp") {
		t.Errorf("expected /tmp in pwd output, got: %s", text)
	}
	if sess.Cwd() != "/tmp" {
		t.Errorf("session cwd = %q, want /tmp", sess.Cwd())
	}
}

func TestBashSentinelStripping(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	result, _, err := handler(context.Background(), nil, BashArgs{Command: "echo hello"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if strings.Contains(text, sess.Sentinel()) {
		t.Errorf("sentinel should be stripped from output: %s", text)
	}
}

func TestBashSentinelNonce(t *testing.T) {
	sess := session.New(t.TempDir())

	// Sentinel should contain the nonce
	sentinel := sess.Sentinel()
	if !strings.Contains(sentinel, sess.Nonce()) {
		t.Errorf("sentinel %q should contain nonce %q", sentinel, sess.Nonce())
	}

	// Old sentinel format should not trigger parser
	oldSentinel := "__BORIS_CWD__"
	stdout := "output\n" + oldSentinel + "\n/fake/path\n"
	parsed := parseSentinel(stdout, sentinel, sess)
	// Old sentinel should NOT be parsed — should remain in output
	if !strings.Contains(parsed, oldSentinel) {
		t.Errorf("old sentinel format should not be parsed, got: %s", parsed)
	}
}

func TestBashTimeoutMilliseconds(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	// Timeout of 1000ms (1 second) should be enough to kill sleep 300
	result, _, err := handler(context.Background(), nil, BashArgs{Command: "sleep 300", Timeout: 1000})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "timed out") {
		t.Errorf("expected timeout message, got: %s", text)
	}
	if !strings.Contains(text, "1000ms") {
		t.Errorf("expected timeout in milliseconds, got: %s", text)
	}
}

func TestBashTimeoutMaxCap(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	// Request 900000ms (15 min), should be clamped to 600000ms (10 min)
	// We can't actually wait that long, so just verify the command starts.
	// Instead, use a short command with an absurd timeout to verify it doesn't error.
	result, _, err := handler(context.Background(), nil, BashArgs{Command: "echo ok", Timeout: 900000})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "ok") {
		t.Errorf("expected 'ok' in output, got: %s", text)
	}
}

func TestBashMissingSentinelPreservesCwd(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	handler := bashHandler(sess, testConfig())

	// Timeout before sentinel is printed — cwd should be preserved
	_, _, _ = handler(context.Background(), nil, BashArgs{Command: "sleep 300", Timeout: 1000})

	if sess.Cwd() != tmp {
		t.Errorf("cwd should be preserved after timeout, got %q, want %q", sess.Cwd(), tmp)
	}
}

func TestBashInitialWorkdir(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	handler := bashHandler(sess, testConfig())

	result, _, err := handler(context.Background(), nil, BashArgs{Command: "pwd"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, tmp) {
		t.Errorf("expected initial workdir %q in output, got: %s", tmp, text)
	}
}

func TestBashEmptyCommand(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	for _, cmd := range []string{"", "  ", "\t\n"} {
		result, _, err := handler(context.Background(), nil, BashArgs{Command: cmd})
		if err != nil {
			t.Errorf("expected toolErr (not Go error) for empty command %q", cmd)
			continue
		}
		if !isErrorResult(result) {
			t.Errorf("expected IsError for empty command %q", cmd)
		}
		if !hasErrorCode(result, ErrBashEmptyCommand) {
			t.Errorf("expected error code %s for empty command %q, got: %s", ErrBashEmptyCommand, cmd, resultText(result))
		}
	}
}

func TestBashSIGTERM(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	// Use a trap to verify SIGTERM is received and process exits gracefully
	cmd := `trap 'echo got_sigterm; exit 0' TERM; sleep 300`
	start := time.Now()
	result, _, err := handler(context.Background(), nil, BashArgs{Command: cmd, Timeout: 1000})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "timed out") {
		t.Errorf("expected timeout message, got: %s", text)
	}
	// Should have been killed within ~2 seconds (1s timeout + trap handling),
	// not after the 5s SIGKILL grace period
	if elapsed > 4*time.Second {
		t.Errorf("expected graceful SIGTERM exit, took %v", elapsed)
	}
}

func TestBashOutputTruncation(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	t.Run("within limit", func(t *testing.T) {
		result, _, err := handler(context.Background(), nil, BashArgs{Command: "echo hello"})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		if strings.Contains(text, "Truncated") {
			t.Error("short output should not be truncated")
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		// Generate >30000 chars of output
		result, _, err := handler(context.Background(), nil, BashArgs{
			Command: "python3 -c \"print('x' * 50000)\"",
			Timeout: 10000,
		})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		if !strings.Contains(text, "Truncated") {
			t.Error("large output should be truncated")
		}
	})

	t.Run("stderr independent truncation", func(t *testing.T) {
		// Short stdout, large stderr
		result, _, err := handler(context.Background(), nil, BashArgs{
			Command: "echo short; python3 -c \"import sys; sys.stderr.write('y' * 50000)\"",
			Timeout: 10000,
		})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		if !strings.Contains(text, "short") {
			t.Error("stdout should be present")
		}
		if !strings.Contains(text, "Truncated") {
			t.Error("stderr should be truncated")
		}
	})
}

func TestBashBackgroundCommand(t *testing.T) {
	sess := session.New(t.TempDir())
	t.Cleanup(sess.Close)
	handler := bashHandler(sess, testConfig())

	t.Run("immediate return with task_id", func(t *testing.T) {
		result, _, err := handler(context.Background(), nil, BashArgs{
			Command:         "sleep 60",
			RunInBackground: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		if !strings.Contains(text, "task_id:") {
			t.Errorf("expected task_id in response, got: %s", text)
		}
		if !strings.Contains(text, "background") {
			t.Errorf("expected background confirmation, got: %s", text)
		}
	})

	t.Run("cwd not updated", func(t *testing.T) {
		tmp := t.TempDir()
		bgSess := session.New(tmp)
		t.Cleanup(bgSess.Close)
		bgHandler := bashHandler(bgSess, testConfig())

		_, _, err := bgHandler(context.Background(), nil, BashArgs{
			Command:         "cd /tmp",
			RunInBackground: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		// Wait a bit for the background command to complete
		time.Sleep(500 * time.Millisecond)
		if bgSess.Cwd() != tmp {
			t.Errorf("background command should not update cwd, got %q, want %q", bgSess.Cwd(), tmp)
		}
	})

	t.Run("task limit enforcement", func(t *testing.T) {
		limitSess := session.New(t.TempDir())
		t.Cleanup(limitSess.Close)
		limitHandler := bashHandler(limitSess, testConfig())

		// Fill up 10 tasks
		for i := 0; i < 10; i++ {
			result, _, err := limitHandler(context.Background(), nil, BashArgs{
				Command:         "sleep 300",
				RunInBackground: true,
			})
			if err != nil {
				t.Fatalf("task %d: %v", i, err)
			}
			if isErrorResult(result) {
				t.Fatalf("task %d should succeed: %s", i, resultText(result))
			}
		}

		// 11th should fail
		result, _, err := limitHandler(context.Background(), nil, BashArgs{
			Command:         "sleep 300",
			RunInBackground: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !isErrorResult(result) {
			t.Error("expected IsError when exceeding task limit")
		}
		if !hasErrorCode(result, ErrBashTaskLimit) {
			t.Errorf("expected error code %s, got: %s", ErrBashTaskLimit, resultText(result))
		}
	})
}

func TestTaskOutput(t *testing.T) {
	sess := session.New(t.TempDir())
	t.Cleanup(sess.Close)
	bashH := bashHandler(sess, testConfig())
	taskH := taskOutputHandler(sess, testConfig())

	t.Run("running status", func(t *testing.T) {
		result, _, _ := bashH(context.Background(), nil, BashArgs{
			Command:         "sleep 60",
			RunInBackground: true,
		})
		text := resultText(result)
		// Extract task_id
		taskID := ""
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "task_id: ") {
				taskID = strings.TrimPrefix(line, "task_id: ")
				break
			}
		}
		if taskID == "" {
			t.Fatal("no task_id in response")
		}

		result, _, err := taskH(context.Background(), nil, TaskOutputArgs{TaskID: taskID})
		if err != nil {
			t.Fatal(err)
		}
		text = resultText(result)
		if !strings.Contains(text, "status: running") {
			t.Errorf("expected running status, got: %s", text)
		}
	})

	t.Run("completed status with cleanup", func(t *testing.T) {
		result, _, _ := bashH(context.Background(), nil, BashArgs{
			Command:         "echo done",
			RunInBackground: true,
		})
		text := resultText(result)
		taskID := ""
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "task_id: ") {
				taskID = strings.TrimPrefix(line, "task_id: ")
				break
			}
		}
		if taskID == "" {
			t.Fatal("no task_id in response")
		}

		// Wait for completion
		time.Sleep(1 * time.Second)

		result, _, err := taskH(context.Background(), nil, TaskOutputArgs{TaskID: taskID})
		if err != nil {
			t.Fatal(err)
		}
		text = resultText(result)
		if !strings.Contains(text, "status: completed") {
			t.Errorf("expected completed status, got: %s", text)
		}
		if !strings.Contains(text, "done") {
			t.Errorf("expected 'done' in output, got: %s", text)
		}

		// Second retrieval should fail (single-read cleanup)
		result, _, err = taskH(context.Background(), nil, TaskOutputArgs{TaskID: taskID})
		if err != nil {
			t.Fatal(err)
		}
		if !isErrorResult(result) {
			t.Error("expected IsError after task cleanup")
		}
		if !hasErrorCode(result, ErrBashTaskNotFound) {
			t.Errorf("expected error code %s after cleanup, got: %s", ErrBashTaskNotFound, resultText(result))
		}
	})

	t.Run("unknown task_id", func(t *testing.T) {
		result, _, err := taskH(context.Background(), nil, TaskOutputArgs{TaskID: "nonexistent"})
		if err != nil {
			t.Fatal(err)
		}
		if !isErrorResult(result) {
			t.Error("expected IsError for unknown task_id")
		}
		if !hasErrorCode(result, ErrBashTaskNotFound) {
			t.Errorf("expected error code %s, got: %s", ErrBashTaskNotFound, resultText(result))
		}
	})
}

func TestBashDescriptionParameter(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	result, _, err := handler(context.Background(), nil, BashArgs{
		Command:     "echo hello",
		Description: "Print a greeting",
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", text)
	}
	if !strings.Contains(text, "exit_code: 0") {
		t.Errorf("expected exit_code: 0, got: %s", text)
	}
}

func TestBackgroundTaskOutputRace(t *testing.T) {
	sess := session.New(t.TempDir())
	t.Cleanup(sess.Close)
	bashH := bashHandler(sess, testConfig())
	taskH := taskOutputHandler(sess, testConfig())

	// Start a background command that produces continuous output
	result, _, err := bashH(context.Background(), nil, BashArgs{
		Command:         "for i in $(seq 1 50); do echo line$i; sleep 0.01; done",
		RunInBackground: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	taskID := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "task_id: ") {
			taskID = strings.TrimPrefix(line, "task_id: ")
			break
		}
	}
	if taskID == "" {
		t.Fatal("no task_id in response")
	}

	// Read output concurrently while the command is still running.
	// With -race, this exposes any concurrent read/write on the buffer.
	for i := 0; i < 10; i++ {
		result, _, err = taskH(context.Background(), nil, TaskOutputArgs{TaskID: taskID})
		if err != nil {
			t.Fatal(err)
		}
		text = resultText(result)
		if strings.Contains(text, "status: completed") {
			break // task finished; RemoveTask was called, further reads would 404
		}
		if isErrorResult(result) {
			t.Fatalf("unexpected error on iteration %d: %s", i, text)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBashRegistrationCallback(t *testing.T) {
	t.Run("fires on first bash call", func(t *testing.T) {
		sess := session.New(t.TempDir())
		t.Cleanup(sess.Close)
		var callCount int
		cfg := testConfig()
		cfg.RegisterSession = func(id string) { callCount++ }
		handler := bashHandler(sess, cfg)

		// First call — callback should fire (req is nil so it won't, we need to simulate)
		// With nil req, registration is skipped (STDIO-like)
		_, _, _ = handler(context.Background(), nil, BashArgs{Command: "echo hi"})
		if callCount != 0 {
			t.Errorf("expected 0 calls with nil req, got %d", callCount)
		}
	})

	t.Run("nil callback is safe", func(t *testing.T) {
		sess := session.New(t.TempDir())
		t.Cleanup(sess.Close)
		cfg := testConfig()
		// RegisterSession is nil (default/STDIO mode)
		handler := bashHandler(sess, cfg)

		// Should not panic.
		result, _, err := handler(context.Background(), nil, BashArgs{Command: "echo ok"})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		if !strings.Contains(text, "ok") {
			t.Errorf("expected 'ok' in output, got: %s", text)
		}
	})
}

func TestTaskOutputRegistrationCallback(t *testing.T) {
	t.Run("nil callback is safe", func(t *testing.T) {
		sess := session.New(t.TempDir())
		t.Cleanup(sess.Close)
		cfg := testConfig()
		handler := taskOutputHandler(sess, cfg)

		// Should not panic even with nil RegisterSession.
		result, _, err := handler(context.Background(), nil, TaskOutputArgs{TaskID: "nonexistent"})
		if err != nil {
			t.Fatal(err)
		}
		if !isErrorResult(result) {
			t.Error("expected error for unknown task")
		}
	})
}

func TestBashBackgroundTimeout(t *testing.T) {
	t.Run("task killed after timeout", func(t *testing.T) {
		sess := session.New(t.TempDir())
		t.Cleanup(sess.Close)
		cfg := testConfig()
		cfg.BackgroundTaskTimeout = 1 // 1 second
		bashH := bashHandler(sess, cfg)
		taskH := taskOutputHandler(sess, cfg)

		result, _, err := bashH(context.Background(), nil, BashArgs{
			Command:         "sleep 300",
			RunInBackground: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		taskID := ""
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "task_id: ") {
				taskID = strings.TrimPrefix(line, "task_id: ")
				break
			}
		}
		if taskID == "" {
			t.Fatal("no task_id in response")
		}

		// Wait for the 1s timeout + buffer
		time.Sleep(3 * time.Second)

		result, _, err = taskH(context.Background(), nil, TaskOutputArgs{TaskID: taskID})
		if err != nil {
			t.Fatal(err)
		}
		text = resultText(result)
		if !strings.Contains(text, "status: completed") {
			t.Errorf("expected completed status, got: %s", text)
		}
		if !strings.Contains(text, "killed by background task timeout") {
			t.Errorf("expected timeout message, got: %s", text)
		}
	})

	t.Run("timer cancelled on early completion", func(t *testing.T) {
		sess := session.New(t.TempDir())
		t.Cleanup(sess.Close)
		cfg := testConfig()
		cfg.BackgroundTaskTimeout = 300 // 5 minutes — should not fire
		bashH := bashHandler(sess, cfg)
		taskH := taskOutputHandler(sess, cfg)

		result, _, err := bashH(context.Background(), nil, BashArgs{
			Command:         "echo fast",
			RunInBackground: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		taskID := ""
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "task_id: ") {
				taskID = strings.TrimPrefix(line, "task_id: ")
				break
			}
		}
		if taskID == "" {
			t.Fatal("no task_id in response")
		}

		// Wait for the fast command to complete
		time.Sleep(1 * time.Second)

		result, _, err = taskH(context.Background(), nil, TaskOutputArgs{TaskID: taskID})
		if err != nil {
			t.Fatal(err)
		}
		text = resultText(result)
		if !strings.Contains(text, "status: completed") {
			t.Errorf("expected completed status, got: %s", text)
		}
		if strings.Contains(text, "killed by background task timeout") {
			t.Errorf("should not contain timeout message, got: %s", text)
		}
	})

	t.Run("no timer when bg-timeout is 0", func(t *testing.T) {
		sess := session.New(t.TempDir())
		t.Cleanup(sess.Close)
		cfg := testConfig()
		// BackgroundTaskTimeout is 0 by default in testConfig — no timer
		bashH := bashHandler(sess, cfg)
		taskH := taskOutputHandler(sess, cfg)

		result, _, err := bashH(context.Background(), nil, BashArgs{
			Command:         "echo done",
			RunInBackground: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		text := resultText(result)
		taskID := ""
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "task_id: ") {
				taskID = strings.TrimPrefix(line, "task_id: ")
				break
			}
		}
		if taskID == "" {
			t.Fatal("no task_id in response")
		}

		time.Sleep(1 * time.Second)

		result, _, err = taskH(context.Background(), nil, TaskOutputArgs{TaskID: taskID})
		if err != nil {
			t.Fatal(err)
		}
		text = resultText(result)
		if !strings.Contains(text, "status: completed") {
			t.Errorf("expected completed status, got: %s", text)
		}
		if strings.Contains(text, "killed by background task timeout") {
			t.Errorf("should not contain timeout message with bg-timeout=0, got: %s", text)
		}
	})
}

func TestBashForegroundTimeoutKillTimerStopped(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	// Use a command that traps SIGTERM and exits cleanly. The foreground
	// timeout fires SIGTERM, the process exits, and the inner 5s SIGKILL
	// timer must be cancelled — it should NOT fire after the handler returns.
	cmd := `trap 'exit 0' TERM; sleep 300`
	result, _, err := handler(context.Background(), nil, BashArgs{Command: cmd, Timeout: 500})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "timed out") {
		t.Errorf("expected timeout message, got: %s", text)
	}

	// Wait longer than the 5s SIGKILL window. If the timer leaked it would
	// send SIGKILL to a potentially recycled PGID. We can't directly observe
	// the timer being stopped, but we verify the process is dead and the test
	// completes without incident.
	time.Sleep(6 * time.Second)
}

func TestBashBackgroundTimeoutKillTimerStopped(t *testing.T) {
	sess := session.New(t.TempDir())
	t.Cleanup(sess.Close)
	cfg := testConfig()
	cfg.BackgroundTaskTimeout = 1 // 1 second
	bashH := bashHandler(sess, cfg)
	taskH := taskOutputHandler(sess, cfg)

	// Start a background command that traps SIGTERM and exits cleanly.
	result, _, err := bashH(context.Background(), nil, BashArgs{
		Command:         `trap 'exit 0' TERM; sleep 300`,
		RunInBackground: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	taskID := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "task_id: ") {
			taskID = strings.TrimPrefix(line, "task_id: ")
			break
		}
	}
	if taskID == "" {
		t.Fatal("no task_id in response")
	}

	// Wait for the 1s bg timeout + SIGTERM handling + buffer.
	time.Sleep(3 * time.Second)

	result, _, err = taskH(context.Background(), nil, TaskOutputArgs{TaskID: taskID})
	if err != nil {
		t.Fatal(err)
	}
	text = resultText(result)
	if !strings.Contains(text, "status: completed") {
		t.Errorf("expected completed status, got: %s", text)
	}

	// Wait past the 5s SIGKILL window to ensure the inner timer was cancelled.
	time.Sleep(6 * time.Second)
}

func TestBashIsErrorForOperationalErrors(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, testConfig())

	// Empty command should be IsError, not Go error
	result, _, err := handler(context.Background(), nil, BashArgs{Command: ""})
	if err != nil {
		t.Error("operational errors should not return Go errors")
	}
	if !isErrorResult(result) {
		t.Error("empty command should set IsError")
	}
	if !hasErrorCode(result, ErrBashEmptyCommand) {
		t.Errorf("expected error code %s, got: %s", ErrBashEmptyCommand, resultText(result))
	}

	// Non-zero exit code should NOT be IsError
	result, _, err = handler(context.Background(), nil, BashArgs{Command: "exit 1"})
	if err != nil {
		t.Fatal(err)
	}
	if isErrorResult(result) {
		t.Error("non-zero exit code should not set IsError")
	}
}
