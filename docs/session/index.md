# Session Storage

The session package provides a secure, backend-agnostic session storage layer with built-in controls for size limits, validation, and observability.

## Features

- **Backend-agnostic** - Use in-memory for development, Redis/KVS for production
- **Multi-site isolation** - Secure session separation across sites (academyos, agentos, etc.)
- **Size limits** - Prevent session bloat with configurable byte limits
- **Per-user limits** - Limit concurrent sessions per user with automatic eviction
- **JSON-only serialization** - Blocks Go-specific formats (gob) that cause interoperability issues
- **Violation callbacks** - Hook into metrics/alerting when policies are violated
- **Structured logging** - Built-in slog integration
- **User-based operations** - Delete all sessions for a user, per-user session indexing

## Quick Start

```go
import (
    "github.com/plexusone/omnistorage-core/session"
    sessionmemory "github.com/plexusone/omnistorage-core/session/backend/memory"
)

// Create an in-memory store with controls
store := sessionmemory.NewWithControls(session.Config{
    SiteID:             "academyos",  // Multi-site isolation
    MaxSessionSize:     1024 * 1024,  // 1MB limit
    MaxSessionsPerUser: 5,            // Max 5 concurrent sessions
    DefaultTTL:         24 * time.Hour,
    CleanupInterval:    5 * time.Minute,
})
defer store.Close()

// Create a session (SiteID is set automatically)
sess, _ := session.NewSession(userID, 24*time.Hour)
sess.Data["role"] = "admin"
store.Create(ctx, sess)  // sess.SiteID = "academyos"

// Retrieve session
sess, err := store.Get(ctx, sessionID)
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Your Application                         │
│  Uses: session.Store interface                              │
└─────────────────────────────────┬───────────────────────────┘
                                  │
┌─────────────────────────────────▼───────────────────────────┐
│              ControlledStore (wrapper.go)                   │
│  - SiteID isolation (multi-site security)                   │
│  - Size limit enforcement                                   │
│  - Per-user session limits                                  │
│  - JSON serialization validation                            │
│  - Violation logging and callbacks                          │
│  - Structured error codes                                   │
└─────────────────────────────────┬───────────────────────────┘
                                  │
         ┌────────────────────────┼────────────────────────┐
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ backend/memory  │    │  backend/kvs    │    │  (your custom)  │
│ In-memory store │    │  KVS adapter    │    │                 │
└─────────────────┘    └────────┬────────┘    └─────────────────┘
                                │
                       ┌────────▼────────┐
                       │ kvs.ListableStore│
                       │ (Redis, SQLite)  │
                       └─────────────────┘
```

## Backends

| Backend | Use Case | Distributed |
|---------|----------|-------------|
| `memory` | Development, testing, single-server | No |
| `kvs` + Redis | Production, multi-server | Yes |
| `kvs` + SQLite | Edge deployments, embedded | No |

## When to Use

Use the session package when you need:

- Server-side session storage with HTTP-only cookies
- BFF (Backend for Frontend) session management
- OAuth token storage with secure server-side handling
- User presence tracking across servers

## Integration with SystemForge

The session package integrates with [SystemForge](https://github.com/grokify/systemforge) via the `store_omnistorage.go` adapter:

```go
import "github.com/grokify/systemforge/session/bff"

store, err := bff.NewOmniStorageStore(bff.OmniStorageConfig{
    Backend:        "redis",
    RedisURL:       "redis://localhost:6379",
    MaxSessionSize: 1024 * 1024,
})
```

See [SystemForge Session Docs](https://github.com/grokify/systemforge/docs/session/) for full integration details.
