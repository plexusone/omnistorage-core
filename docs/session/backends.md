# Session Backends

## Memory Backend

The memory backend stores sessions in-process. Ideal for development and single-server deployments.

```go
import (
    "github.com/plexusone/omnistorage-core/session"
    sessionmemory "github.com/plexusone/omnistorage-core/session/backend/memory"
)

cfg := session.Config{
    MaxSessionSize:  1024 * 1024,
    DefaultTTL:      24 * time.Hour,
    CleanupInterval: 5 * time.Minute,
}

// Without controls (raw store)
store := sessionmemory.New(cfg)

// With controls (recommended)
store := sessionmemory.NewWithControls(cfg)
```

**Characteristics:**

- Zero external dependencies
- Automatic cleanup of expired sessions
- User index for efficient `DeleteByUserID`
- Not persistent across restarts
- Not distributed (single-server only)

## KVS Backend

The KVS backend adapts any `kvs.ListableStore` implementation for session storage. This enables Redis, SQLite, or custom backends.

### With Redis

```go
import (
    "github.com/plexusone/omnistorage-core/session"
    sessionkvs "github.com/plexusone/omnistorage-core/session/backend/kvs"
    kvsredis "github.com/plexusone/omnistorage-core/kvs/backend/redis"
)

// Create Redis KVS backend
redisStore, err := kvsredis.New(kvsredis.Config{
    URL:       "redis://localhost:6379",
    KeyPrefix: "myapp:session:",
})
if err != nil {
    return err
}

// Create session store with controls
cfg := session.Config{
    MaxSessionSize:  1024 * 1024,
    DefaultTTL:      24 * time.Hour,
    CleanupInterval: 5 * time.Minute,
    KeyPrefix:       "session:",
}

store := sessionkvs.NewWithControls(redisStore, cfg)
```

**Redis Characteristics:**

- Distributed across multiple servers
- Automatic TTL expiration via Redis EXPIRE
- Persistent across restarts
- User index cleanup runs periodically

### Key Structure

The KVS backend uses the following key structure:

```
{prefix}{session_id}           → Session JSON
{prefix}user:{user_id}         → JSON array of session IDs
```

Example with `session:` prefix:

```
session:abc123def456           → {"id":"abc123def456","user_id":"..."}
session:user:550e8400-e29b...  → ["abc123def456","xyz789ghi012"]
```

## Custom Backend

Implement the `session.Store` interface to create a custom backend:

```go
type Store interface {
    Create(ctx context.Context, session *Session) error
    Get(ctx context.Context, id string) (*Session, error)
    Update(ctx context.Context, session *Session) error
    Delete(ctx context.Context, id string) error
    DeleteByUserID(ctx context.Context, userID string) (int, error)
    Touch(ctx context.Context, id string) error
    Close() error
}
```

Wrap your implementation with controls:

```go
store := session.WithControls(myCustomStore, cfg,
    session.WithLogger(logger),
    session.WithViolationHandler(handler),
)
```

## Backend Comparison

| Feature | Memory | KVS+Redis | KVS+SQLite |
|---------|--------|-----------|------------|
| Distributed | No | Yes | No |
| Persistent | No | Yes | Yes |
| TTL Support | Manual | Native | Manual |
| Dependencies | None | Redis | SQLite |
| Best For | Dev/Test | Production | Edge/Embedded |
