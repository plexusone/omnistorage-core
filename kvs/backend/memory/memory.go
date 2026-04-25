// Package memory provides an in-memory key-value storage backend.
//
// This is the default storage backend, suitable for development
// and testing. Data is not persisted across restarts.
package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/plexusone/omnistorage-core/kvs"
)

// Verify interface compliance.
var (
	_ kvs.Store         = (*Store)(nil)
	_ kvs.ListableStore = (*Store)(nil)
)

// Store implements kvs.Store with in-memory maps.
type Store struct {
	mu      sync.RWMutex
	data    map[string]entry
	closed  bool
	closeCh chan struct{}
}

type entry struct {
	value     []byte
	expiresAt time.Time
}

// New creates a new in-memory storage.
func New() *Store {
	s := &Store{
		data:    make(map[string]entry),
		closeCh: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go s.cleanup()

	return s
}

// Get retrieves a value by key.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, kvs.ErrClosed
	}

	e, ok := s.data[key]
	if !ok {
		return nil, kvs.ErrNotFound
	}

	// Check expiration
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		return nil, kvs.ErrNotFound
	}

	// Return a copy to prevent mutation
	result := make([]byte, len(e.value))
	copy(result, e.value)
	return result, nil
}

// Set stores a value with an optional TTL.
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return kvs.ErrClosed
	}

	// Copy value to prevent external mutation
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	e := entry{value: valueCopy}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}

	s.data[key] = e
	return nil
}

// Delete removes a key.
func (s *Store) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return kvs.ErrClosed
	}

	delete(s.data, key)
	return nil
}

// List returns all keys matching the given prefix.
func (s *Store) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, kvs.ErrClosed
	}

	now := time.Now()
	var keys []string
	for key, e := range s.data {
		// Skip expired entries
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			continue
		}
		// Filter by prefix
		if prefix == "" || strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}

	sort.Strings(keys)
	return keys, nil
}

// Close releases storage resources.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	close(s.closeCh)
	s.data = nil
	return nil
}

// cleanup periodically removes expired entries.
func (s *Store) cleanup() {
	ticker := time.NewTicker(time.Minute)
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
	for key, e := range s.data {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			delete(s.data, key)
		}
	}
}

// Len returns the number of entries (for testing).
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}
