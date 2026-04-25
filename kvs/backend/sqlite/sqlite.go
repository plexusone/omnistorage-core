// Package sqlite provides a SQLite key-value storage backend.
//
// This is the recommended storage backend for persistent storage.
// Data is persisted to a local SQLite database file.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver (no CGO required)

	"github.com/plexusone/omnistorage-core/kvs"
)

// Verify interface compliance.
var (
	_ kvs.Store         = (*Store)(nil)
	_ kvs.ListableStore = (*Store)(nil)
)

// Store implements kvs.Store with SQLite.
type Store struct {
	db     *sql.DB
	path   string
	closed bool
}

// Config configures the SQLite storage.
type Config struct {
	// Path is the database file path.
	Path string
}

// New creates a new SQLite storage.
func New(cfg Config) (*Store, error) {
	if cfg.Path == "" {
		cfg.Path = "kvstore.db"
	}

	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", cfg.Path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Create tables
	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	s := &Store{
		db:   db,
		path: cfg.Path,
	}

	// Start background cleanup
	go s.cleanupLoop()

	return s, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS kv (
			key TEXT PRIMARY KEY,
			value BLOB NOT NULL,
			expires_at INTEGER
		);
		CREATE INDEX IF NOT EXISTS idx_kv_expires ON kv(expires_at) WHERE expires_at IS NOT NULL;
	`)
	return err
}

// Get retrieves a value by key.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if s.closed {
		return nil, kvs.ErrClosed
	}

	var value []byte
	var expiresAt sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		"SELECT value, expires_at FROM kv WHERE key = ?",
		key,
	).Scan(&value, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, kvs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	// Check expiration
	if expiresAt.Valid && time.Now().Unix() > expiresAt.Int64 {
		// Expired, delete and return not found
		_ = s.Delete(ctx, key)
		return nil, kvs.ErrNotFound
	}

	return value, nil
}

// Set stores a value with an optional TTL.
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s.closed {
		return kvs.ErrClosed
	}

	var expiresAt sql.NullInt64
	if ttl > 0 {
		expiresAt = sql.NullInt64{
			Int64: time.Now().Add(ttl).Unix(),
			Valid: true,
		}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO kv (key, value, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, expires_at = excluded.expires_at`,
		key, value, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	return nil
}

// Delete removes a key.
func (s *Store) Delete(ctx context.Context, key string) error {
	if s.closed {
		return kvs.ErrClosed
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM kv WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

// List returns all keys matching the given prefix.
func (s *Store) List(ctx context.Context, prefix string) ([]string, error) {
	if s.closed {
		return nil, kvs.ErrClosed
	}

	now := time.Now().Unix()

	var rows *sql.Rows
	var err error

	if prefix == "" {
		rows, err = s.db.QueryContext(ctx,
			"SELECT key FROM kv WHERE expires_at IS NULL OR expires_at > ? ORDER BY key",
			now,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			"SELECT key FROM kv WHERE key LIKE ? AND (expires_at IS NULL OR expires_at > ?) ORDER BY key",
			prefix+"%", now,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	return keys, nil
}

// Close releases storage resources.
func (s *Store) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true
	return s.db.Close()
}

// cleanupLoop periodically removes expired entries.
func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if s.closed {
			return
		}
		s.removeExpired()
	}
}

func (s *Store) removeExpired() {
	if s.closed {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _ = s.db.ExecContext(ctx,
		"DELETE FROM kv WHERE expires_at IS NOT NULL AND expires_at < ?",
		time.Now().Unix(),
	)
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.path
}
