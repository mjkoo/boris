package session

import (
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
