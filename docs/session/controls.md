# Session Controls

The `ControlledStore` wrapper adds security controls and observability to any session backend.

## Size Limits

Prevent session bloat attacks by enforcing maximum session size:

```go
cfg := session.Config{
    MaxSessionSize: 1024 * 1024, // 1MB limit
}

store := sessionmemory.NewWithControls(cfg)

// This will fail if session serializes to > 1MB
err := store.Create(ctx, largeSession)
if session.ErrorCode(err) == session.ErrCodeSizeLimitExceeded {
    // Handle size limit exceeded
}
```

**Why size limits matter:**

- Prevents memory exhaustion in session store
- Blocks abuse where attackers stuff data into sessions
- Catches bugs where large objects accidentally stored in sessions
- Redis/memory backends have finite capacity

## Multi-Site Isolation (SiteID)

When hosting multiple sites in a single service (e.g., academyos, agentos, dashforge), use `SiteID` to isolate sessions:

```go
// Configure store for academyos site
cfg := session.Config{
    SiteID:     "academyos",
    DefaultTTL: 24 * time.Hour,
}

store := sessionmemory.NewWithControls(cfg)

// Sessions are automatically tagged with SiteID
sess, _ := session.NewSession(userID, 24*time.Hour)
store.Create(ctx, sess)  // sess.SiteID = "academyos"

// Get operations validate SiteID
// A session from "agentos" returns ErrNotFound (security: no leakage)
```

**How SiteID isolation works:**

| Operation | Behavior |
|-----------|----------|
| `Create` | Automatically sets `session.SiteID` from config |
| `Get` | Returns `ErrNotFound` if session belongs to different site |
| `ListByUserID` | Only returns sessions for the configured site |
| `MaxSessionsPerUser` | Counts only sessions for this site |

**Why SiteID matters:**

- **Security** - Sessions from one site cannot be used on another
- **Analytics** - Query sessions by site for monitoring
- **Isolation** - Per-site session limits and policies
- **Shared backend** - Multiple sites can safely share Redis/memory

## Max Sessions Per User

Limit concurrent sessions per user to prevent session accumulation:

```go
cfg := session.Config{
    MaxSessionsPerUser: 5,  // Allow max 5 concurrent sessions
    SiteID:             "academyos",  // Limit is per-site
    DefaultTTL:         24 * time.Hour,
}

store := sessionmemory.NewWithControls(cfg)

// When user exceeds limit, oldest sessions are evicted
for i := 0; i < 6; i++ {
    sess, _ := session.NewSession(userID, 24*time.Hour)
    store.Create(ctx, sess)  // 6th session evicts 1st session
}
```

**Behavior:**

- Oldest sessions (by `CreatedAt`) are automatically evicted
- Eviction triggers `ViolationSessionLimitExceeded` callback
- When `SiteID` is set, limit applies per-site (user can have 5 sessions per site)
- Requires backend to implement `SessionLister` interface (memory and KVS do)

## JSON Serialization Enforcement

The controlled store validates that all session data can be serialized to JSON. This blocks Go-specific serialization formats that cause issues:

```go
sess.Data["func"] = func() {} // Will fail validation
sess.Data["channel"] = make(chan int) // Will fail validation
sess.Data["valid"] = map[string]any{"key": "value"} // OK
```

**Why JSON-only:**

