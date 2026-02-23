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

func TestSessionCleanupStoreUnknownSession(t *testing.T) {
	store := &SessionCleanupStore{Registry: NewRegistry()}
	// Should not panic.
	if err := store.SessionClosed(context.Background(), "unknown"); err != nil {
		t.Errorf("SessionClosed on unknown ID returned error: %v", err)
	}
}
