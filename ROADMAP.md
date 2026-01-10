# Roadmap

This document tracks the development roadmap for omnistorage.

## Design Philosophy

Omnistorage uses **interface composition** to support both simple and advanced use cases:

- **Backend** - Basic interface for read/write operations (data pipelines, simple apps)
- **ExtendedBackend** - Adds metadata, copy/move, directory ops (rclone-like tools)

Applications can use just the `Backend` interface for simple operations, while advanced tools can use `ExtendedBackend` for full functionality. The basic interface remains stable and backward-compatible.

```go
// Simple apps use Backend
func SaveData(backend omnistorage.Backend, path string, data []byte) error {
    w, _ := backend.NewWriter(ctx, path)
    defer w.Close()
    _, err := w.Write(data)
    return err
}

// Advanced tools can check for extended features
func CopyFile(backend omnistorage.Backend, src, dst string) error {
    if ext, ok := backend.(omnistorage.ExtendedBackend); ok && ext.Features().Copy {
        return ext.Copy(ctx, src, dst) // Server-side copy
    }
    return omnistorage.CopyPath(ctx, backend, src, backend, dst) // Fallback
}
```

---

## Phase 1: Foundation (MVP) ✅

Core interfaces and essential implementations.

### Core Package

- [x] `interfaces.go` - Backend, RecordWriter, RecordReader interfaces
- [x] `options.go` - WriterOption, ReaderOption, configs
- [x] `registry.go` - Register(), Open(), Backends()
- [x] `errors.go` - Common error types (ErrNotFound, etc.)

### Format Layer

- [x] `format/ndjson/writer.go` - NDJSON RecordWriter
- [x] `format/ndjson/reader.go` - NDJSON RecordReader
- [x] `format/ndjson/ndjson_test.go` - Tests

### Compression Layer

- [x] `compress/gzip/writer.go` - Gzip io.WriteCloser wrapper
- [x] `compress/gzip/reader.go` - Gzip io.ReadCloser wrapper
- [x] `compress/gzip/gzip_test.go` - Tests

### Backend Layer

- [x] `backend/file/backend.go` - Local filesystem backend
- [x] `backend/file/file_test.go` - Tests
- [x] `backend/memory/backend.go` - In-memory backend
- [x] `backend/memory/memory_test.go` - Tests

### Testing

- [x] Conformance test suite for backends
- [x] Integration tests

### Documentation

- [x] README.md with usage examples
- [x] GoDoc comments on all public APIs

---

## Phase 2: Extended Interfaces (rclone-like) ✅

Extended backend interface for metadata, directory operations, and server-side operations.

### Core Interfaces

- [x] `object_info.go` - ObjectInfo interface (Size, ModTime, Hash, MimeType)
- [x] `hash.go` - HashType enum and hash utilities
- [x] `features.go` - Features struct for capability discovery
- [x] `extended.go` - ExtendedBackend interface embedding Backend

### Extended Backend Interface

```go
type ExtendedBackend interface {
    Backend
    Stat(ctx context.Context, path string) (ObjectInfo, error)
    Mkdir(ctx context.Context, path string) error
    Rmdir(ctx context.Context, path string) error
    Copy(ctx context.Context, src, dst string) error
    Move(ctx context.Context, src, dst string) error
    Features() Features
}
```

### Update File Backend

- [x] `backend/file/extended.go` - ExtendedBackend implementation for file
- [x] `backend/file/extended_test.go` - Tests

### Utilities

- [x] `copy.go` - CopyPath helper (fallback for backends without Copy)
- [x] `move.go` - MovePath helper (fallback for backends without Move)

---

## Phase 3: Cloud Storage ✅

S3-compatible and channel backends with extended interface support.

### Backends

- [x] `backend/s3/backend.go` - S3-compatible backend (AWS, R2, MinIO, Wasabi, etc.)
- [x] `backend/s3/extended.go` - ExtendedBackend implementation (integrated in backend.go)
- [x] `backend/s3/options.go` - S3-specific configuration
- [x] `backend/s3/multipart.go` - Multipart upload support (via AWS SDK manager)
- [x] `backend/s3/s3_test.go` - Tests (with LocalStack/MinIO)
- [x] `backend/channel/backend.go` - Go channel backend (inter-goroutine communication)
- [x] `backend/channel/backend_test.go` - Tests (15 tests)

### Utilities

- [x] `multi/writer.go` - Fan-out to multiple backends (WriteAll, WriteBestEffort, WriteQuorum modes)
- [x] `multi/writer_test.go` - Tests (14 tests)

### Compression

- [x] `compress/zstd/writer.go` - Zstandard compression
- [x] `compress/zstd/reader.go`
- [x] `compress/zstd/zstd_test.go`

### Format