- **Interoperability** - Other languages/tools can read session data
- **Debugging** - Session data is human-readable in Redis
- **Security** - Prevents deserialization attacks (unlike Java's native serialization)
- **Stability** - No version compatibility issues when struct definitions change

## Violation Callbacks

Hook into policy violations for metrics, alerting, and audit logging:

```go
store := sessionmemory.NewWithControls(cfg,
    session.WithViolationHandler(func(ctx context.Context, event session.ViolationEvent) {
        // Emit OpenTelemetry metrics
        meter.Int64Counter("session.violations").Add(ctx, 1,
            attribute.String("type", string(event.Type)),
        )

        // Alert on repeated abuse
        if event.Type == session.ViolationSizeExceeded {
            alerting.Warn("Session size limit exceeded", map[string]any{
                "user_id":    event.UserID,
                "session_id": event.SessionID,
                "size":       event.Size,
                "limit":      event.Limit,
            })
        }
    }),
)
```

**Violation Types:**

| Type | Description |
|------|-------------|
| `ViolationSizeExceeded` | Session data exceeds `MaxSessionSize` |
| `ViolationNotSerializable` | Session data cannot be marshaled to JSON |
| `ViolationInvalidSession` | Session missing required fields (ID, UserID) |
| `ViolationSessionLimitExceeded` | User exceeded `MaxSessionsPerUser` (oldest evicted) |
| `ViolationSiteMismatch` | Session belongs to different site (Get returns ErrNotFound) |

## Structured Logging

Built-in slog integration logs all violations:

```go
store := sessionmemory.NewWithControls(cfg,
    session.WithLogger(slog.Default()),
)
```

Log output:

```json
{
  "level": "WARN",
  "msg": "session policy violation",
  "type": "size_exceeded",
  "session_id": "abc123",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "size": 1048577,
  "limit": 1048576,
  "message": "size 1048577 exceeds limit 1048576"
}
```

## Structured Errors

Errors include machine-readable codes for HTTP status mapping:

```go
err := store.Create(ctx, session)
if err != nil {
    code := session.ErrorCode(err)
    switch code {
    case session.ErrCodeSizeLimitExceeded:
        http.Error(w, "Session data too large", http.StatusRequestEntityTooLarge)
    case session.ErrCodeNotSerializable:
        http.Error(w, "Invalid session data", http.StatusBadRequest)
    case session.ErrCodeInvalidSession:
        http.Error(w, "Invalid session", http.StatusBadRequest)
    default:
        http.Error(w, "Session error", http.StatusInternalServerError)
    }
}
```

**Error Codes:**

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `SESSION_SIZE_EXCEEDED` | 413 | Payload too large |
| `SESSION_NOT_SERIALIZABLE` | 400 | Invalid data format |
| `SESSION_INVALID` | 400 | Missing required fields |
| `SESSION_NOT_FOUND` | 404 | Session doesn't exist or wrong site |
| `SESSION_EXPIRED` | 401 | Session TTL elapsed |
| `SESSION_LIMIT_EXCEEDED` | 429 | Too many concurrent sessions |
| `SESSION_SITE_MISMATCH` | 404 | Session belongs to different site (treated as not found) |

## Configuration Reference

```go
type Config struct {
    // SiteID identifies which site/service this store handles.
    // Used for multi-site isolation (e.g., "academyos", "agentos").
    // When set, all sessions are tagged and validated against this site.
    SiteID string

    // MaxSessionSize is the maximum serialized session size in bytes.
    // Set to 0 for no limit (not recommended for production).
    MaxSessionSize int

    // MaxSessionsPerUser limits concurrent sessions per user.
    // When exceeded, oldest sessions are automatically evicted.
    // Set to 0 for no limit.
    MaxSessionsPerUser int

    // DefaultTTL is applied to sessions without explicit expiration.
    DefaultTTL time.Duration

    // CleanupInterval controls automatic expired session cleanup.
    // Set to 0 to disable (Redis handles this via TTL).
    CleanupInterval time.Duration

    // KeyPrefix for KVS backends.
    KeyPrefix string
}

// Sensible defaults
cfg := session.DefaultConfig()
// MaxSessionSize:  1MB
// DefaultTTL:      24 hours
// CleanupInterval: 5 minutes
// KeyPrefix:       "session:"
```

## Session Struct

```go
type Session struct {
    ID             string     // Unique session identifier
    UserID         uuid.UUID  // Authenticated user's ID
    SiteID         string     // Site identifier (set by ControlledStore)
    OrganizationID *uuid.UUID // Optional tenant context within site
    Data           map[string]any  // Arbitrary session data (JSON-safe)
    CreatedAt      time.Time
    UpdatedAt      time.Time
    LastAccessedAt time.Time
    ExpiresAt      time.Time
    IPAddress      string     // Client IP (optional)
    UserAgent      string     // Client UA (optional)
}
```
