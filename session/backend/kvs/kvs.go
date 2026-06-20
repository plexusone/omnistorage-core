// Package kvs provides a session storage backend using kvs.ListableStore.
//
// This adapter allows any kvs.ListableStore (Redis, SQLite, etc.) to be
// used as a session storage backend.
package kvs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/plexusone/omnistorage-core/kvs"
	"github.com/plexusone/omnistorage-core/session"
)

// Verify interface compliance.
var _ session.Store = (*Store)(nil)

// Store implements session.Store using a kvs.ListableStore backend.
type Store struct {
	backend kvs.ListableStore
	config  session.Config
	closed  bool
	closeCh chan struct{}
}

// New creates a new KVS-backed session store.
func New(backend kvs.ListableStore, cfg session.Config) *Store {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "session:"
	}

	s := &Store{
		backend: backend,
		config:  cfg,
		closeCh: make(chan struct{}),
	}

	// Start background cleanup if interval is set
	// Note: For Redis with TTL, this is mostly a no-op but cleans user index
	if cfg.CleanupInterval > 0 {
		go s.cleanup()
	}

	return s
}

// Key helpers
func (s *Store) sessionKey(id string) string {
	return s.config.KeyPrefix + id
}

func (s *Store) userSessionsKey(userID string) string {
	return s.config.KeyPrefix + "user:" + userID
}

// Create stores a new session.
func (s *Store) Create(ctx context.Context, sess *session.Session) error {
	if s.closed {
		return session.ErrClosed
	}

	if sess == nil || sess.ID == "" {
		return session.ErrInvalidSession
	}

	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	ttl := sess.TTL()
	if ttl <= 0 && s.config.DefaultTTL > 0 {
		ttl = s.config.DefaultTTL
	}

	// Store session data
	if err := s.backend.Set(ctx, s.sessionKey(sess.ID), data, ttl); err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}

	// Update user index (store as JSON array of session IDs)
	userKey := s.userSessionsKey(sess.UserID.String())
	if err := s.addToUserIndex(ctx, userKey, sess.ID, ttl); err != nil {
		// Best effort - don't fail the create
		// The cleanup routine will handle orphaned entries
	}

	return nil
}

// Get retrieves a session by ID.
func (s *Store) Get(ctx context.Context, id string) (*session.Session, error) {
	if s.closed {
		return nil, session.ErrClosed
	}

	data, err := s.backend.Get(ctx, s.sessionKey(id))
	if err == kvs.ErrNotFound {
		return nil, session.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var sess session.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	if sess.IsExpired() {
		_ = s.Delete(ctx, id)
		return nil, session.ErrExpired
	}

	return &sess, nil
}

// Update updates an existing session.
func (s *Store) Update(ctx context.Context, sess *session.Session) error {
	if s.closed {
		return session.ErrClosed
	}

	if sess == nil || sess.ID == "" {
		return session.ErrInvalidSession
	}

	// Check if session exists
	_, err := s.backend.Get(ctx, s.sessionKey(sess.ID))
	if err == kvs.ErrNotFound {
		return session.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}

	sess.UpdatedAt = time.Now()

	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	ttl := sess.TTL()
	if ttl <= 0 && s.config.DefaultTTL > 0 {
		ttl = s.config.DefaultTTL
	}

	if err := s.backend.Set(ctx, s.sessionKey(sess.ID), data, ttl); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// Delete removes a session by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	if s.closed {
		return session.ErrClosed
	}

	// Get session to find user ID for index cleanup
	data, err := s.backend.Get(ctx, s.sessionKey(id))
	if err == nil {
		var sess session.Session
		if json.Unmarshal(data, &sess) == nil {
			userKey := s.userSessionsKey(sess.UserID.String())
			_ = s.removeFromUserIndex(ctx, userKey, id)
		}
	}

	return s.backend.Delete(ctx, s.sessionKey(id))
}

// DeleteByUserID removes all sessions for a user.
func (s *Store) DeleteByUserID(ctx context.Context, userID string) (int, error) {
	if s.closed {
		return 0, session.ErrClosed
	}

	userKey := s.userSessionsKey(userID)
	sessionIDs, err := s.getUserSessionIDs(ctx, userKey)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, id := range sessionIDs {
		if err := s.backend.Delete(ctx, s.sessionKey(id)); err == nil {
			count++
		}
	}

	// Delete the user index
	_ = s.backend.Delete(ctx, userKey)

	return count, nil
}

