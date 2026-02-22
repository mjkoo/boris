package session

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"sync"
)

// SyncBuffer is a concurrency-safe buffer that implements io.Writer.
// It is safe for concurrent use, e.g. as cmd.Stdout while reading
// accumulated output from another goroutine.
type SyncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *SyncBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *SyncBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

// BackgroundTask represents a command running in the background.
type BackgroundTask struct {
	ID       string
	Cmd      *exec.Cmd
	Stdout   *SyncBuffer
	Stderr   *SyncBuffer
	Done     chan struct{}
	ExitCode int
}

// Session holds per-session state including the tracked working directory,
// a random nonce for sentinel generation, and background task tracking.
type Session struct {
	mu    sync.Mutex
	cwd   string
	nonce string
	tasks map[string]*BackgroundTask
}

// New creates a Session with the given initial working directory.
func New(cwd string) *Session {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate session nonce: %v", err))
	}
	return &Session{
		cwd:   cwd,
		nonce: hex.EncodeToString(b),
		tasks: make(map[string]*BackgroundTask),
	}
}

// Nonce returns the session's random nonce.
func (s *Session) Nonce() string {
	return s.nonce
}

// Sentinel returns the cwd sentinel string for this session.
func (s *Session) Sentinel() string {
	return fmt.Sprintf("__BORIS_CWD_%s__", s.nonce)
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

// AddTask stores a background task. Returns an error if the limit is reached.
func (s *Session) AddTask(task *BackgroundTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tasks) >= 10 {
		return fmt.Errorf("maximum concurrent background task limit (10) reached")
	}
	s.tasks[task.ID] = task
	return nil
}

// GetTask retrieves a background task by ID.
func (s *Session) GetTask(id string) (*BackgroundTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	return t, ok
}

// RemoveTask removes a background task by ID.
func (s *Session) RemoveTask(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, id)
}

// TaskCount returns the number of active background tasks.
func (s *Session) TaskCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tasks)
}
