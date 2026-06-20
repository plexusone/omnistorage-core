package session_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/plexusone/omnistorage-core/session"
	sessionmemory "github.com/plexusone/omnistorage-core/session/backend/memory"
)

func TestControlledStore_SizeLimit(t *testing.T) {
	cfg := session.Config{
		MaxSessionSize: 1024, // 1KB limit
		DefaultTTL:     time.Hour,
	}
	store := sessionmemory.NewWithControls(cfg)
	defer store.Close()

	ctx := context.Background()
	userID := uuid.New()

	t.Run("session within limit succeeds", func(t *testing.T) {
		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}
		sess.Data["small"] = "value"

		err = store.Create(ctx, sess)
		if err != nil {
			t.Errorf("Create should succeed for small session: %v", err)
		}
	})

	t.Run("session exceeding limit fails", func(t *testing.T) {
		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}
		// Add large data to exceed 1KB limit
		sess.Data["large"] = strings.Repeat("x", 2000)

		err = store.Create(ctx, sess)
		if err == nil {
			t.Error("Create should fail for oversized session")
		}

		code := session.ErrorCode(err)
		if code != session.ErrCodeSizeLimitExceeded {
			t.Errorf("Expected error code %s, got %s", session.ErrCodeSizeLimitExceeded, code)
		}
	})

	t.Run("update exceeding limit fails", func(t *testing.T) {
		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}
		sess.Data["small"] = "value"

		err = store.Create(ctx, sess)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Now try to update with large data
		sess.Data["large"] = strings.Repeat("x", 2000)
		err = store.Update(ctx, sess)
		if err == nil {
			t.Error("Update should fail for oversized session")
		}

		code := session.ErrorCode(err)
		if code != session.ErrCodeSizeLimitExceeded {
			t.Errorf("Expected error code %s, got %s", session.ErrCodeSizeLimitExceeded, code)
		}
	})
}

func TestControlledStore_JSONValidation(t *testing.T) {
	cfg := session.Config{
		DefaultTTL: time.Hour,
	}
	store := sessionmemory.NewWithControls(cfg)
	defer store.Close()

	ctx := context.Background()
	userID := uuid.New()

	t.Run("valid JSON data succeeds", func(t *testing.T) {
		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}
		sess.Data["string"] = "value"
		sess.Data["number"] = 42
		sess.Data["bool"] = true
		sess.Data["nested"] = map[string]any{"key": "value"}

		err = store.Create(ctx, sess)
		if err != nil {
			t.Errorf("Create should succeed for valid JSON data: %v", err)
		}
	})

	t.Run("nil data succeeds", func(t *testing.T) {
		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}
		sess.Data = nil

		err = store.Create(ctx, sess)
		if err != nil {
			t.Errorf("Create should succeed for nil data: %v", err)
		}
	})

	t.Run("function in data fails", func(t *testing.T) {
		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}
		sess.Data["func"] = func() {} // Functions are not JSON serializable

		err = store.Create(ctx, sess)
		if err == nil {
			t.Error("Create should fail for non-serializable data")
		}

		code := session.ErrorCode(err)
		if code != session.ErrCodeNotSerializable {
			t.Errorf("Expected error code %s, got %s", session.ErrCodeNotSerializable, code)
		}
	})

	t.Run("channel in data fails", func(t *testing.T) {
		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}
		sess.Data["chan"] = make(chan int) // Channels are not JSON serializable

		err = store.Create(ctx, sess)
		if err == nil {
			t.Error("Create should fail for non-serializable data")
		}

		code := session.ErrorCode(err)
		if code != session.ErrCodeNotSerializable {
			t.Errorf("Expected error code %s, got %s", session.ErrCodeNotSerializable, code)
		}
	})
}

