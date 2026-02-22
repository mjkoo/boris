package session

import "sync"

// Session holds per-session state, currently just the tracked working directory.
type Session struct {
	mu  sync.Mutex
	cwd string
}

// New creates a Session with the given initial working directory.
func New(cwd string) *Session {
	return &Session{cwd: cwd}
}

// Cwd returns the current working directory.
func (s *Session) Cwd() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}

// SetCwd updates the current working directory.
func (s *Session) SetCwd(cwd string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cwd = cwd
}