// Touch updates the LastAccessedAt timestamp.
func (s *Store) Touch(ctx context.Context, id string) error {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	sess.LastAccessedAt = time.Now()
	return s.Update(ctx, sess)
}

// Close closes the store.
func (s *Store) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.closeCh)
	return s.backend.Close()
}

// User index helpers

func (s *Store) getUserSessionIDs(ctx context.Context, userKey string) ([]string, error) {
	data, err := s.backend.Get(ctx, userKey)
	if err == kvs.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var sessionIDs []string
	if err := json.Unmarshal(data, &sessionIDs); err != nil {
		return nil, err
	}
	return sessionIDs, nil
}

func (s *Store) addToUserIndex(ctx context.Context, userKey, sessionID string, ttl time.Duration) error {
	sessionIDs, _ := s.getUserSessionIDs(ctx, userKey)

	// Add if not already present
	for _, id := range sessionIDs {
		if id == sessionID {
			return nil
		}
	}
	sessionIDs = append(sessionIDs, sessionID)

	data, err := json.Marshal(sessionIDs)
	if err != nil {
		return err
	}

	// Use slightly longer TTL for index
	indexTTL := ttl + time.Hour
	return s.backend.Set(ctx, userKey, data, indexTTL)
}

func (s *Store) removeFromUserIndex(ctx context.Context, userKey, sessionID string) error {
	sessionIDs, err := s.getUserSessionIDs(ctx, userKey)
	if err != nil || len(sessionIDs) == 0 {
		return err
	}

	// Remove the session ID
	filtered := make([]string, 0, len(sessionIDs)-1)
	for _, id := range sessionIDs {
		if id != sessionID {
			filtered = append(filtered, id)
		}
	}

	if len(filtered) == 0 {
		return s.backend.Delete(ctx, userKey)
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return err
	}

	// Keep existing TTL (best effort)
	return s.backend.Set(ctx, userKey, data, 0)
}

// cleanup periodically cleans orphaned user index entries.
func (s *Store) cleanup() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.closeCh:
			return
		case <-ticker.C:
			s.cleanupOrphanedIndexes()
		}
	}
}

func (s *Store) cleanupOrphanedIndexes() {
	ctx := context.Background()

	// List all user index keys
	keys, err := s.backend.List(ctx, s.config.KeyPrefix+"user:")
	if err != nil {
		return
	}

	for _, key := range keys {
		// Extract user ID from key
		userID := strings.TrimPrefix(key, s.config.KeyPrefix+"user:")
		sessionIDs, err := s.getUserSessionIDs(ctx, key)
		if err != nil {
			continue
		}

		// Check which sessions still exist
		validIDs := make([]string, 0, len(sessionIDs))
		for _, id := range sessionIDs {
			_, err := s.backend.Get(ctx, s.sessionKey(id))
			if err == nil {
				validIDs = append(validIDs, id)
			}
		}

		// Update or delete the index
		if len(validIDs) == 0 {
			_ = s.backend.Delete(ctx, s.userSessionsKey(userID))
		} else if len(validIDs) < len(sessionIDs) {
			data, _ := json.Marshal(validIDs)
			_ = s.backend.Set(ctx, key, data, 0)
		}
	}
}

// ListByUserID returns all sessions for a user, sorted by CreatedAt ascending (oldest first).
func (s *Store) ListByUserID(ctx context.Context, userID string) ([]*session.Session, error) {
	if s.closed {
		return nil, session.ErrClosed
	}

	userKey := s.userSessionsKey(userID)
	sessionIDs, err := s.getUserSessionIDs(ctx, userKey)
	if err != nil {
		return nil, err
	}

	if len(sessionIDs) == 0 {
		return nil, nil
	}

	// Fetch all sessions
	sessions := make([]*session.Session, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		sess, err := s.Get(ctx, id)
		if err == nil {
			sessions = append(sessions, sess)
		}
		// Skip sessions that no longer exist or are expired
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

// NewWithControls creates a new KVS-backed session store with controls.
// This is a convenience function that wraps New() with WithControls().
func NewWithControls(backend kvs.ListableStore, cfg session.Config, opts ...session.ControlOption) session.Store {
	return session.WithControls(New(backend, cfg), cfg, opts...)
}
