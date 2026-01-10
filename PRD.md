# Product Requirements Document (PRD)

## Omnistorage

**Version:** 0.1.0
**Status:** Draft
**Last Updated:** 2026-01-08

---

## 1. Overview

### 1.1 Problem Statement

Modern applications need to read and write data to various storage backends (local files, cloud object storage, cloud drives, etc.). Currently, developers must:

- Learn different APIs for each storage provider
- Write provider-specific code that's tightly coupled
- Duplicate logic for common operations (compression, formatting)
- Struggle to switch between providers or support multiple providers

### 1.2 Solution

Omnistorage is a unified storage abstraction layer for Go that provides:

- **Single interface** for 100+ storage backends
- **Composable layers** for compression and formatting
- **Backend registration** allowing internal and external implementations
- **Record-oriented streaming** for log/event/record workloads

### 1.3 Target Users

- **Application developers** who need to write to various storage backends
- **Library authors** building tools that need pluggable storage
- **DevOps/Platform teams** standardizing on storage abstractions
- **Data pipeline projects** that generate streaming record data

---

## 2. Goals and Non-Goals

### 2.1 Goals

1. **Unified Interface**: Single API for all storage backends
2. **Composability**: Layer compression, formatting, and transport independently
3. **Extensibility**: External packages can implement backends
4. **Simplicity**: Easy to use for common cases
5. **Performance**: Minimal overhead, streaming support
6. **Compatibility**: Support all major storage providers (inspired by rclone)

### 2.2 Non-Goals

1. **Full filesystem semantics**: Not a FUSE/VFS replacement (but may support mounting in future)
2. **Database replacement**: Not for structured queries
3. **Real-time streaming**: Not a Kafka/NATS replacement (though can write to them)

---

## 3. User Stories

### 3.1 Application Developer

> As an application developer, I want to write log records to S3 without learning the S3 SDK, so that I can focus on my application logic.

```go
backend := s3.New(config)
w, _ := backend.NewWriter("logs/2024-01-08.ndjson.gz")
defer w.Close()

gzw := gzip.NewWriter(w)
ndjw := ndjson.NewWriter(gzw)

for _, record := range records {
    data, _ := json.Marshal(record)
    ndjw.Write(data)
}
```

### 3.2 Library Author

> As a library author, I want to accept a storage interface so users can choose their own backend.

```go
type MyLibrary struct {
    storage omnistorage.Backend
}

func (l *MyLibrary) Save(path string, data []byte) error {
    w, err := l.storage.NewWriter(path)
    if err != nil {
        return err
    }
    defer w.Close()
    _, err = w.Write(data)
    return err
}
```

### 3.3 Multi-Cloud Support

> As a platform engineer, I want to switch between AWS S3 and Cloudflare R2 without code changes.

```go
// Configuration-driven backend selection
backend, _ := omnistorage.Open("s3", map[string]string{
    "bucket":   os.Getenv("STORAGE_BUCKET"),
    "region":   os.Getenv("STORAGE_REGION"),
    "endpoint": os.Getenv("STORAGE_ENDPOINT"), // Empty for AWS, R2 URL for Cloudflare
})
```

### 3.4 External Backend Implementation

> As a storage provider, I want to create an omnistorage backend for my service.

```go
// github.com/mycompany/omnistorage-mycloud
package mycloud

import "github.com/grokify/omnistorage"

func init() {
    omnistorage.Register("mycloud", New)
}

type Backend struct { /* ... */ }

func (b *Backend) NewWriter(path string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
    // Implementation
}
```

### 3.5 File Synchronization (rclone-like)

> As a DevOps engineer, I want to sync files between local storage and S3 with progress tracking and resume support.

```go
// One-way sync from local to S3
srcBackend := file.New(file.Config{Root: "/data"})
dstBackend := s3.New(s3Config)

syncer := sync.New(srcBackend, dstBackend, sync.Options{
    Delete:    true,  // Delete files in dst not in src
    DryRun:    false,
    Bandwidth: 10 * 1024 * 1024, // 10 MB/s limit
})

stats, err := syncer.Sync(ctx)
fmt.Printf("Transferred: %d files, %s\n", stats.Transferred, stats.Bytes)
```

### 3.6 Object Metadata Access

> As an application developer, I want to check file sizes and modification times without downloading files.

```go
backend := s3.New(config)

// Get object metadata
info, err := backend.Stat(ctx, "logs/data.json")
if err != nil {
    return err
}

fmt.Printf("Size: %d bytes\n", info.Size())
fmt.Printf("Modified: %s\n", info.ModTime())
fmt.Printf("Hash: %s\n", info.Hash(omnistorage.HashMD5))
```

### 3.7 Server-Side Operations

> As a developer, I want to copy/move files within S3 without downloading them.

```go
backend := s3.New(config)

// Check if server-side copy is supported
if backend.Features().Copy {
    // Efficient server-side copy
    err := backend.Copy(ctx, "source/file.txt", "dest/file.txt")
} else {
    // Fallback to read+write
    omnistorage.CopyPath(ctx, backend, "source/file.txt", backend, "dest/file.txt")
}
```

