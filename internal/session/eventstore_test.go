package session

import (
	"context"
	"testing"
)

func TestSessionCleanupStoreSessionClosed(t *testing.T) {
	r := NewRegistry()
	store := &SessionCleanupStore{Registry: r}

	s := New("/workspace")
	task := startSleepTask(t, "t1")
	if err := s.AddTask(task); err != nil {
		t.Fatal(err)
	}
	r.Register("sdk-abc", s)

	if err := store.SessionClosed(context.Background(), "sdk-abc"); err != nil {
		t.Fatal(err)
	}

	// Session should be closed.
	select {
	case <-task.Done:
	default:
		t.Error("expected task to be killed after SessionClosed")
	}
}

func TestSessionCleanupStoreNoOps(t *testing.T) {
	store := &SessionCleanupStore{Registry: NewRegistry()}

	if err := store.Open(context.Background(), "s", "stream"); err != nil {
		t.Errorf("Open returned error: %v", err)
	}
	if err := store.Append(context.Background(), "s", "stream", []byte("data")); err != nil {
		t.Errorf("Append returned error: %v", err)
	}

	iter := store.After(context.Background(), "s", "stream", 0)
	count := 0
	iter(func(_ []byte, _ error) bool {
		count++
		return true
	})
	if count != 0 {
		t.Errorf("After should yield 0 items, got %d", count)
	}
}

func TestSessionCleanupStoreUnknownSession(t *testing.T) {
	store := &SessionCleanupStore{Registry: NewRegistry()}
	// Should not panic.
	if err := store.SessionClosed(context.Background(), "unknown"); err != nil {
		t.Errorf("SessionClosed on unknown ID returned error: %v", err)
	}
}
