package session

import (
	"fmt"
	"sync"
	"testing"
)

func TestRegistryRegisterAndClose(t *testing.T) {
	r := NewRegistry()
	s := New("/workspace")

	task := startSleepTask(t, "t1")
	if err := s.AddTask(task); err != nil {
		t.Fatal(err)
	}

	r.Register("sdk-123", s)
	r.CloseAndRemove("sdk-123")

	// Session should be closed — task killed, AddTask rejected.
	select {
	case <-task.Done:
	default:
		t.Error("expected task to be killed after CloseAndRemove")
	}

	err := s.AddTask(&BackgroundTask{ID: "late", Done: make(chan struct{})})
	if err == nil {
		t.Error("expected error adding task to closed session")
	}
}

func TestRegistryCloseAndRemoveUnknownID(t *testing.T) {
	r := NewRegistry()
	// Should not panic or error.
	r.CloseAndRemove("nonexistent")
}

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := New("/workspace")
			id := string(rune('a' + i%26))
			r.Register(id, s)
			r.CloseAndRemove(id)
		}(i)
	}
	wg.Wait()
	// No race detector failure means success.
}

func TestRegistryCloseAll(t *testing.T) {
	r := NewRegistry()

	// Register 3 sessions, each with a running task.
	var tasks []*BackgroundTask
	var sessions []*Session
	for i := 0; i < 3; i++ {
		s := New("/workspace")
		task := startSleepTask(t, string(rune('a'+i)))
		if err := s.AddTask(task); err != nil {
			t.Fatal(err)
		}
		r.Register(fmt.Sprintf("sdk-%d", i), s)
		tasks = append(tasks, task)
		sessions = append(sessions, s)
	}

	r.CloseAll()

	// All tasks should be killed.
	for _, task := range tasks {
		select {
		case <-task.Done:
		default:
			t.Errorf("expected task %s to be killed after CloseAll", task.ID)
		}
	}

	// All sessions should reject new tasks.
	for i, s := range sessions {
		err := s.AddTask(&BackgroundTask{ID: "late", Done: make(chan struct{})})
		if err == nil {
			t.Errorf("session %d: expected error adding task to closed session", i)
		}
	}
}

func TestRegistryCloseAllIdempotent(t *testing.T) {
	r := NewRegistry()
	s := New("/workspace")
	task := startSleepTask(t, "t1")
	if err := s.AddTask(task); err != nil {
		t.Fatal(err)
	}
	r.Register("sdk-1", s)

	// Should not panic or error on repeated calls.
	r.CloseAll()
	r.CloseAll()
	r.CloseAll()

	select {
	case <-task.Done:
	default:
		t.Error("expected task to be killed after CloseAll")
	}
}

func TestRegistryCloseAllConcurrentWithCloseAndRemove(t *testing.T) {
	r := NewRegistry()

	for i := 0; i < 10; i++ {
		s := New("/workspace")
		task := startSleepTask(t, fmt.Sprintf("t%d", i))
		if err := s.AddTask(task); err != nil {
			t.Fatal(err)
		}
		r.Register(fmt.Sprintf("sdk-%d", i), s)
	}

	var wg sync.WaitGroup
	// Half the goroutines do CloseAll, the other half do CloseAndRemove.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				r.CloseAll()
			} else {
				r.CloseAndRemove(fmt.Sprintf("sdk-%d", i))
			}
		}(i)
	}
	wg.Wait()
	// No race detector failure or panic means success.
}
