# Key-Value Storage (KVS)

The KVS package provides a simple, unified interface for key-value storage with support for TTL and listing operations.

## Interface

```go
// Store is the basic key-value interface.
type Store interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Close() error
}

// ListableStore extends Store with key listing.
type ListableStore interface {
    Store
    List(ctx context.Context, prefix string) ([]string, error)
}
```

## Backends

### Redis

```go
import kvsredis "github.com/plexusone/omnistorage-core/kvs/backend/redis"

store, err := kvsredis.New(kvsredis.Config{
    URL:       "redis://localhost:6379",
    KeyPrefix: "myapp:",
})
if err != nil {
    return err
}
defer store.Close()

// Set with 1 hour TTL
err = store.Set(ctx, "user:123", []byte(`{"name":"john"}`), time.Hour)

// Get
data, err := store.Get(ctx, "user:123")

// List keys with prefix
keys, err := store.List(ctx, "user:")
```

**Configuration:**

```go
type Config struct {
    // URL is the Redis connection URL.
    // Format: redis://[user:password@]host:port[/db]
    URL string

    // KeyPrefix is prepended to all keys.
    KeyPrefix string

    // PoolSize is the maximum number of connections.
    // Default: 10 * GOMAXPROCS
    PoolSize int

    // ConnectTimeout is the connection timeout.
    // Default: 5 seconds
    ConnectTimeout time.Duration
}
```

## Use Cases

| Use Case | Example |
|----------|---------|
| Session storage | `session/backend/kvs` uses KVS for sessions |
| Caching | Cache API responses with TTL |
| Rate limiting | Store request counts with sliding window |
| Feature flags | Store configuration that updates without restart |
| OAuth state | Store OAuth state parameters during flow |

## TTL Handling

- Pass `0` for no expiration
- Redis uses native `EXPIRE` commands
- Memory backend tracks expiration manually

```go
// No expiration
store.Set(ctx, "permanent", data, 0)

// 5 minute TTL
store.Set(ctx, "temp", data, 5*time.Minute)
```

## Error Handling

```go
import "github.com/plexusone/omnistorage-core/kvs"

data, err := store.Get(ctx, "key")
if err == kvs.ErrNotFound {
    // Key doesn't exist
}
```
