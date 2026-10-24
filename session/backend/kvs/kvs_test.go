package kvs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omnistorage-core/kvs"
	"github.com/plexusone/omnistorage-core/kvs/backend/memory"
	"github.com/plexusone/omnistorage-core/session"
	sessionkvs "github.com/plexusone/omnistorage-core/session/backend/kvs"
)

func newSession(t *testing.T, s *sessionkvs.Store, userID uuid.UUID) *session.Session {
	t.Helper()
	sess, err := session.NewSession(userID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	return sess
}

// A session ID comes from a client cookie. One that names the user index
// key ("user:<id>") must not delete or read the index: deleting it would
// hide the user's sessions from DeleteByUserID, so "log out everywhere"
// would miss them.
func TestCraftedIDCannotReachTheUserIndex(t *testing.T) {
	ctx := context.Background()
	s := sessionkvs.New(memory.New(), session.Config{})
	victim := uuid.New()
	sess := newSession(t, s, victim)

	crafted := "user:" + victim.String()
	if err := s.Delete(ctx, crafted); err != nil {
		t.Fatalf("Delete(crafted): %v", err)
	}
	if _, err := s.Get(ctx, crafted); !errors.Is(err, session.ErrNotFound) {
		t.Errorf("Get(crafted) = %v, want ErrNotFound", err)
	}

	n, err := s.DeleteByUserID(ctx, victim.String())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("DeleteByUserID removed %d sessions, want 1: the index was lost", n)
	}
	if _, err := s.Get(ctx, sess.ID); !errors.Is(err, session.ErrNotFound) {
		t.Errorf("victim session survived DeleteByUserID: %v", err)
	}
}

func TestInvalidSessionIDsAreRefused(t *testing.T) {
	ctx := context.Background()
	s := sessionkvs.New(memory.New(), session.Config{})
	for _, id := range []string{"", "user:x", "a b", "a/b", string(make([]byte, session.MaxSessionIDLength+1))} {
		sess := &session.Session{ID: id, UserID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)}
		if err := s.Create(ctx, sess); !errors.Is(err, session.ErrInvalidSession) {
			t.Errorf("Create(%q) = %v, want ErrInvalidSession", id, err)
		}
		if err := s.Update(ctx, sess); !errors.Is(err, session.ErrInvalidSession) {
			t.Errorf("Update(%q) = %v, want ErrInvalidSession", id, err)
		}
	}
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	s := sessionkvs.New(memory.New(), session.Config{})
	user := uuid.New()
	sess := newSession(t, s, user)

	got, err := s.Get(ctx, sess.ID)
	if err != nil || got.ID != sess.ID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if err := s.Delete(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, sess.ID); !errors.Is(err, session.ErrNotFound) {
		t.Errorf("deleted session still found: %v", err)
	}
	if err := s.Delete(ctx, sess.ID); err != nil {
		t.Errorf("deleting a missing session: %v", err)
	}
}

func TestValidSessionID(t *testing.T) {
	id, err := session.GenerateSessionID()
	if err != nil {
		t.Fatal(err)
	}
	for _, ok := range []string{id, "abc-DEF_123.x"} {
		if !session.ValidSessionID(ok) {
			t.Errorf("ValidSessionID(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "user:1", "a b", "a\x00b", "é"} {
		if session.ValidSessionID(bad) {
			t.Errorf("ValidSessionID(%q) = true", bad)
		}
	}
}

// flakyIndex fails reads of user index keys while failing is set.
type flakyIndex struct {
	kvs.ListableStore
	failing bool
}

var errIndexRead = errors.New("index read failed")

func (f *flakyIndex) Get(ctx context.Context, key string) ([]byte, error) {
	if f.failing && strings.Contains(key, "user:") {
		return nil, errIndexRead
	}
	return f.ListableStore.Get(ctx, key)
}

// A failed read of the user index must not let Create overwrite the index
// with only the new session, which would hide the user's other sessions
// from DeleteByUserID. The failure is logged and Create still succeeds.
func TestCreateKeepsIndexWhenIndexReadFails(t *testing.T) {
	ctx := context.Background()
	backend := &flakyIndex{ListableStore: memory.New()}
	s := sessionkvs.New(backend, session.Config{})
	user := uuid.New()

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	first := newSession(t, s, user)

	backend.failing = true
	second := newSession(t, s, user)
	backend.failing = false

	if !strings.Contains(logs.String(), "failed to add session to user index") {
		t.Fatalf("index failure was not logged; logs: %s", logs.String())
	}
	if strings.Contains(logs.String(), second.ID) {
		t.Fatal("log contains the session ID")
	}

	// The first session must still be reachable through the index.
	n, err := s.DeleteByUserID(ctx, user.String())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("DeleteByUserID deleted %d, want 1", n)
	}
	if _, err := s.Get(ctx, first.ID); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("first session survived DeleteByUserID: err = %v", err)
	}
}
