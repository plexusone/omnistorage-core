// Package session provides a session storage interface with controls.
//
// Session storage extends key-value storage with session-specific operations:
//   - User-based session lookups and deletion
//   - Automatic TTL management
//   - Size limits and validation
//   - Touch/heartbeat operations
//
// # Available Backends
//
//   - memory: In-memory storage (for development/testing)
//   - kvs: Adapts any kvs.ListableStore (Redis, SQLite, etc.)
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrNotFound             = errors.New("session not found")
	ErrExpired              = errors.New("session expired")
	ErrInvalidSession       = errors.New("invalid session")
	ErrSizeLimitExceeded    = errors.New("session size limit exceeded")
	ErrSessionLimitExceeded = errors.New("session limit exceeded")
	ErrSiteMismatch         = errors.New("session belongs to different site")
	ErrClosed               = errors.New("store is closed")
)

// Session represents a server-side session.
type Session struct {
	// ID is the unique session identifier.
	ID string `json:"id"`

	// UserID is the authenticated user's ID.
	UserID uuid.UUID `json:"user_id"`

	// SiteID identifies which site/service this session belongs to.
	// Used for multi-site isolation (e.g., "academyos", "agentos", "dashforge").
	// Set automatically by ControlledStore based on Config.SiteID.
	SiteID string `json:"site_id,omitempty"`

	// OrganizationID is the current organization context (optional).
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`

	// Data contains arbitrary session data.
	// This can include tokens, preferences, etc.
	Data map[string]any `json:"data,omitempty"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the session was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// LastAccessedAt is when the session was last accessed.
	LastAccessedAt time.Time `json:"last_accessed_at"`

	// ExpiresAt is when the session expires.
	ExpiresAt time.Time `json:"expires_at"`

	// IPAddress is the IP address that created the session.
	IPAddress string `json:"ip_address,omitempty"`

	// UserAgent is the user agent that created the session.
	UserAgent string `json:"user_agent,omitempty"`
}

// IsExpired returns true if the session has expired.
func (s *Session) IsExpired() bool {
	return !s.ExpiresAt.IsZero() && time.Now().After(s.ExpiresAt)
}

// TTL returns the remaining time until expiration.
func (s *Session) TTL() time.Duration {
	if s.ExpiresAt.IsZero() {
		return 0
	}
	ttl := time.Until(s.ExpiresAt)
	if ttl < 0 {
		return 0
	}
	return ttl
}

// SessionIDLength is the length of generated session IDs in bytes.
const SessionIDLength = 32

// GenerateSessionID generates a cryptographically secure session ID.
func GenerateSessionID() (string, error) {
	bytes := make([]byte, SessionIDLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// NewSession creates a new session with the given parameters.
func NewSession(userID uuid.UUID, ttl time.Duration) (*Session, error) {
	id, err := GenerateSessionID()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	session := &Session{
		ID:             id,
		UserID:         userID,
		Data:           make(map[string]any),
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: now,
	}

	if ttl > 0 {
		session.ExpiresAt = now.Add(ttl)
	}

	return session, nil
}

// Store defines the interface for session storage.
// Implementations must be safe for concurrent use.
type Store interface {
	// Create stores a new session.
	Create(ctx context.Context, session *Session) error

	// Get retrieves a session by ID.
	// Returns ErrNotFound if the session doesn't exist.
	// Returns ErrExpired if the session has expired.
	Get(ctx context.Context, id string) (*Session, error)

	// Update updates an existing session.
	// Returns ErrNotFound if the session doesn't exist.
	Update(ctx context.Context, session *Session) error

	// Delete removes a session by ID.
	Delete(ctx context.Context, id string) error

	// DeleteByUserID removes all sessions for a user.
	// Returns the number of sessions deleted.
	DeleteByUserID(ctx context.Context, userID string) (int, error)

	// Touch updates the LastAccessedAt timestamp.
	// This is used to track session activity without modifying other fields.
	Touch(ctx context.Context, id string) error

	// Close closes the store and releases any resources.
	Close() error
}

// Config contains configuration for session stores.
type Config struct {
	// SiteID identifies which site/service this store handles.
	// Used for multi-site isolation (e.g., "academyos", "agentos", "dashforge").
	// When set, all sessions are tagged with this SiteID and Get operations
	// validate the session belongs to this site.
	// Required for ControlledStore when running multiple sites.
	SiteID string

	// MaxSessionSize is the maximum serialized session size in bytes.
	// Set to 0 for no limit.
	MaxSessionSize int

	// MaxSessionsPerUser limits concurrent sessions per user.
	// When exceeded, oldest sessions are deleted to make room.
	// Set to 0 for no limit.
	MaxSessionsPerUser int

	// DefaultTTL is the default session TTL if not specified.
	DefaultTTL time.Duration

	// CleanupInterval is how often to run automatic cleanup.
	// Set to 0 to disable automatic cleanup.
	CleanupInterval time.Duration

	// KeyPrefix is the prefix for session keys (for KVS backends).
	KeyPrefix string
}

// SessionLister is an optional interface for stores that support listing sessions.
// Stores implementing this interface can enforce MaxSessionsPerUser.
type SessionLister interface {
	// ListByUserID returns all sessions for a user, sorted by CreatedAt ascending (oldest first).
	ListByUserID(ctx context.Context, userID string) ([]*Session, error)
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		MaxSessionSize:  1024 * 1024, // 1MB
		DefaultTTL:      24 * time.Hour,
		CleanupInterval: 5 * time.Minute,
		KeyPrefix:       "session:",
	}
}
