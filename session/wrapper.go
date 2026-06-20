package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// ViolationType identifies the type of policy violation.
type ViolationType string

const (
	ViolationSizeExceeded         ViolationType = "size_exceeded"
	ViolationNotSerializable      ViolationType = "not_serializable"
	ViolationInvalidSession       ViolationType = "invalid_session"
	ViolationSessionLimitExceeded ViolationType = "session_limit_exceeded"
	ViolationSiteMismatch         ViolationType = "site_mismatch"
)

// ViolationEvent contains information about a policy violation.
type ViolationEvent struct {
	Type      ViolationType
	SessionID string
	UserID    uuid.UUID
	SiteID    string // Site identifier for multi-site isolation
	Size      int    // Current size in bytes
	Limit     int    // Configured limit
	Message   string
}

// ViolationHandler is called when a session policy is violated.
// Use this to emit metrics, alerts, or audit logs.
type ViolationHandler func(ctx context.Context, event ViolationEvent)

// ControlledStore wraps a Store with size limits, validation, and observability.
type ControlledStore struct {
	store            Store
	config           Config
	logger           *slog.Logger
	violationHandler ViolationHandler
}

// ControlOption configures a ControlledStore.
type ControlOption func(*ControlledStore)

// WithLogger sets the logger for the controlled store.
func WithLogger(logger *slog.Logger) ControlOption {
	return func(s *ControlledStore) {
		s.logger = logger
	}
}

// WithViolationHandler sets a callback for policy violations.
// Use this to emit metrics (e.g., OTel counters) or alerts.
func WithViolationHandler(handler ViolationHandler) ControlOption {
	return func(s *ControlledStore) {
		s.violationHandler = handler
	}
}

