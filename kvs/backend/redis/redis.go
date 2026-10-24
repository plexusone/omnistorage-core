// Package redis provides a Redis-backed key-value storage backend.
//
// This backend is suitable for production deployments requiring
// persistence and multi-server support.
package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/plexusone/omnistorage-core/kvs"
	"github.com/redis/go-redis/v9"
)

// Verify interface compliance.
var (
	_ kvs.Store         = (*Store)(nil)
	_ kvs.ListableStore = (*Store)(nil)
)

// Config configures the Redis store.
type Config struct {
	// URL is the Redis connection URL (e.g., "redis://localhost:6379").
	URL string

	// KeyPrefix is prepended to all keys (default: "").
	KeyPrefix string

	// PoolSize is the maximum number of connections (default: 10).
	PoolSize int

	// ConnectTimeout is the connection timeout (default: 5s).
	ConnectTimeout time.Duration

	// AllowInsecure permits connecting to a non-loopback Redis server
	// without TLS, without a password, or with TLS certificate
	// verification disabled (skip_verify). By default New refuses such
	// connections with ErrInsecureConnection, because session data and
	// tokens would cross the network in plaintext or be readable by
	// anyone who can reach the server. Loopback addresses and Unix
	// sockets are always allowed. Set this only for trusted networks.
	AllowInsecure bool
}

// ErrInsecureConnection is returned by New when the Redis URL names a
// non-loopback server and does not use TLS (rediss://) with certificate
// verification and a password, and Config.AllowInsecure is false.
var ErrInsecureConnection = errors.New("insecure redis connection")

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		PoolSize:       10,
		ConnectTimeout: 5 * time.Second,
	}
}

// Store implements kvs.Store with Redis.
type Store struct {
	client    *redis.Client
	keyPrefix string
	closed    bool
}

// New creates a new Redis storage backend.
func New(cfg Config) (*Store, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("redis URL is required")
	}

	opt, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	if !cfg.AllowInsecure {
		if err := checkSecure(opt); err != nil {
			return nil, err
		}
	}

	if cfg.PoolSize > 0 {
		opt.PoolSize = cfg.PoolSize
	}

	client := redis.NewClient(opt)

	// Test connection
	timeout := cfg.ConnectTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &Store{
		client:    client,
		keyPrefix: cfg.KeyPrefix,
	}, nil
}

// checkSecure returns ErrInsecureConnection unless opt connects over a
// Unix socket or to a loopback address, or uses verified TLS and a
// password.
func checkSecure(opt *redis.Options) error {
	if opt.Network == "unix" {
		return nil
	}
	host, _, err := net.SplitHostPort(opt.Addr)
	if err != nil {
		host = opt.Addr
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	switch {
	case opt.TLSConfig == nil:
		return fmt.Errorf("%w: %s does not use TLS (use rediss://, or set AllowInsecure)", ErrInsecureConnection, host)
	case opt.TLSConfig.InsecureSkipVerify:
		return fmt.Errorf("%w: %s skips TLS certificate verification (remove skip_verify, or set AllowInsecure)", ErrInsecureConnection, host)
	case opt.Password == "":
		return fmt.Errorf("%w: %s has no password (add credentials to the URL, or set AllowInsecure)", ErrInsecureConnection, host)
	}
	return nil
}

// key returns the full Redis key with prefix.
func (s *Store) key(k string) string {
	return s.keyPrefix + k
}

// Get retrieves a value by key.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if s.closed {
		return nil, kvs.ErrClosed
	}

	data, err := s.client.Get(ctx, s.key(key)).Bytes()
	if err == redis.Nil {
		return nil, kvs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	return data, nil
}

// Set stores a value with an optional TTL.
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s.closed {
		return kvs.ErrClosed
	}

	err := s.client.Set(ctx, s.key(key), value, ttl).Err()
	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

// Delete removes a key.
func (s *Store) Delete(ctx context.Context, key string) error {
	if s.closed {
		return kvs.ErrClosed
	}

	err := s.client.Del(ctx, s.key(key)).Err()
	if err != nil {
		return fmt.Errorf("redis delete: %w", err)
	}

	return nil
}

// List returns all keys matching the given prefix.
func (s *Store) List(ctx context.Context, prefix string) ([]string, error) {
	if s.closed {
		return nil, kvs.ErrClosed
	}

	// Build the full pattern. The prefix is literal, so escape glob
	// metacharacters before appending the wildcard; otherwise a prefix
	// such as "user:*" would match keys outside the requested prefix.
	fullPrefix := s.key(prefix)
	pattern := escapeGlob(fullPrefix) + "*"

	var keys []string
	var cursor uint64

	for {
		var scanKeys []string
		var err error

		scanKeys, cursor, err = s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan: %w", err)
		}

		// Keep only keys under the requested prefix and strip KeyPrefix.
		for _, k := range scanKeys {
			if strings.HasPrefix(k, fullPrefix) && len(k) > len(s.keyPrefix) {
				keys = append(keys, k[len(s.keyPrefix):])
			}
		}

		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// escapeGlob escapes the Redis glob metacharacters \, *, ?, [ and ] so
// that s matches only itself in a SCAN MATCH or KEYS pattern.
func escapeGlob(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\', '*', '?', '[', ']':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Close releases storage resources.
func (s *Store) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.client.Close()
}

// Client returns the underlying Redis client for advanced operations.
func (s *Store) Client() *redis.Client {
	return s.client
}
