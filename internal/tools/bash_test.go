package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/mjkoo/boris/internal/session"
)

func TestBashSimpleCommand(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, 120)

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
	handler := bashHandler(sess, 120)

	result, _, err := handler(context.Background(), nil, BashArgs{Command: "exit 42"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "exit_code: 42") {
		t.Errorf("expected exit_code: 42, got: %s", text)
	}
}

func TestBashStderrCapture(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, 120)

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
	handler := bashHandler(sess, 120)

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
	handler := bashHandler(sess, 120)

	result, _, err := handler(context.Background(), nil, BashArgs{Command: "echo hello"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if strings.Contains(text, cwdSentinel) {
		t.Errorf("sentinel should be stripped from output: %s", text)
	}
}

func TestBashTimeout(t *testing.T) {
	sess := session.New(t.TempDir())
	handler := bashHandler(sess, 120)

	result, _, err := handler(context.Background(), nil, BashArgs{Command: "sleep 300", Timeout: 1})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "timed out") {
		t.Errorf("expected timeout message, got: %s", text)
	}
}

func TestBashMissingSentinelPreservesCwd(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	handler := bashHandler(sess, 120)

	// Timeout before sentinel is printed — cwd should be preserved
	_, _, _ = handler(context.Background(), nil, BashArgs{Command: "sleep 300", Timeout: 1})

	if sess.Cwd() != tmp {
		t.Errorf("cwd should be preserved after timeout, got %q, want %q", sess.Cwd(), tmp)
	}
}

func TestBashInitialWorkdir(t *testing.T) {
	tmp := t.TempDir()
	sess := session.New(tmp)
	handler := bashHandler(sess, 120)

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
	handler := bashHandler(sess, 120)

	for _, cmd := range []string{"", "  ", "\t\n"} {
		_, _, err := handler(context.Background(), nil, BashArgs{Command: cmd})
		if err == nil {
			t.Errorf("expected error for empty command %q", cmd)
		}
	}
}