func TestControlledStore_SessionValidation(t *testing.T) {
	cfg := session.Config{
		DefaultTTL: time.Hour,
	}
	store := sessionmemory.NewWithControls(cfg)
	defer store.Close()

	ctx := context.Background()

	t.Run("nil session fails", func(t *testing.T) {
		err := store.Create(ctx, nil)
		if err == nil {
			t.Error("Create should fail for nil session")
		}

		code := session.ErrorCode(err)
		if code != session.ErrCodeInvalidSession {
			t.Errorf("Expected error code %s, got %s", session.ErrCodeInvalidSession, code)
		}
	})

	t.Run("empty session ID fails", func(t *testing.T) {
		sess := &session.Session{
			UserID: uuid.New(),
		}

		err := store.Create(ctx, sess)
		if err == nil {
			t.Error("Create should fail for empty session ID")
		}

		code := session.ErrorCode(err)
		if code != session.ErrCodeInvalidSession {
			t.Errorf("Expected error code %s, got %s", session.ErrCodeInvalidSession, code)
		}
	})

	t.Run("empty user ID fails", func(t *testing.T) {
		sess := &session.Session{
			ID: "test-session-id",
		}

		err := store.Create(ctx, sess)
		if err == nil {
			t.Error("Create should fail for empty user ID")
		}

		code := session.ErrorCode(err)
		if code != session.ErrCodeInvalidSession {
			t.Errorf("Expected error code %s, got %s", session.ErrCodeInvalidSession, code)
		}
	})
}

func TestControlledStore_ViolationHandler(t *testing.T) {
	var violations []session.ViolationEvent

	cfg := session.Config{
		MaxSessionSize: 100, // Very small limit
		DefaultTTL:     time.Hour,
	}

	rawStore := sessionmemory.New(cfg)
	store := session.WithControls(rawStore, cfg,
		session.WithViolationHandler(func(ctx context.Context, event session.ViolationEvent) {
			violations = append(violations, event)
		}),
	)
	defer store.Close()

	ctx := context.Background()
	userID := uuid.New()

	t.Run("size violation triggers handler", func(t *testing.T) {
		violations = nil // Reset

		sess, _ := session.NewSession(userID, time.Hour)
		sess.Data["large"] = strings.Repeat("x", 200)

		_ = store.Create(ctx, sess)

		if len(violations) == 0 {
			t.Error("Violation handler should have been called")
		}

		if len(violations) > 0 && violations[0].Type != session.ViolationSizeExceeded {
			t.Errorf("Expected violation type %s, got %s", session.ViolationSizeExceeded, violations[0].Type)
		}
	})

	t.Run("serialization violation triggers handler", func(t *testing.T) {
		violations = nil // Reset

		sess, _ := session.NewSession(userID, time.Hour)
		sess.Data["func"] = func() {}

		_ = store.Create(ctx, sess)

		if len(violations) == 0 {
			t.Error("Violation handler should have been called")
		}

		if len(violations) > 0 && violations[0].Type != session.ViolationNotSerializable {
			t.Errorf("Expected violation type %s, got %s", session.ViolationNotSerializable, violations[0].Type)
		}
	})

	t.Run("invalid session triggers handler", func(t *testing.T) {
		violations = nil // Reset

		_ = store.Create(ctx, nil)

		if len(violations) == 0 {
			t.Error("Violation handler should have been called")
		}

		if len(violations) > 0 && violations[0].Type != session.ViolationInvalidSession {
			t.Errorf("Expected violation type %s, got %s", session.ViolationInvalidSession, violations[0].Type)
		}
	})
}

func TestSessionError(t *testing.T) {
	t.Run("error code extraction", func(t *testing.T) {
		err := session.NewSessionError(
			session.ErrCodeSizeLimitExceeded,
			"test message",
			map[string]any{"size": 1000},
			nil,
		)

		code := session.ErrorCode(err)
		if code != session.ErrCodeSizeLimitExceeded {
			t.Errorf("Expected code %s, got %s", session.ErrCodeSizeLimitExceeded, code)
		}
	})

	t.Run("error details extraction", func(t *testing.T) {
		details := map[string]any{"size": 1000, "limit": 500}
		err := session.NewSessionError(
			session.ErrCodeSizeLimitExceeded,
			"test message",
			details,
			nil,
		)

		extracted := session.ErrorDetails(err)
		if extracted["size"] != 1000 {
			t.Errorf("Expected size 1000, got %v", extracted["size"])
		}
	})

	t.Run("non-session error returns empty code", func(t *testing.T) {
		var v int
		err := json.Unmarshal([]byte("invalid"), &v)
		code := session.ErrorCode(err)
		if code != "" {
			t.Errorf("Expected empty code for non-session error, got %s", code)
		}
	})
}

