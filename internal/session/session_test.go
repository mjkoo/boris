package session

import (
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	s := New("/workspace")
	if got := s.Cwd(); got != "/workspace" {
		t.Errorf("Cwd() = %q, want %q", got, "/workspace")
	}
}

func TestSetGetRoundTrip(t *testing.T) {
	s := New("/initial")
	s.SetCwd("/updated")
	if got := s.Cwd(); got != "/updated" {
		t.Errorf("Cwd() = %q, want %q", got, "/updated")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := New("/start")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.SetCwd("/concurrent")
			_ = s.Cwd()
		}()
	}
	wg.Wait()
	// No race detector failure means success
}

func TestNonce(t *testing.T) {
	s := New("/workspace")
	nonce := s.Nonce()
	if len(nonce) != 8 {
		t.Errorf("nonce should be 8 hex chars, got %q (len %d)", nonce, len(nonce))
	}

	// Each session should have a unique nonce
	s2 := New("/workspace")
	if s.Nonce() == s2.Nonce() {
		t.Error("two sessions should have different nonces")
	}
}

func TestSentinel(t *testing.T) {
	s := New("/workspace")
	sentinel := s.Sentinel()

	if !strings.HasPrefix(sentinel, "__BORIS_CWD_") {
		t.Errorf("sentinel should start with __BORIS_CWD_, got %q", sentinel)
	}
	if !strings.HasSuffix(sentinel, "__") {
		t.Errorf("sentinel should end with __, got %q", sentinel)
	}
	if !strings.Contains(sentinel, s.Nonce()) {
		t.Errorf("sentinel should contain nonce %q, got %q", s.Nonce(), sentinel)
	}
}

func TestBackgroundTasks(t *testing.T) {
	s := New("/workspace")

	t.Run("add and get task", func(t *testing.T) {
		task := &BackgroundTask{
			ID:   "test-1",
			Done: make(chan struct{}),
		}
		if err := s.AddTask(task); err != nil {
			t.Fatal(err)
		}
		got, ok := s.GetTask("test-1")
		if !ok {
			t.Fatal("expected to find task")
		}
		if got.ID != "test-1" {
			t.Errorf("got task ID %q, want %q", got.ID, "test-1")
		}
	})

	t.Run("remove task", func(t *testing.T) {
		s.RemoveTask("test-1")
		_, ok := s.GetTask("test-1")
		if ok {
			t.Error("task should be removed")
		}
	})

	t.Run("task count", func(t *testing.T) {
		if s.TaskCount() != 0 {
			t.Errorf("expected 0 tasks, got %d", s.TaskCount())
		}
		s.AddTask(&BackgroundTask{ID: "a", Done: make(chan struct{})})
		s.AddTask(&BackgroundTask{ID: "b", Done: make(chan struct{})})
		if s.TaskCount() != 2 {
			t.Errorf("expected 2 tasks, got %d", s.TaskCount())
		}
		s.RemoveTask("a")
		s.RemoveTask("b")
	})

	t.Run("max 10 tasks", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			err := s.AddTask(&BackgroundTask{
				ID:   string(rune('0' + i)),
				Done: make(chan struct{}),
			})
			if err != nil {
				t.Fatalf("task %d should succeed: %v", i, err)
			}
		}
		err := s.AddTask(&BackgroundTask{
			ID:   "overflow",
			Done: make(chan struct{}),
		})
		if err == nil {
			t.Error("expected error for exceeding task limit")
		}
		// Clean up
		for i := 0; i < 10; i++ {
			s.RemoveTask(string(rune('0' + i)))
		}
	})
}

// startSleepTask starts a real "sleep" process as a background task with
// process group isolation, matching how bash.go launches background tasks.
func startSleepTask(t *testing.T, id string) *BackgroundTask {
	t.Helper()
	cmd := exec.Command("sleep", "300")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start sleep process: %v", err)
	}
	task := &BackgroundTask{
		ID:   id,
		Cmd:  cmd,
		Done: make(chan struct{}),
	}
	go func() {
		defer close(task.Done)
		_ = cmd.Wait()
	}()
	return task
}

func TestCloseKillsRunningTasks(t *testing.T) {
	s := New("/workspace")

	task1 := startSleepTask(t, "t1")
	task2 := startSleepTask(t, "t2")
	task3 := startSleepTask(t, "t3")
	for _, task := range []*BackgroundTask{task1, task2, task3} {
		if err := s.AddTask(task); err != nil {
			t.Fatal(err)
		}
	}

	s.Close()

	// All tasks should be done after Close returns.
	for _, task := range []*BackgroundTask{task1, task2, task3} {
		select {
		case <-task.Done:
		case <-time.After(10 * time.Second):
			t.Fatalf("task %s still running after Close", task.ID)
		}
	}

	if s.TaskCount() != 0 {
		t.Errorf("expected 0 tasks after Close, got %d", s.TaskCount())
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	s := New("/workspace")
	task := startSleepTask(t, "t1")
	if err := s.AddTask(task); err != nil {
		t.Fatal(err)
	}

	s.Close()
	// Second call should not panic or block.
	s.Close()

	select {
	case <-task.Done:
	case <-time.After(10 * time.Second):
		t.Fatal("task still running after Close")
	}
}

func TestCloseSkipsCompletedTasks(t *testing.T) {
	s := New("/workspace")

	// Start a task that completes immediately.
	cmd := exec.Command("true")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	task := &BackgroundTask{ID: "completed", Cmd: cmd, Done: done}
	go func() {
		defer close(done)
		_ = cmd.Wait()
	}()

	// Wait for it to finish.
	<-done

	if err := s.AddTask(task); err != nil {
		t.Fatal(err)
	}

	// Close should not block or error on the already-completed task.
	s.Close()
}

func TestAddTaskRejectedAfterClose(t *testing.T) {
	s := New("/workspace")
	s.Close()

	err := s.AddTask(&BackgroundTask{ID: "late", Done: make(chan struct{})})
	if err == nil {
		t.Error("expected error adding task to closed session")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected 'closed' in error message, got: %v", err)
	}
}

func TestCloseConcurrentSafety(t *testing.T) {
	s := New("/workspace")
	for i := 0; i < 3; i++ {
		task := startSleepTask(t, string(rune('a'+i)))
		if err := s.AddTask(task); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Close()
		}()
	}
	wg.Wait()
	// No race detector failure means success.
}