- [ ] `format/lengthprefix/writer.go` - Length-prefixed binary framing
- [ ] `format/lengthprefix/reader.go`
- [ ] `format/lengthprefix/lengthprefix_test.go`

---

## Phase 4: Cloud Providers (Partial) ✅

Major cloud provider backends.

### Backends

- [ ] `backend/gcs/backend.go` - Google Cloud Storage
- [ ] `backend/gcs/extended.go` - ExtendedBackend implementation
- [ ] `backend/azure/backend.go` - Azure Blob Storage
- [ ] `backend/azure/extended.go` - ExtendedBackend implementation
- [x] `backend/sftp/backend.go` - SFTP with ExtendedBackend support
- [x] `backend/sftp/options.go` - SFTP configuration (password, key file, known_hosts)
- [ ] `backend/ftp/backend.go` - FTP
- [ ] `backend/webdav/backend.go` - WebDAV

### Testing

- [ ] Integration tests with real cloud services (CI secrets)
- [ ] Performance benchmarks

---

## Phase 5: Sync Engine (rclone-inspired) ✅

File synchronization and comparison tools. Inspired by rclone's most-used features.

### Design Philosophy

The sync engine provides rclone-like functionality as a Go library:

- **Sync** - Make destination match source (like `rclone sync`)
- **Copy** - Copy files without deleting extras (like `rclone copy`)
- **Move** - Move files from source to destination (like `rclone move`)
- **Check** - Verify files match between backends (like `rclone check`)
- **Verify** - Verify file integrity (like `rclone cryptcheck`)

### Core Sync Package ✅

- [x] `sync/sync.go` - Sync, CopyDir, Move, MoveFile implementations
- [x] `sync/copy.go` - Copy function for single files/directories
- [x] `sync/check.go` - Check and Diff functions
- [x] `sync/verify.go` - Verify and VerifyFile functions
- [x] `sync/options.go` - Sync options, results, and types
- [x] `sync/ratelimit.go` - Token bucket rate limiting
- [x] `sync/retry.go` - Retry with exponential backoff
- [x] `sync/sync_test.go` - Tests (80+ tests)

```go
// Core sync API
type Options struct {
    DeleteExtra      bool              // Delete files in dst not in src
    DryRun           bool              // Report changes without making them
    Checksum         bool              // Compare by checksum vs modtime/size
    Progress         func(Progress)    // Progress callback
    Concurrency      int               // Parallel transfers (default: 4)
    BandwidthLimit   int64             // Rate limit in bytes/second
    Retry            *RetryConfig      // Retry configuration
    Filter           *filter.Filter    // Include/exclude filters
    PreserveMetadata *MetadataOptions  // Metadata preservation options
}

func Sync(ctx, src, dst, srcPath, dstPath, opts) (*Result, error)
func Copy(ctx, src, dst, srcPath, dstPath, opts) (*Result, error)
func Move(ctx, src, dst, srcPath, dstPath, opts) (*Result, error)
func Check(ctx, src, dst, srcPath, dstPath, opts) (*CheckResult, error)
func Verify(ctx, src, dst, srcPath, dstPath, opts) (*VerifyResult, error)
```

### Comparison Tools ✅

- [x] `sync/check.go` - Check function (compare src/dst)
- [x] `sync/check.go` - Diff function (show differences)
- [ ] `sync/dedupe.go` - Find duplicate files (future)

### Transfer Controls ✅

- [x] Parallel transfers - `Options{Concurrency: N}`
- [x] Bandwidth limiting - `Options{BandwidthLimit: N}` (token bucket)
- [x] Retry with backoff - `Options{Retry: &RetryConfig{}}`
- [x] Progress callbacks - `Options{Progress: func(Progress)}`

### Filtering ✅

- [x] `sync/filter/filter.go` - Filter implementation with all features
- [x] Include/Exclude patterns - `filter.Include("*.json")`, `filter.Exclude("*.tmp")`
- [x] Size filters - `filter.MinSize(100)`, `filter.MaxSize(1*MB)`
- [x] Age filters - `filter.MinAge(24*time.Hour)`, `filter.MaxAge(7*24*time.Hour)`
- [x] Filter from file - `filter.FromFile("filters.txt")`
- [x] `sync/filter/filter_test.go` - Tests (14 tests)

---

## Phase 6: Consumer Cloud & Protocols

Consumer cloud storage and messaging.