// WithControls wraps a Store with size limits and validation controls.
func WithControls(store Store, cfg Config, opts ...ControlOption) *ControlledStore {
	s := &ControlledStore{
		store:  store,
		config: cfg,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// validateSession checks if a session is valid.
func (s *ControlledStore) validateSession(ctx context.Context, session *Session) error {
	if session == nil {
		err := NewSessionError(
			ErrCodeInvalidSession,
			"session is nil",
			nil,
			ErrInvalidSession,
		)
		s.reportViolation(ctx, ViolationEvent{
			Type:    ViolationInvalidSession,
			Message: "session is nil",
		})
		return err
	}
	if session.ID == "" {
		err := NewSessionError(
			ErrCodeInvalidSession,
			"empty session ID",
			map[string]any{"user_id": session.UserID.String()},
			ErrInvalidSession,
		)
		s.reportViolation(ctx, ViolationEvent{
			Type:    ViolationInvalidSession,
			UserID:  session.UserID,
			Message: "empty session ID",
		})
		return err
	}
	if session.UserID == (uuid.UUID{}) {
		err := NewSessionError(
			ErrCodeInvalidSession,
			"empty user ID",
			map[string]any{"session_id": session.ID},
			ErrInvalidSession,
		)
		s.reportViolation(ctx, ViolationEvent{
			Type:      ViolationInvalidSession,
			SessionID: session.ID,
			Message:   "empty user ID",
		})
		return err
	}
	return nil
}

// validateSerializable ensures session data can be serialized to JSON.
// This prevents storing Go-specific types (gob, pointers to unexported structs, etc.)
// that would fail on deserialization or be unreadable by other languages.
func (s *ControlledStore) validateSerializable(ctx context.Context, session *Session) error {
	if session.Data == nil {
		return nil
	}

	// Attempt JSON round-trip to catch non-serializable types
	data, err := json.Marshal(session.Data)
	if err != nil {
		serr := NewSessionError(
			ErrCodeNotSerializable,
			"session data is not JSON-serializable",
			map[string]any{
				"session_id": session.ID,
				"user_id":    session.UserID.String(),
				"error":      err.Error(),
			},
			err,
		)
		s.reportViolation(ctx, ViolationEvent{
			Type:      ViolationNotSerializable,
			SessionID: session.ID,
			UserID:    session.UserID,
			Message:   fmt.Sprintf("marshal failed: %v", err),
		})
		return serr
	}

	// Validate it unmarshals correctly (catches edge cases)
	var check map[string]any
	if err := json.Unmarshal(data, &check); err != nil {
		serr := NewSessionError(
			ErrCodeNotSerializable,
			"session data JSON is malformed",
			map[string]any{
				"session_id": session.ID,
				"user_id":    session.UserID.String(),
				"error":      err.Error(),
			},
			err,
		)
		s.reportViolation(ctx, ViolationEvent{
			Type:      ViolationNotSerializable,
			SessionID: session.ID,
			UserID:    session.UserID,
			Message:   fmt.Sprintf("unmarshal failed: %v", err),
		})
		return serr
	}

	return nil
}

// checkSizeLimit validates the session doesn't exceed the size limit.
func (s *ControlledStore) checkSizeLimit(ctx context.Context, session *Session) error {
	if s.config.MaxSessionSize <= 0 {
		return nil // No limit configured
	}

	data, err := json.Marshal(session)
	if err != nil {
		// This shouldn't happen if validateSerializable passed,
		// but handle it gracefully
		return NewSessionError(
			ErrCodeNotSerializable,
			"failed to serialize session for size check",
			map[string]any{"session_id": session.ID},
			err,
		)
	}

	size := len(data)
	if size > s.config.MaxSessionSize {
		serr := NewSessionError(
			ErrCodeSizeLimitExceeded,
			fmt.Sprintf("session size %d bytes exceeds limit %d bytes", size, s.config.MaxSessionSize),
			map[string]any{
				"session_id": session.ID,
				"user_id":    session.UserID.String(),
				"size":       size,
				"limit":      s.config.MaxSessionSize,
			},
			ErrSizeLimitExceeded,
		)

		s.reportViolation(ctx, ViolationEvent{
			Type:      ViolationSizeExceeded,
			SessionID: session.ID,
			UserID:    session.UserID,
			Size:      size,
			Limit:     s.config.MaxSessionSize,
			Message:   fmt.Sprintf("size %d exceeds limit %d", size, s.config.MaxSessionSize),
		})

		return serr
	}

	return nil
}

// reportViolation logs and dispatches violation events.
func (s *ControlledStore) reportViolation(ctx context.Context, event ViolationEvent) {
	// Always log violations
	s.logger.WarnContext(ctx, "session policy violation",
		slog.String("type", string(event.Type)),
		slog.String("session_id", event.SessionID),
		slog.String("user_id", event.UserID.String()),
		slog.String("site_id", event.SiteID),
		slog.Int("size", event.Size),
		slog.Int("limit", event.Limit),
		slog.String("message", event.Message),
	)

	// Call custom handler if configured (for metrics/alerts)
	if s.violationHandler != nil {
		s.violationHandler(ctx, event)
	}
}

// Create stores a new session with validation.
func (s *ControlledStore) Create(ctx context.Context, session *Session) error {
	if err := s.validateSession(ctx, session); err != nil {
		return err
	}

	// Set SiteID from config for multi-site isolation
	if s.config.SiteID != "" {
		session.SiteID = s.config.SiteID
	}

	if err := s.validateSerializable(ctx, session); err != nil {
		return err
	}
	if err := s.checkSizeLimit(ctx, session); err != nil {
		return err
	}
	if err := s.enforceSessionLimit(ctx, session); err != nil {
		return err
	}
	return s.store.Create(ctx, session)
}

// enforceSessionLimit ensures the user doesn't exceed MaxSessionsPerUser.
// If the limit is exceeded, oldest sessions are deleted to make room.
// When SiteID is configured, only counts sessions for this site.
func (s *ControlledStore) enforceSessionLimit(ctx context.Context, session *Session) error {
	if s.config.MaxSessionsPerUser <= 0 {
		return nil // No limit configured
	}

	// Check if the store supports listing sessions
	lister, ok := s.store.(SessionLister)
	if !ok {
		// Store doesn't support listing, can't enforce limit
		s.logger.DebugContext(ctx, "store does not implement SessionLister, cannot enforce session limit")
		return nil
	}

	userID := session.UserID.String()
	allSessions, err := lister.ListByUserID(ctx, userID)
	if err != nil {
		// Log but don't fail - best effort enforcement
		s.logger.WarnContext(ctx, "failed to list sessions for limit check",
			"user_id", userID,
			"error", err,
		)
		return nil
	}

	// Filter by SiteID if configured (only count sessions for this site)
	var sessions []*Session
	if s.config.SiteID != "" {
		sessions = make([]*Session, 0, len(allSessions))
		for _, sess := range allSessions {
			if sess.SiteID == s.config.SiteID {
				sessions = append(sessions, sess)
			}
		}
	} else {
		sessions = allSessions
	}

	// Calculate how many sessions to delete (sessions is sorted oldest first)
	// We need to make room for the new session, so we delete until we have maxSessions-1
	sessionsToDelete := len(sessions) - s.config.MaxSessionsPerUser + 1
	if sessionsToDelete <= 0 {
		return nil // Within limit
	}

	// Report violation (but don't fail - we'll evict old sessions)
	s.reportViolation(ctx, ViolationEvent{
		Type:      ViolationSessionLimitExceeded,
		SessionID: session.ID,
		UserID:    session.UserID,
		SiteID:    s.config.SiteID,
		Size:      len(sessions),
		Limit:     s.config.MaxSessionsPerUser,
		Message:   fmt.Sprintf("user has %d sessions, limit is %d, evicting %d oldest", len(sessions), s.config.MaxSessionsPerUser, sessionsToDelete),
	})

	// Delete oldest sessions to make room
	for i := 0; i < sessionsToDelete && i < len(sessions); i++ {
		oldSession := sessions[i]
		if err := s.store.Delete(ctx, oldSession.ID); err != nil {
			s.logger.WarnContext(ctx, "failed to evict old session",
				"session_id", oldSession.ID,
				"user_id", userID,
				"error", err,
			)
			// Continue trying to delete others
		} else {
			s.logger.InfoContext(ctx, "evicted old session to enforce limit",
				"session_id", oldSession.ID,
				"user_id", userID,
			)
		}
	}

	return nil
}

// Get retrieves a session by ID.
// If SiteID is configured, validates the session belongs to this site.
func (s *ControlledStore) Get(ctx context.Context, id string) (*Session, error) {
	session, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Validate SiteID if configured
	if s.config.SiteID != "" && session.SiteID != s.config.SiteID {
		s.reportViolation(ctx, ViolationEvent{
			Type:      ViolationSiteMismatch,
			SessionID: session.ID,
			UserID:    session.UserID,
			SiteID:    session.SiteID,
			Message:   fmt.Sprintf("session belongs to site %q, expected %q", session.SiteID, s.config.SiteID),
		})

		// Return ErrNotFound to avoid leaking that the session exists on another site
		return nil, ErrNotFound
	}

	return session, nil
}

// Update updates an existing session with validation.
func (s *ControlledStore) Update(ctx context.Context, session *Session) error {
	if err := s.validateSession(ctx, session); err != nil {
		return err
	}
	if err := s.validateSerializable(ctx, session); err != nil {
		return err
	}
	if err := s.checkSizeLimit(ctx, session); err != nil {
		return err
	}
	return s.store.Update(ctx, session)
}

// Delete removes a session by ID.
func (s *ControlledStore) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

// DeleteByUserID removes all sessions for a user.
func (s *ControlledStore) DeleteByUserID(ctx context.Context, userID string) (int, error) {
	return s.store.DeleteByUserID(ctx, userID)
}

// Touch updates the LastAccessedAt timestamp.
func (s *ControlledStore) Touch(ctx context.Context, id string) error {
	return s.store.Touch(ctx, id)
}

// Close closes the store.
func (s *ControlledStore) Close() error {
	return s.store.Close()
}

// Unwrap returns the underlying store.
func (s *ControlledStore) Unwrap() Store {
	return s.store
}

// ListByUserID returns all sessions for a user if the underlying store supports it.
// If SiteID is configured, only returns sessions for this site.
// This implements SessionLister for transparent pass-through.
func (s *ControlledStore) ListByUserID(ctx context.Context, userID string) ([]*Session, error) {
	lister, ok := s.store.(SessionLister)
	if !ok {
		return nil, fmt.Errorf("underlying store does not implement SessionLister")
	}

	sessions, err := lister.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Filter by SiteID if configured
	if s.config.SiteID == "" {
		return sessions, nil
	}

	filtered := make([]*Session, 0, len(sessions))
	for _, sess := range sessions {
		if sess.SiteID == s.config.SiteID {
			filtered = append(filtered, sess)
		}
	}
	return filtered, nil
}