func TestNewSession(t *testing.T) {
	userID := uuid.New()

	t.Run("creates valid session", func(t *testing.T) {
		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}

		if sess.ID == "" {
			t.Error("Session ID should not be empty")
		}
		if sess.UserID != userID {
			t.Errorf("Expected user ID %s, got %s", userID, sess.UserID)
		}
		if sess.Data == nil {
			t.Error("Session Data should be initialized")
		}
		if sess.ExpiresAt.IsZero() {
			t.Error("Session ExpiresAt should be set")
		}
	})

	t.Run("session ID is unique", func(t *testing.T) {
		sess1, _ := session.NewSession(userID, time.Hour)
		sess2, _ := session.NewSession(userID, time.Hour)

		if sess1.ID == sess2.ID {
			t.Error("Session IDs should be unique")
		}
	})
}

func TestControlledStore_MaxSessionsPerUser(t *testing.T) {
	cfg := session.Config{
		MaxSessionsPerUser: 3,
		DefaultTTL:         time.Hour,
	}
	store := sessionmemory.NewWithControls(cfg)
	defer store.Close()

	ctx := context.Background()
	userID := uuid.New()

	t.Run("sessions within limit are allowed", func(t *testing.T) {
		// Create 3 sessions (at the limit)
		for i := 0; i < 3; i++ {
			sess, err := session.NewSession(userID, time.Hour)
			if err != nil {
				t.Fatalf("NewSession failed: %v", err)
			}
			// Add small delay to ensure different CreatedAt times
			time.Sleep(10 * time.Millisecond)
			if err := store.Create(ctx, sess); err != nil {
				t.Errorf("Create should succeed within limit: %v", err)
			}
		}
	})

	t.Run("oldest session evicted when limit exceeded", func(t *testing.T) {
		// List current sessions
		lister := store.(session.SessionLister)
		sessions, err := lister.ListByUserID(ctx, userID.String())
		if err != nil {
			t.Fatalf("ListByUserID failed: %v", err)
		}
		if len(sessions) != 3 {
			t.Fatalf("Expected 3 sessions, got %d", len(sessions))
		}

		// Remember the oldest session ID
		oldestID := sessions[0].ID

		// Create a 4th session - should evict the oldest
		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}
		if err := store.Create(ctx, sess); err != nil {
			t.Errorf("Create should succeed by evicting old session: %v", err)
		}

		// Verify we still have 3 sessions
		sessions, err = lister.ListByUserID(ctx, userID.String())
		if err != nil {
			t.Fatalf("ListByUserID failed: %v", err)
		}
		if len(sessions) != 3 {
			t.Errorf("Expected 3 sessions after eviction, got %d", len(sessions))
		}

		// Verify the oldest session was evicted
		for _, s := range sessions {
			if s.ID == oldestID {
				t.Error("Oldest session should have been evicted")
			}
		}

		// Verify the new session exists
		found := false
		for _, s := range sessions {
			if s.ID == sess.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("New session should exist")
		}
	})

	t.Run("different users have independent limits", func(t *testing.T) {
		otherUserID := uuid.New()

		// Create 3 sessions for another user
		for i := 0; i < 3; i++ {
			sess, err := session.NewSession(otherUserID, time.Hour)
			if err != nil {
				t.Fatalf("NewSession failed: %v", err)
			}
			if err := store.Create(ctx, sess); err != nil {
				t.Errorf("Create should succeed for different user: %v", err)
			}
		}

		// Verify each user has 3 sessions
		lister := store.(session.SessionLister)

		sessions1, _ := lister.ListByUserID(ctx, userID.String())
		if len(sessions1) != 3 {
			t.Errorf("Expected 3 sessions for first user, got %d", len(sessions1))
		}

		sessions2, _ := lister.ListByUserID(ctx, otherUserID.String())
		if len(sessions2) != 3 {
			t.Errorf("Expected 3 sessions for second user, got %d", len(sessions2))
		}
	})
}

