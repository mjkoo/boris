package session

import (
	"context"
	"iter"
)

// SessionCleanupStore is a minimal mcp.EventStore implementation whose sole
// purpose is to receive SessionClosed notifications from the go-sdk and
// trigger Boris session cleanup via the SessionRegistry.
//
// Open, Append, and After are no-ops — stream resumption is not supported.
type SessionCleanupStore struct {
	Registry *SessionRegistry
}

// Open is a no-op; stream resumption is not supported.
func (s *SessionCleanupStore) Open(_ context.Context, _, _ string) error {
	return nil
}

// Append is a no-op; stream resumption is not supported.
func (s *SessionCleanupStore) Append(_ context.Context, _, _ string, _ []byte) error {
	return nil
}

// After returns an empty iterator; stream resumption is not supported.
func (s *SessionCleanupStore) After(_ context.Context, _, _ string, _ int) iter.Seq2[[]byte, error] {
	return func(func([]byte, error) bool) {}
}

// SessionClosed is called by the go-sdk when an HTTP session terminates
// (idle timeout, client DELETE, or connection drop). It closes the
// corresponding Boris session and removes it from the registry.
func (s *SessionCleanupStore) SessionClosed(_ context.Context, sessionID string) error {
	s.Registry.CloseAndRemove(sessionID)
	return nil
}