### 3.8 Filtering and Selection

> As a backup administrator, I want to sync only specific file types and sizes.

```go
syncer := sync.New(src, dst, sync.Options{
    Filter: filter.New(
        filter.Include("*.json"),
        filter.Include("*.log"),
        filter.Exclude("*.tmp"),
        filter.MaxSize(100 * 1024 * 1024), // 100 MB
        filter.MinAge(24 * time.Hour),     // Older than 1 day
    ),
})
```

---

## 4. Features

### 4.1 Core Features (MVP)

| Feature | Description | Priority |
|---------|-------------|----------|
| Backend Interface | `NewWriter`, `NewReader`, `Exists`, `Delete`, `List`, `Close` | P0 |
| RecordWriter Interface | `Write([]byte)`, `Flush`, `Close` | P0 |
| RecordReader Interface | `Read() ([]byte, error)`, `Close` | P0 |
| File Backend | Local filesystem | P0 |
| Memory Backend | In-memory buffer | P0 |
| NDJSON Format | Newline-delimited JSON framing | P0 |
| Gzip Compression | Gzip compress/decompress wrapper | P0 |
| Backend Registration | `Register()`, `Open()` pattern | P0 |

### 4.2 Phase 2 Features

| Feature | Description | Priority |
|---------|-------------|----------|
| S3 Backend | AWS S3, R2, MinIO, Wasabi, etc. | P1 |
| Channel Backend | Go channel for in-process streaming | P1 |
| Multi Writer | Fan-out to multiple backends | P1 |
| Zstd Compression | Zstandard compression | P1 |
| Length-Prefix Format | Binary length-prefixed framing | P1 |

### 4.3 Phase 3 Features

| Feature | Description | Priority |
|---------|-------------|----------|
| GCS Backend | Google Cloud Storage | P2 |
| Azure Backend | Azure Blob Storage | P2 |
| SFTP Backend | SSH File Transfer Protocol | P2 |
| FTP Backend | File Transfer Protocol | P2 |
| WebDAV Backend | WebDAV protocol | P2 |

### 4.4 Extended Backend Interface (rclone-like)

| Feature | Description | Priority |
|---------|-------------|----------|
| ObjectInfo Interface | Size, ModTime, Hash, MimeType | P1 |
| Stat Operation | Get object metadata without download | P1 |
| Mkdir/Rmdir | Explicit directory operations | P1 |
| Copy Operation | Server-side copy when supported | P1 |
| Move Operation | Server-side move when supported | P1 |
| Features Discovery | Query backend capabilities | P1 |
| Purge Operation | Recursive delete | P2 |
| SetModTime | Set modification time | P2 |
| About | Get storage quota/usage info | P2 |

### 4.5 Sync Engine (rclone-like)

| Feature | Description | Priority |
|---------|-------------|----------|
| One-way Sync | Make destination match source | P2 |
| Bidirectional Sync | Two-way synchronization | P3 |
| Check/Verify | Verify files match between backends | P2 |
| Diff | Show differences between backends | P2 |
| Dedupe | Find and remove duplicate files | P3 |
| Progress Tracking | Real-time transfer progress | P2 |
| Resume Support | Resume interrupted transfers | P2 |

### 4.6 Filtering System

| Feature | Description | Priority |
|---------|-------------|----------|
| Include Patterns | Glob patterns for inclusion | P2 |
| Exclude Patterns | Glob patterns for exclusion | P2 |
| Size Filters | Min/max file size | P2 |
| Age Filters | Min/max modification time | P2 |
| Filter Files | Load filters from file | P3 |

### 4.7 Transfer Controls

| Feature | Description | Priority |
|---------|-------------|----------|
| Bandwidth Limiting | Limit transfer speed | P2 |
| Parallel Transfers | Concurrent file transfers | P2 |
| Chunked Uploads | Multipart/chunked uploads | P2 |
| Checksums | Verify integrity with hashes | P2 |
| Retries | Automatic retry with backoff | P2 |

### 4.8 Additional Backends

| Feature | Description | Priority |
|---------|-------------|----------|
| Dropbox Backend | Dropbox API | P3 |
| Google Drive Backend | Google Drive API | P3 |
| OneDrive Backend | Microsoft OneDrive API | P3 |
| HTTP Backend | HTTP POST/PUT for webhooks | P3 |
| Kafka Backend | Apache Kafka producer | P3 |

### 4.9 Middleware

| Feature | Description | Priority |
|---------|-------------|----------|
| Buffered Writer | Configurable buffering layer | P3 |
| Metrics Wrapper | Observability (bytes, latency, errors) | P3 |
| Encryption | Client-side encryption | P3 |
| Caching | Local cache for remote backends | P3 |

### 4.10 Security & Authentication

