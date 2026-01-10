# Technical Requirements Document (TRD)

## Omnistorage

**Version:** 0.1.0
**Status:** Draft
**Last Updated:** 2026-01-08

---

## 1. Architecture Overview

### 1.1 Layered Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
│         (data pipelines, custom apps, libraries)            │
│         Marshals domain types (IR records, logs, etc.)      │
├─────────────────────────────────────────────────────────────┤
│                     Format Layer                            │
│              (NDJSON, length-prefixed, CSV)                 │
│              Handles record framing/delimiting              │
├─────────────────────────────────────────────────────────────┤
│                   Compression Layer                         │
│                  (gzip, zstd, snappy)                       │
│              Optional compression/decompression             │
├─────────────────────────────────────────────────────────────┤
│                    Backend Layer                            │
│        (File, S3, GCS, SFTP, Dropbox, Channel, etc.)        │
│              Raw byte transport/storage                     │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 Package Structure

```
github.com/grokify/omnistorage/
├── go.mod
├── go.sum
├── interfaces.go           # Core interfaces (Backend, RecordWriter, RecordReader)
├── options.go              # Common option types
├── registry.go             # Backend registration (Register, Open)
├── errors.go               # Common error types
│
├── format/                 # Record framing implementations
│   ├── ndjson/
│   │   ├── writer.go       # NDJSON RecordWriter
│   │   ├── reader.go       # NDJSON RecordReader
│   │   └── ndjson_test.go
│   ├── lengthprefix/
│   │   ├── writer.go       # Length-prefixed binary framing
│   │   ├── reader.go
│   │   └── lengthprefix_test.go
│   └── csv/
│       ├── writer.go
│       ├── reader.go
│       └── csv_test.go
│
├── compress/               # Compression wrappers
│   ├── gzip/
│   │   ├── writer.go       # Wraps io.Writer with gzip
│   │   ├── reader.go       # Wraps io.Reader with gzip
│   │   └── gzip_test.go
│   ├── zstd/
│   │   ├── writer.go
│   │   ├── reader.go
│   │   └── zstd_test.go
│   └── snappy/
│       └── ...
│
├── backend/                # Storage backend implementations
│   ├── file/
│   │   ├── backend.go      # Local filesystem Backend
│   │   ├── options.go
│   │   └── file_test.go
│   ├── memory/
│   │   ├── backend.go      # In-memory Backend
│   │   └── memory_test.go
│   ├── channel/
│   │   ├── backend.go      # Go channel Backend
│   │   └── channel_test.go
│   ├── s3/
│   │   ├── backend.go      # S3-compatible Backend
│   │   ├── options.go
│   │   └── s3_test.go
│   ├── gcs/
│   │   └── ...
│   ├── azure/
│   │   └── ...
│   ├── sftp/
│   │   └── ...
│   └── http/
│       └── ...
│
├── multi/                  # Multi-backend utilities
│   ├── writer.go           # Fan-out to multiple backends
│   └── writer_test.go
│
├── middleware/             # Optional wrappers
│   ├── buffer/             # Buffered writes
│   ├── retry/              # Retry with backoff
│   └── metrics/            # Observability
│
└── examples/
    ├── basic/
    ├── s3-upload/
    ├── multi-backend/
    └── custom-backend/     # Example external backend
```

---

## 2. Core Interfaces

### 2.1 Backend Interface

```go
package omnistorage

import "io"

// Backend represents a storage backend (S3, GCS, local file, etc.).
// Implementations handle raw byte transport to/from storage.
type Backend interface {
    // NewWriter creates a writer for the given path/key.
    // The returned writer must be closed after use.
    NewWriter(path string, opts ...WriterOption) (io.WriteCloser, error)

    // NewReader creates a reader for the given path/key.
    // Returns ErrNotFound if the path does not exist.
    NewReader(path string, opts ...ReaderOption) (io.ReadCloser, error)

    // Exists checks if a path exists.
    Exists(path string) (bool, error)

    // Delete removes a path.
    // Returns nil if the path does not exist (idempotent).
    Delete(path string) error

    // List lists paths with the given prefix.
    // Returns an empty slice if no paths match.
    List(prefix string) ([]string, error)

    // Close releases any resources held by the backend.
    Close() error
}
```

### 2.2 RecordWriter Interface

