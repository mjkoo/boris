package session

import "sync"

// SessionRegistry maps go-sdk session IDs to Boris sessions, enabling
// cleanup when the SDK signals session end (via EventStore.SessionClosed).
type SessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewRegistry creates an empty SessionRegistry.
func NewRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]*Session),
	}
}

// Register associates a go-sdk session ID with a Boris session.
// If the ID is already registered, the existing entry is overwritten.
func (r *SessionRegistry) Register(id string, sess *Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[id] = sess
}

// CloseAndRemove closes the Boris session for the given ID and removes it
// from the registry. If the ID is not found, this is a no-op.
func (r *SessionRegistry) CloseAndRemove(id string) {
	r.mu.Lock()
	sess, ok := r.sessions[id]
	if ok {
		delete(r.sessions, id)
	}
	r.mu.Unlock()
	if ok {
		sess.Close()
	}
}

// CloseAll closes every session in the registry and clears the map.
// It is safe to call concurrently with CloseAndRemove and is idempotent.
func (r *SessionRegistry) CloseAll() {
	r.mu.Lock()
	sessions := make([]*Session, 0, len(r.sessions))
	for _, sess := range r.sessions {
		sessions = append(sessions, sess)
	}
	r.sessions = make(map[string]*Session)
	r.mu.Unlock()

	for _, sess := range sessions {
		sess.Close()
	}
}