| Feature | Description | Priority |
|---------|-------------|----------|
| Credential Management | Secure storage via omnivault integration | P2 |
| OAuth Support | OAuth flows via goauth integration | P2 |
| IAM Role Support | AWS/GCP instance profiles | P2 |
| Signed URLs | Generate temporary access URLs | P2 |
| Credential Rotation | Support for rotating credentials | P3 |

### 4.11 Observability

| Feature | Description | Priority |
|---------|-------------|----------|
| Structured Logging | slog integration | P2 |
| Metrics Export | Prometheus/OpenTelemetry metrics | P2 |
| Distributed Tracing | OpenTelemetry tracing support | P3 |
| Audit Logging | Compliance audit trails | P3 |

### 4.12 Data Integrity & Versioning

| Feature | Description | Priority |
|---------|-------------|----------|
| Object Versioning | Support backend versioning features | P2 |
| Lifecycle Policies | Expiration, storage class transitions | P3 |
| Soft Delete | Trash/recycle bin support | P3 |
| Point-in-Time Recovery | Restore to previous state | P3 |

### 4.13 Large File Handling

| Feature | Description | Priority |
|---------|-------------|----------|
| Streaming Mode | Handle files larger than memory | P1 |
| Multipart Thresholds | Configurable chunking thresholds | P2 |
| Resume Support | Resume interrupted transfers | P2 |
| Memory Management | Configurable buffer pools | P3 |

### 4.14 Configuration Management

| Feature | Description | Priority |
|---------|-------------|----------|
| Config Files | YAML/TOML configuration support | P2 |
| Environment Variables | Override config via env vars | P2 |
| Encrypted Credentials | Secure credential storage | P2 |
| Profile Management | Multiple named configurations | P2 |

### 4.15 Testing Infrastructure

| Feature | Description | Priority |
|---------|-------------|----------|
| Mock Backend | In-memory mock for unit testing | P1 |
| Conformance Suite | Test suite for backend authors | P2 |
| Benchmark Suite | Performance benchmarks | P2 |
| Integration Framework | Framework for integration tests | P2 |

### 4.16 Events & Notifications

| Feature | Description | Priority |
|---------|-------------|----------|
| Change Notifications | Notify on file changes | P3 |
| Webhooks | HTTP callbacks on events | P3 |
| Watch API | Watch directory for changes | P3 |

---

## 5. Success Metrics

| Metric | Target |
|--------|--------|
| Backend Coverage | Support 20+ backends within 6 months |
| API Stability | No breaking changes after v1.0 |
| Adoption | Used by 3+ public projects |
| Performance | <5% overhead vs direct SDK usage |
| Test Coverage | >80% code coverage |

---

## 6. Dependencies

### 6.1 Required

- Go 1.21+ (for generics, slog)
- Standard library (`io`, `compress/gzip`, `os`)

### 6.2 Optional (per backend)

- `github.com/aws/aws-sdk-go-v2` - S3 backend
- `cloud.google.com/go/storage` - GCS backend
- `github.com/Azure/azure-sdk-for-go` - Azure backend
- `github.com/pkg/sftp` - SFTP backend
- `github.com/klauspost/compress/zstd` - Zstd compression

### 6.3 Security & Authentication

- `github.com/agentplexus/omnivault` - Secure credential management
- `github.com/grokify/goauth` - OAuth flow support

### 6.4 Observability

- `log/slog` - Structured logging (standard library)
- `go.opentelemetry.io/otel` - OpenTelemetry metrics and tracing
- `github.com/prometheus/client_golang` - Prometheus metrics

---

## 7. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| API churn before v1.0 | Breaking changes hurt adoption | Careful design, community feedback |
| Backend maintenance burden | 100+ backends = lots of code | Community contributions, external packages |
| Performance overhead | Abstraction slows things down | Benchmarks, optimization, optional bypass |
| SDK version conflicts | Different projects use different SDK versions | Minimal dependencies, separate backend modules |

---

## 8. Open Questions

1. Should backends be separate Go modules to isolate dependencies?
2. Should we support async/batch writes at the interface level?
3. How do we handle backend-specific features (S3 multipart, etc.)?
4. Should we provide a CLI tool for testing backends?
5. **Interface Design**: Should ExtendedBackend embed Backend or use separate interfaces with type assertions?
6. **Iterator Pattern**: Should List() return an iterator instead of `[]string` for large directories?
7. **Sync Scope**: How much of rclone's sync functionality should be in core vs separate package?
8. **CLI Tool**: Should we provide an `omnistorage` CLI similar to rclone for testing and operations?

---

## 9. References

### Inspiration

- [rclone](https://github.com/rclone/rclone) - Inspiration for backend coverage and sync capabilities
- [go-cloud](https://github.com/google/go-cloud) - Google's portable cloud APIs
- [afero](https://github.com/spf13/afero) - Filesystem abstraction
- [stow](https://github.com/graymeta/stow) - Cloud storage abstraction

### Integrated Libraries

- [omnivault](https://github.com/agentplexus/omnivault) - Secure credential management
- [goauth](https://github.com/grokify/goauth) - OAuth flow support for consumer cloud backends