```go
// RecordWriter writes framed records (byte slices) to an underlying writer.
// Implementations handle record delimiting (newlines, length-prefix, etc.).
type RecordWriter interface {
    // Write writes a single record.
    // The record should not contain the delimiter (e.g., no trailing newline for NDJSON).
    Write(data []byte) error

    // Flush flushes any buffered data to the underlying writer.
    Flush() error

    // Close flushes and closes the writer.
    Close() error
}
```

### 2.3 RecordReader Interface

```go
// RecordReader reads framed records from an underlying reader.
type RecordReader interface {
    // Read reads the next record.
    // Returns io.EOF when no more records are available.
    Read() ([]byte, error)

    // Close closes the reader.
    Close() error
}
```

### 2.4 Options

```go
// WriterOption configures a writer.
type WriterOption func(*WriterConfig)

// WriterConfig holds writer configuration.
type WriterConfig struct {
    BufferSize  int               // Buffer size in bytes (0 = default)
    ContentType string            // MIME type hint
    Metadata    map[string]string // Backend-specific metadata
}

// ReaderOption configures a reader.
type ReaderOption func(*ReaderConfig)

// ReaderConfig holds reader configuration.
type ReaderConfig struct {
    BufferSize int // Buffer size in bytes (0 = default)
    Offset     int64 // Start reading from offset (if supported)
    Limit      int64 // Maximum bytes to read (0 = no limit)
}
```

---

## 3. Backend Registration

### 3.1 Registration Pattern

```go
package omnistorage

import (
    "fmt"
    "sync"
)

var (
    backendsMu sync.RWMutex
    backends   = make(map[string]BackendFactory)
)

// BackendFactory creates a Backend from configuration.
type BackendFactory func(config map[string]string) (Backend, error)

// Register registers a backend factory under the given name.
// Typically called from init() in backend packages.
func Register(name string, factory BackendFactory) {
    backendsMu.Lock()
    defer backendsMu.Unlock()
    if factory == nil {
        panic("omnistorage: Register factory is nil")
    }
    if _, dup := backends[name]; dup {
        panic("omnistorage: Register called twice for backend " + name)
    }
    backends[name] = factory
}

// Open opens a backend by name with the given configuration.
func Open(name string, config map[string]string) (Backend, error) {
    backendsMu.RLock()
    factory, ok := backends[name]
    backendsMu.RUnlock()
    if !ok {
        return nil, fmt.Errorf("omnistorage: unknown backend %q", name)
    }
    return factory(config)
}

// Backends returns a list of registered backend names.
func Backends() []string {
    backendsMu.RLock()
    defer backendsMu.RUnlock()
    names := make([]string, 0, len(backends))
    for name := range backends {
        names = append(names, name)
    }
    return names
}
```

### 3.2 Backend Self-Registration

```go
// backend/file/backend.go
package file

import "github.com/grokify/omnistorage"

func init() {
    omnistorage.Register("file", New)
}

func New(config map[string]string) (omnistorage.Backend, error) {
    root := config["root"]
    if root == "" {
        root = "."
    }
    return &Backend{root: root}, nil
}
```

---

## 4. Format Implementations

### 4.1 NDJSON Writer

```go
// format/ndjson/writer.go
package ndjson

import (
    "bufio"
    "io"

    "github.com/grokify/omnistorage"
)

// Writer writes newline-delimited records.
type Writer struct {
    w   *bufio.Writer
    raw io.Writer
}

// NewWriter creates an NDJSON writer wrapping the given io.Writer.
func NewWriter(w io.Writer) *Writer {
    return &Writer{
        w:   bufio.NewWriter(w),
        raw: w,
    }
}

func (w *Writer) Write(data []byte) error {
    if _, err := w.w.Write(data); err != nil {
        return err
    }
    return w.w.WriteByte('\n')
}

func (w *Writer) Flush() error {
    return w.w.Flush()
}

func (w *Writer) Close() error {
    if err := w.w.Flush(); err != nil {
        return err
    }
    if c, ok := w.raw.(io.Closer); ok {
        return c.Close()
    }
    return nil
}

// Ensure Writer implements RecordWriter
var _ omnistorage.RecordWriter = (*Writer)(nil)
```

### 4.2 NDJSON Reader

