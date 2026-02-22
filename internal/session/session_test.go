package session

import (
	"strings"
	"sync"
	"testing"
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