> **Note:** Cloud provider backends with large SDK dependencies are in separate repos
> to keep the core omnistorage package lightweight:
>
> - **Google backends** → [github.com/grokify/omnistorage-google](https://github.com/grokify/omnistorage-google)
>   - Google Drive ✅
>   - Google Cloud Storage (planned)

### Cloud Drive Backends

- [ ] `backend/dropbox/backend.go` - Dropbox
- [ ] `backend/dropbox/extended.go` - ExtendedBackend implementation
- [x] Google Drive - See `github.com/grokify/omnistorage-google/backend/drive`
- [ ] Google Cloud Storage - See `github.com/grokify/omnistorage-google/backend/gcs`
- [ ] `backend/onedrive/backend.go` - Microsoft OneDrive
- [ ] `backend/onedrive/extended.go` - ExtendedBackend implementation
- [ ] `backend/box/backend.go` - Box
- [ ] `backend/icloud/backend.go` - iCloud Drive

### Protocol Backends

- [ ] `backend/http/backend.go` - HTTP POST/PUT (webhooks)
- [ ] `backend/smb/backend.go` - SMB/CIFS

### Messaging Backends

- [ ] `backend/kafka/backend.go` - Apache Kafka
- [ ] `backend/nats/backend.go` - NATS
- [ ] `backend/sqs/backend.go` - AWS SQS
- [ ] `backend/pubsub/backend.go` - Google Pub/Sub

---

## Phase 7: Security & Authentication

Security, credential management, and OAuth support.

### Credential Management

- [ ] `auth/credentials.go` - Credential interface
- [ ] `auth/omnivault/provider.go` - Integration with github.com/agentplexus/omnivault
- [ ] `auth/env/provider.go` - Environment variable credentials
- [ ] `auth/file/provider.go` - File-based credentials (encrypted)
- [ ] `auth/iam/provider.go` - AWS/GCP IAM role support

### OAuth Support

- [ ] `auth/oauth/provider.go` - OAuth credential provider
- [ ] `auth/oauth/goauth.go` - Integration with github.com/grokify/goauth
- [ ] `auth/oauth/token.go` - Token refresh handling
- [ ] `auth/oauth/oauth_test.go` - Tests

### Signed URLs

- [ ] `signedurl/signedurl.go` - Signed URL generation interface
- [ ] `signedurl/s3.go` - S3 presigned URLs
- [ ] `signedurl/gcs.go` - GCS signed URLs

---

## Phase 8: Observability

Logging, metrics, and tracing support.

### Logging

- [ ] `observe/logging/logger.go` - slog integration
- [ ] `observe/logging/context.go` - Context-aware logging
- [ ] `observe/logging/middleware.go` - Logging middleware for backends

### Metrics

- [ ] `observe/metrics/metrics.go` - Metrics interface
- [ ] `observe/metrics/prometheus.go` - Prometheus exporter
- [ ] `observe/metrics/otel.go` - OpenTelemetry metrics

### Tracing

- [ ] `observe/tracing/tracing.go` - Tracing interface
- [ ] `observe/tracing/otel.go` - OpenTelemetry tracing
- [ ] `observe/tracing/middleware.go` - Tracing middleware

### Audit

- [ ] `observe/audit/audit.go` - Audit logging interface
- [ ] `observe/audit/file.go` - File-based audit log
- [ ] `observe/audit/structured.go` - Structured audit events

---

## Phase 9: Middleware & Utilities

Advanced features and middleware.

### Middleware

- [ ] `middleware/buffer/writer.go` - Configurable buffering
- [ ] `middleware/retry/writer.go` - Retry with exponential backoff
- [ ] `middleware/metrics/writer.go` - Observability (bytes, latency, errors)
- [ ] `middleware/ratelimit/writer.go` - Rate limiting
- [ ] `middleware/encrypt/writer.go` - Client-side encryption
- [ ] `middleware/cache/cache.go` - Local cache for remote backends

### Utilities

- [ ] `multi/reader.go` - Read from multiple sources (merge/concat)
- [ ] `tee/writer.go` - Write to backend and return copy

### Format

- [ ] `format/csv/writer.go` - CSV format
- [ ] `format/csv/reader.go`
- [ ] `format/parquet/writer.go` - Parquet format (future)

---

## Phase 10: Testing Infrastructure

Testing tools and frameworks for omnistorage.

### Mock Backend

- [ ] `backend/mock/backend.go` - Configurable mock backend
- [ ] `backend/mock/expectations.go` - Set expectations for testing
- [ ] `backend/mock/assertions.go` - Assert operations occurred

### Conformance Suite

- [ ] `testing/conformance/suite.go` - Backend conformance test suite
- [ ] `testing/conformance/basic.go` - Basic operations tests
- [ ] `testing/conformance/extended.go` - ExtendedBackend tests
- [ ] `testing/conformance/concurrent.go` - Concurrency tests

### Benchmarks

- [ ] `testing/benchmark/benchmark.go` - Benchmark framework
- [ ] `testing/benchmark/throughput.go` - Throughput benchmarks
- [ ] `testing/benchmark/latency.go` - Latency benchmarks

---

## Phase 11: Configuration & Events

Configuration management and event notifications.

### Configuration

- [ ] `config/config.go` - Configuration interface
- [ ] `config/yaml.go` - YAML configuration loader
- [ ] `config/toml.go` - TOML configuration loader
- [ ] `config/env.go` - Environment variable overrides
- [ ] `config/profile.go` - Profile/remote management

### Events

- [ ] `events/events.go` - Event interface
- [ ] `events/watcher.go` - Watch for file changes
- [ ] `events/webhook.go` - Webhook notifications
- [ ] `events/channel.go` - Go channel events

### Versioning

- [ ] `versioning/versioning.go` - Object versioning interface
- [ ] `versioning/lifecycle.go` - Lifecycle policies
- [ ] `versioning/softdelete.go` - Soft delete support

---

## Phase 12: Extended Backend Coverage

Additional backends inspired by rclone.

### Object Storage

- [ ] Backblaze B2
- [ ] DigitalOcean Spaces
- [ ] Linode Object Storage
- [ ] Oracle Cloud Storage
- [ ] Alibaba Cloud OSS
- [ ] Tencent Cloud COS
- [ ] Huawei Cloud OBS

### Cloud Drives

- [ ] pCloud
- [ ] MEGA
- [ ] Yandex Disk
- [ ] Mail.ru Cloud
- [ ] Jottacloud
- [ ] Koofr
- [ ] HiDrive

### Enterprise

- [ ] Citrix ShareFile
- [ ] Enterprise File Fabric
- [ ] Nextcloud/ownCloud

### Specialized

- [ ] Internet Archive
- [ ] HDFS (Hadoop)
- [ ] Storj (decentralized)

---

## Phase 13: CLI Tool

Command-line interface similar to rclone for operations and testing.

### Core CLI

- [ ] `cmd/omnistorage/main.go` - CLI entry point
- [ ] `cmd/omnistorage/ls.go` - List files
- [ ] `cmd/omnistorage/cp.go` - Copy files
- [ ] `cmd/omnistorage/mv.go` - Move files
- [ ] `cmd/omnistorage/rm.go` - Remove files
- [ ] `cmd/omnistorage/sync.go` - Sync directories
- [ ] `cmd/omnistorage/check.go` - Verify files match
- [ ] `cmd/omnistorage/config.go` - Manage backend configurations

### CLI Features

- [ ] Progress bars for transfers
- [ ] Verbose/quiet output modes
- [ ] JSON output for scripting
- [ ] Interactive configuration wizard

---

## Phase 14: Advanced Sync Features (Partial) ✅

Advanced synchronization capabilities.

### Bidirectional Sync ✅

- [x] `sync/bisync.go` - Two-way synchronization with conflict resolution
- [x] `sync/bisync.go` - ConflictStrategy: NewerWins, LargerWins, SourceWins, DestWins, KeepBoth, Skip, Error
- [x] `sync/bisync_test.go` - Tests

### Incremental/Snapshot

- [ ] `sync/snapshot.go` - Point-in-time snapshots
- [ ] `sync/incremental.go` - Incremental backup support

---

## Future Considerations

Items for future evaluation.

### Features

- [ ] Async/batch write support at interface level
- [ ] Streaming uploads with progress callbacks
- [ ] Connection pooling configuration
- [ ] FUSE mount support (separate package)
- [ ] WebDAV/HTTP server for backends

### Tooling

- [ ] Backend health check utility
- [ ] Migration tool between backends
- [ ] Performance profiler

### Integration

- [ ] Example integrations
- [ ] OpenTelemetry tracing
- [ ] Prometheus metrics exporter

---

## Version History

| Version | Date | Milestone |
|---------|------|-----------|
| v0.1.0 | 2026-01-10 | Initial release: Phases 1-5 complete + SFTP + Bisync |
| v0.2.0 | TBD | Phase 6 partial (Consumer cloud backends) |
| v0.3.0 | TBD | Phase 7 (Security & authentication) |
| v0.4.0 | TBD | Phase 8 (Observability) |
| v1.0.0 | TBD | Stable API, production ready |
| v1.1.0 | TBD | CLI tool (Phase 13) |

### v0.1.0 Includes

- **Core**: Backend, ExtendedBackend, RecordWriter/Reader interfaces
- **Backends**: File, Memory, S3, SFTP, Channel
- **Compression**: Gzip, Zstd
- **Format**: NDJSON
- **Sync**: Sync, Copy, Move, Check, Verify, Bisync with filtering and rate limiting
- **Multi-writer**: Fan-out to multiple backends

---

## Contributing

We welcome contributions! Priority areas:

1. **New backends** - Follow `backend/file` as a template
2. **Tests** - Especially integration tests with real services
3. **Documentation** - Examples, guides, GoDoc
4. **Bug fixes** - Issues labeled `good first issue`

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