```go
// format/ndjson/reader.go
package ndjson

import (
    "bufio"
    "io"
    "strings"

    "github.com/grokify/omnistorage"
)

// Reader reads newline-delimited records.
type Reader struct {
    scanner *bufio.Scanner
    raw     io.Reader
}

// NewReader creates an NDJSON reader wrapping the given io.Reader.
func NewReader(r io.Reader) *Reader {
    scanner := bufio.NewScanner(r)
    // Support large lines (1MB)
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, 1024*1024)
    return &Reader{
        scanner: scanner,
        raw:     r,
    }
}

func (r *Reader) Read() ([]byte, error) {
    for r.scanner.Scan() {
        line := strings.TrimSpace(r.scanner.Text())
        if line == "" {
            continue
        }
        return []byte(line), nil
    }
    if err := r.scanner.Err(); err != nil {
        return nil, err
    }
    return nil, io.EOF
}

func (r *Reader) Close() error {
    if c, ok := r.raw.(io.Closer); ok {
        return c.Close()
    }
    return nil
}

var _ omnistorage.RecordReader = (*Reader)(nil)
```

---

## 5. Compression Wrappers

### 5.1 Gzip Writer

```go
// compress/gzip/writer.go
package gzip

import (
    "compress/gzip"
    "io"
)

// Writer wraps an io.Writer with gzip compression.
type Writer struct {
    gw  *gzip.Writer
    raw io.Writer
}

// NewWriter creates a gzip-compressed writer.
func NewWriter(w io.Writer) *Writer {
    return &Writer{
        gw:  gzip.NewWriter(w),
        raw: w,
    }
}

// NewWriterLevel creates a gzip-compressed writer with specified compression level.
func NewWriterLevel(w io.Writer, level int) (*Writer, error) {
    gw, err := gzip.NewWriterLevel(w, level)
    if err != nil {
        return nil, err
    }
    return &Writer{gw: gw, raw: w}, nil
}

func (w *Writer) Write(p []byte) (n int, err error) {
    return w.gw.Write(p)
}

func (w *Writer) Close() error {
    if err := w.gw.Close(); err != nil {
        return err
    }
    if c, ok := w.raw.(io.Closer); ok {
        return c.Close()
    }
    return nil
}

var _ io.WriteCloser = (*Writer)(nil)
```

---

## 6. Backend Implementations

### 6.1 File Backend

```go
// backend/file/backend.go
package file

import (
    "io"
    "os"
    "path/filepath"

    "github.com/grokify/omnistorage"
)

type Backend struct {
    root string
}

func New(config map[string]string) (omnistorage.Backend, error) {
    root := config["root"]
    if root == "" {
        root = "."
    }
    return &Backend{root: root}, nil
}

func (b *Backend) NewWriter(path string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
    fullPath := filepath.Join(b.root, path)

    // Create parent directories
    if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
        return nil, err
    }

    return os.Create(fullPath)
}

func (b *Backend) NewReader(path string, opts ...omnistorage.ReaderOption) (io.ReadCloser, error) {
    fullPath := filepath.Join(b.root, path)
    f, err := os.Open(fullPath)
    if os.IsNotExist(err) {
        return nil, omnistorage.ErrNotFound
    }
    return f, err
}

func (b *Backend) Exists(path string) (bool, error) {
    fullPath := filepath.Join(b.root, path)
    _, err := os.Stat(fullPath)
    if os.IsNotExist(err) {
        return false, nil
    }
    return err == nil, err
}

func (b *Backend) Delete(path string) error {
    fullPath := filepath.Join(b.root, path)
    err := os.Remove(fullPath)
    if os.IsNotExist(err) {
        return nil // Idempotent
    }
    return err
}

func (b *Backend) List(prefix string) ([]string, error) {
    var paths []string
    root := filepath.Join(b.root, prefix)

    err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() {
            rel, _ := filepath.Rel(b.root, path)
            paths = append(paths, rel)
        }
        return nil
    })

    return paths, err
}

func (b *Backend) Close() error {
    return nil
}

var _ omnistorage.Backend = (*Backend)(nil)
```

### 6.2 S3 Backend (outline)

```go
// backend/s3/backend.go
package s3

import (
    "context"
    "io"

    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/grokify/omnistorage"
)

type Backend struct {
    client *s3.Client
    bucket string
}

type Config struct {
    Bucket    string
    Region    string
    Endpoint  string // Custom endpoint for R2, MinIO, etc.
    AccessKey string
    SecretKey string
}

func New(config map[string]string) (omnistorage.Backend, error) {
    // Parse config, create S3 client
    // ...
}

func (b *Backend) NewWriter(path string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
    // Return a writer that buffers and uploads on Close()
    // Or use multipart upload for streaming
}

func (b *Backend) NewReader(path string, opts ...omnistorage.ReaderOption) (io.ReadCloser, error) {
    // GetObject and return body
}

// ... other methods
```

