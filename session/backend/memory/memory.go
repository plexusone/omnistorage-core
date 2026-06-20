// Package memory provides an in-memory session storage backend.
//
// This backend is suitable for development and testing.
// Data is not persisted across restarts.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/plexusone/omnistorage-core/session"
)

// Verify interface compliance.
var _ session.Store = (*Store)(nil)

// Store implements session.Store with in-memory storage.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*session.Session
	// userIndex maps userID -> set of session IDs
	userIndex map[string]map[string]struct{}
	config    session.Config
	closed    bool
	closeCh   chan struct{}
}

// New creates a new in-memory session store.
func New(cfg session.Config) *Store {
	s := &Store{
		sessions:  make(map[string]*session.Session),
		userIndex: make(map[string]map[string]struct{}),
		config:    cfg,
		closeCh:   make(chan struct{}),
	}

	// Start background cleanup if interval is set
	if cfg.CleanupInterval > 0 {
		go s.cleanup()
	}

	return s
}

// Create stores a new session.
func (s *Store) Create(ctx context.Context, sess *session.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if sess == nil || sess.ID == "" {
		return session.ErrInvalidSession
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return session.ErrClosed
	}

	// Copy session to prevent external modification
	sessCopy := *sess
	s.sessions[sess.ID] = &sessCopy

	// Update user index
	userID := sess.UserID.String()
	if s.userIndex[userID] == nil {
		s.userIndex[userID] = make(map[string]struct{})
	}
	s.userIndex[userID][sess.ID] = struct{}{}

	return nil
}

// Get retrieves a session by ID.
func (s *Store) Get(ctx context.Context, id string) (*session.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	sess, exists := s.sessions[id]
	s.mu.RUnlock()

	if !exists {
		return nil, session.ErrNotFound
	}

	if s.closed {
		return nil, session.ErrClosed
	}

	if sess.IsExpired() {
		// Clean up expired session
		s.mu.Lock()
		s.deleteSessionLocked(id)
		s.mu.Unlock()
		return nil, session.ErrExpired
	}

	// Return a copy to prevent external modification
	sessCopy := *sess
	return &sessCopy, nil
}

// Update updates an existing session.
func (s *Store) Update(ctx context.Context, sess *session.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if sess == nil || sess.ID == "" {
		return session.ErrInvalidSession
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return session.ErrClosed
	}

	if _, exists := s.sessions[sess.ID]; !exists {
		return session.ErrNotFound
	}

	sess.UpdatedAt = time.Now()
	sessCopy := *sess
	s.sessions[sess.ID] = &sessCopy

	return nil
}

// Delete removes a session by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return session.ErrClosed
	}

	s.deleteSessionLocked(id)
	return nil
}

// deleteSessionLocked removes a session. Must be called with mu locked.
func (s *Store) deleteSessionLocked(id string) {
	sess, exists := s.sessions[id]
	if !exists {
		return
	}

	// Remove from user index
	userID := sess.UserID.String()
	if userSessions, ok := s.userIndex[userID]; ok {
		delete(userSessions, id)
		if len(userSessions) == 0 {
			delete(s.userIndex, userID)
		}
	}

	delete(s.sessions, id)
}

// DeleteByUserID removes all sessions for a user.
func (s *Store) DeleteByUserID(ctx context.Context, userID string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, session.ErrClosed
	}

	sessionIDs, exists := s.userIndex[userID]
	if !exists {
		return 0, nil
	}

	count := len(sessionIDs)
	for id := range sessionIDs {
		delete(s.sessions, id)
	}
	delete(s.userIndex, userID)

	return count, nil
}

// Touch updates the LastAccessedAt timestamp.
func (s *Store) Touch(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return session.ErrClosed
	}

	sess, exists := s.sessions[id]
	if !exists {
		return session.ErrNotFound
	}

	sess.LastAccessedAt = time.Now()
	return nil
}

// Close stops the cleanup goroutine and releases resources.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	close(s.closeCh)
	s.sessions = nil
	s.userIndex = nil
	return nil
}

// cleanup periodically removes expired sessions.
func (s *Store) cleanup() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.closeCh:
			return
		case <-ticker.C:
			s.removeExpired()
		}
	}
}

func (s *Store) removeExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	now := time.Now()
	for id, sess := range s.sessions {
		if !sess.ExpiresAt.IsZero() && now.After(sess.ExpiresAt) {
			s.deleteSessionLocked(id)
		}
	}
}

// Len returns the number of sessions (for testing).
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// ListByUserID returns all sessions for a user, sorted by CreatedAt ascending (oldest first).
func (s *Store) ListByUserID(ctx context.Context, userID string) ([]*session.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, session.ErrClosed
	}

	sessionIDs, exists := s.userIndex[userID]
	if !exists {
		return nil, nil
	}

	// Collect sessions
	sessions := make([]*session.Session, 0, len(sessionIDs))
	for id := range sessionIDs {
		if sess, ok := s.sessions[id]; ok {
			// Return copies to prevent external modification
			sessCopy := *sess
			sessions = append(sessions, &sessCopy)
		}
	}

	// Sort by CreatedAt ascending (oldest first)
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].CreatedAt.Before(sessions[i].CreatedAt) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	return sessions, nil
}

// NewWithControls creates a new in-memory session store with controls.
// This is a convenience function that wraps New() with WithControls().
func NewWithControls(cfg session.Config, opts ...session.ControlOption) session.Store {
	return session.WithControls(New(cfg), cfg, opts...)
}