func TestControlledStore_MaxSessionsViolationHandler(t *testing.T) {
	var violations []session.ViolationEvent

	cfg := session.Config{
		MaxSessionsPerUser: 2,
		DefaultTTL:         time.Hour,
	}

	rawStore := sessionmemory.New(cfg)
	store := session.WithControls(rawStore, cfg,
		session.WithViolationHandler(func(ctx context.Context, event session.ViolationEvent) {
			violations = append(violations, event)
		}),
	)
	defer store.Close()

	ctx := context.Background()
	userID := uuid.New()

	// Create 2 sessions (at limit)
	for i := 0; i < 2; i++ {
		sess, _ := session.NewSession(userID, time.Hour)
		time.Sleep(10 * time.Millisecond)
		_ = store.Create(ctx, sess)
	}

	violations = nil // Reset

	// Create 3rd session - should trigger violation and eviction
	sess, _ := session.NewSession(userID, time.Hour)
	_ = store.Create(ctx, sess)

	if len(violations) == 0 {
		t.Error("Violation handler should have been called")
	}

	if len(violations) > 0 && violations[0].Type != session.ViolationSessionLimitExceeded {
		t.Errorf("Expected violation type %s, got %s", session.ViolationSessionLimitExceeded, violations[0].Type)
	}
}