---

## 7. Error Handling

```go
// errors.go
package omnistorage

import "errors"

var (
    // ErrNotFound is returned when a path does not exist.
    ErrNotFound = errors.New("omnistorage: not found")

    // ErrAlreadyExists is returned when a path already exists (if applicable).
    ErrAlreadyExists = errors.New("omnistorage: already exists")

    // ErrPermissionDenied is returned when access is denied.
    ErrPermissionDenied = errors.New("omnistorage: permission denied")

    // ErrBackendClosed is returned when operating on a closed backend.
    ErrBackendClosed = errors.New("omnistorage: backend closed")
)
```

---

## 8. Usage Examples

### 8.1 Basic File Writing

```go
import (
    "github.com/grokify/omnistorage/backend/file"
    "github.com/grokify/omnistorage/format/ndjson"
)

func main() {
    // Create file backend
    backend, _ := file.New(map[string]string{"root": "/data"})
    defer backend.Close()

    // Create writer
    raw, _ := backend.NewWriter("logs/app.ndjson")
    w := ndjson.NewWriter(raw)
    defer w.Close()

    // Write records
    w.Write([]byte(`{"level":"info","msg":"hello"}`))
    w.Write([]byte(`{"level":"error","msg":"oops"}`))
}
```

### 8.2 Compressed S3 Upload

```go
import (
    "github.com/grokify/omnistorage/backend/s3"
    "github.com/grokify/omnistorage/compress/gzip"
    "github.com/grokify/omnistorage/format/ndjson"
)

func main() {
    backend, _ := s3.New(map[string]string{
        "bucket":   "my-bucket",
        "region":   "us-west-2",
        "endpoint": "", // AWS S3
    })
    defer backend.Close()

    raw, _ := backend.NewWriter("logs/2024-01-08.ndjson.gz")
    compressed := gzip.NewWriter(raw)
    w := ndjson.NewWriter(compressed)
    defer w.Close()

    // Write records
    for _, record := range records {
        data, _ := json.Marshal(record)
        w.Write(data)
    }
}
```

### 8.3 Configuration-Driven Backend

```go
import (
    "os"
    "github.com/grokify/omnistorage"
    _ "github.com/grokify/omnistorage/backend/file"
    _ "github.com/grokify/omnistorage/backend/s3"
)

func main() {
    // Backend selected by environment
    backendType := os.Getenv("STORAGE_BACKEND") // "file" or "s3"

    config := map[string]string{
        "root":     os.Getenv("STORAGE_ROOT"),
        "bucket":   os.Getenv("STORAGE_BUCKET"),
        "region":   os.Getenv("STORAGE_REGION"),
        "endpoint": os.Getenv("STORAGE_ENDPOINT"),
    }

    backend, _ := omnistorage.Open(backendType, config)
    defer backend.Close()

    // Use backend...
}
```

---

## 9. Testing Strategy

### 9.1 Unit Tests

- Each backend has unit tests with mock/stub dependencies
- Format and compression layers tested independently
- Interface compliance tests for all implementations

### 9.2 Integration Tests

- File backend: actual filesystem operations
- S3 backend: LocalStack or MinIO container
- Test matrix: all format × compression × backend combinations

### 9.3 Conformance Tests

```go
// conformance_test.go
package omnistorage_test

func TestBackendConformance(t *testing.T, backend omnistorage.Backend) {
    t.Run("WriteRead", func(t *testing.T) { ... })
    t.Run("Exists", func(t *testing.T) { ... })
    t.Run("Delete", func(t *testing.T) { ... })
    t.Run("List", func(t *testing.T) { ... })
    t.Run("NotFound", func(t *testing.T) { ... })
}
```

---

## 10. Performance Considerations

1. **Buffering**: Writers should buffer by default (configurable)
2. **Streaming**: Avoid loading entire files into memory
3. **Connection pooling**: Reuse connections for network backends
4. **Multipart uploads**: Large files should use multipart (S3)
5. **Benchmarks**: Include benchmarks for common operations

---

## 11. Security Considerations

1. **Credential handling**: Never log credentials, use secure config
2. **Path traversal**: Validate paths to prevent `../` attacks
3. **TLS**: Default to TLS for network backends
4. **Permissions**: Respect file permissions, document defaults