func TestControlledStore_SiteID(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Create sets SiteID from config", func(t *testing.T) {
		cfg := session.Config{
			SiteID:     "academyos",
			DefaultTTL: time.Hour,
		}
		rawStore := sessionmemory.New(cfg)
		store := session.WithControls(rawStore, cfg)
		defer store.Close()

		sess, err := session.NewSession(userID, time.Hour)
		if err != nil {
			t.Fatalf("NewSession failed: %v", err)
		}

		// SiteID should be empty before Create
		if sess.SiteID != "" {
			t.Errorf("SiteID should be empty before Create, got %q", sess.SiteID)
		}

		err = store.Create(ctx, sess)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// SiteID should be set after Create
		if sess.SiteID != "academyos" {
			t.Errorf("Expected SiteID %q, got %q", "academyos", sess.SiteID)
		}

		// Verify it's persisted
		retrieved, err := store.Get(ctx, sess.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if retrieved.SiteID != "academyos" {
			t.Errorf("Expected persisted SiteID %q, got %q", "academyos", retrieved.SiteID)
		}
	})

	t.Run("Get rejects sessions from different site", func(t *testing.T) {
		// Create a session with site "agentos" using raw store
		rawStore := sessionmemory.New(session.Config{DefaultTTL: time.Hour})

		sess, _ := session.NewSession(userID, time.Hour)
		sess.SiteID = "agentos"
		_ = rawStore.Create(ctx, sess)

		// Create controlled store for "academyos"
		store := session.WithControls(rawStore, session.Config{
			SiteID:     "academyos",
			DefaultTTL: time.Hour,
		})
		defer store.Close()

		// Get should return ErrNotFound (not leak that session exists)
		_, err := store.Get(ctx, sess.ID)
		if err == nil {
			t.Error("Get should fail for session from different site")
		}
		if err != session.ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Get allows sessions from same site", func(t *testing.T) {
		cfg := session.Config{
			SiteID:     "dashforge",
			DefaultTTL: time.Hour,
		}
		store := sessionmemory.NewWithControls(cfg)
		defer store.Close()

		sess, _ := session.NewSession(userID, time.Hour)
		_ = store.Create(ctx, sess)

		retrieved, err := store.Get(ctx, sess.ID)
		if err != nil {
			t.Errorf("Get should succeed for session from same site: %v", err)
		}
		if retrieved.SiteID != "dashforge" {
			t.Errorf("Expected SiteID %q, got %q", "dashforge", retrieved.SiteID)
		}
	})

	t.Run("ListByUserID filters by site", func(t *testing.T) {
		// Create raw store with sessions from multiple sites
		rawStore := sessionmemory.New(session.Config{DefaultTTL: time.Hour})

		// Create sessions for different sites
		for _, site := range []string{"academyos", "academyos", "agentos", "dashforge"} {
			sess, _ := session.NewSession(userID, time.Hour)
			sess.SiteID = site
			_ = rawStore.Create(ctx, sess)
		}

		// Create controlled store for "academyos"
		store := session.WithControls(rawStore, session.Config{
			SiteID:     "academyos",
			DefaultTTL: time.Hour,
		})

		// ListByUserID should only return academyos sessions
		sessions, err := store.ListByUserID(ctx, userID.String())
		if err != nil {
			t.Fatalf("ListByUserID failed: %v", err)
		}

		if len(sessions) != 2 {
			t.Errorf("Expected 2 sessions for academyos, got %d", len(sessions))
		}

		for _, sess := range sessions {
			if sess.SiteID != "academyos" {
				t.Errorf("Expected SiteID %q, got %q", "academyos", sess.SiteID)
			}
		}
	})

	t.Run("MaxSessionsPerUser respects site isolation", func(t *testing.T) {
		// Create raw store with sessions from different sites
		rawStore := sessionmemory.New(session.Config{DefaultTTL: time.Hour})

		// Create 2 sessions for "agentos"
		for i := 0; i < 2; i++ {
			sess, _ := session.NewSession(userID, time.Hour)
			sess.SiteID = "agentos"
			time.Sleep(10 * time.Millisecond)
			_ = rawStore.Create(ctx, sess)
		}

		// Create controlled store for "academyos" with limit of 2
		store := session.WithControls(rawStore, session.Config{
			SiteID:             "academyos",
			MaxSessionsPerUser: 2,
			DefaultTTL:         time.Hour,
		})

		// Creating 2 sessions for academyos should work (agentos sessions don't count)
		for i := 0; i < 2; i++ {
			sess, _ := session.NewSession(userID, time.Hour)
			time.Sleep(10 * time.Millisecond)
			err := store.Create(ctx, sess)
			if err != nil {
				t.Errorf("Create should succeed within site limit: %v", err)
			}
		}

		// Verify we have 4 total sessions (2 agentos + 2 academyos)
		allSessions, _ := rawStore.ListByUserID(ctx, userID.String())
		if len(allSessions) != 4 {
			t.Errorf("Expected 4 total sessions, got %d", len(allSessions))
		}
	})
}

func TestControlledStore_SiteIDViolationHandler(t *testing.T) {
	var violations []session.ViolationEvent

	// Create raw store with a session from "agentos"
	rawStore := sessionmemory.New(session.Config{DefaultTTL: time.Hour})
	ctx := context.Background()
	userID := uuid.New()

	sess, _ := session.NewSession(userID, time.Hour)
	sess.SiteID = "agentos"
	_ = rawStore.Create(ctx, sess)

	// Create controlled store for "academyos" with violation handler
	store := session.WithControls(rawStore, session.Config{
		SiteID:     "academyos",
		DefaultTTL: time.Hour,
	}, session.WithViolationHandler(func(ctx context.Context, event session.ViolationEvent) {
		violations = append(violations, event)
	}))
	defer store.Close()

	// Try to get the agentos session - should trigger violation
	_, _ = store.Get(ctx, sess.ID)

	if len(violations) == 0 {
		t.Error("Violation handler should have been called")
	}

	if len(violations) > 0 {
		if violations[0].Type != session.ViolationSiteMismatch {
			t.Errorf("Expected violation type %s, got %s", session.ViolationSiteMismatch, violations[0].Type)
		}
		if violations[0].SiteID != "agentos" {
			t.Errorf("Expected violation SiteID %q, got %q", "agentos", violations[0].SiteID)
		}
	}
}
