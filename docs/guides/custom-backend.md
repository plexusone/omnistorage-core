# Custom Backend Guide

This guide shows how to implement a custom omnistorage backend.

## Overview

A backend implements the `Backend` interface:

```go
type Backend interface {
    NewWriter(ctx context.Context, path string, opts ...WriterOption) (io.WriteCloser, error)
    NewReader(ctx context.Context, path string, opts ...ReaderOption) (io.ReadCloser, error)
    Exists(ctx context.Context, path string) (bool, error)
    Delete(ctx context.Context, path string) error
    List(ctx context.Context, prefix string) ([]string, error)
    Close() error
}
```

## Basic Implementation

### Step 1: Create the Package

```go
// backend/mycloud/backend.go
package mycloud

import (
    "context"
    "io"

    "github.com/grokify/omnistorage"
)
```

### Step 2: Define the Backend Struct

```go
type Backend struct {
    client *MyCloudClient
    bucket string
    closed bool
    mu     sync.RWMutex
}

type Config struct {
    Bucket   string
    APIKey   string
    Endpoint string
}

func New(config Config) (*Backend, error) {
    client, err := NewMyCloudClient(config.APIKey, config.Endpoint)
    if err != nil {
        return nil, err
    }

    return &Backend{
        client: client,
        bucket: config.Bucket,
    }, nil
}
```

### Step 3: Implement NewWriter

```go
func (b *Backend) NewWriter(ctx context.Context, path string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
    if err := b.checkClosed(); err != nil {
        return nil, err
    }

    if err := ctx.Err(); err != nil {
        return nil, err
    }

    if path == "" {
        return nil, omnistorage.ErrInvalidPath
    }

    // Apply options
    config := omnistorage.DefaultWriterConfig()
    for _, opt := range opts {
        opt(&config)
    }

    return &writer{
        backend:     b,
        path:        path,
        contentType: config.ContentType,
        buffer:      &bytes.Buffer{},
    }, nil
}

type writer struct {
    backend     *Backend
    path        string
    contentType string
    buffer      *bytes.Buffer
    closed      bool
}

func (w *writer) Write(p []byte) (n int, err error) {
    if w.closed {
        return 0, omnistorage.ErrWriterClosed
    }
    return w.buffer.Write(p)
}

func (w *writer) Close() error {
    if w.closed {
        return nil
    }
    w.closed = true

    // Upload buffered data to cloud
    return w.backend.client.Upload(w.path, w.buffer.Bytes(), w.contentType)
}
```

### Step 4: Implement NewReader

```go
func (b *Backend) NewReader(ctx context.Context, path string, opts ...omnistorage.ReaderOption) (io.ReadCloser, error) {
    if err := b.checkClosed(); err != nil {
        return nil, err
    }

    if err := ctx.Err(); err != nil {
        return nil, err
    }

    if path == "" {
        return nil, omnistorage.ErrInvalidPath
    }

    // Download from cloud
    data, err := b.client.Download(path)
    if err != nil {
        if isNotFoundError(err) {
            return nil, omnistorage.ErrNotFound
        }
        return nil, err
    }

    return io.NopCloser(bytes.NewReader(data)), nil
}
```

### Step 5: Implement Other Methods

```go
func (b *Backend) Exists(ctx context.Context, path string) (bool, error) {
    if err := b.checkClosed(); err != nil {
        return false, err
    }

    exists, err := b.client.Exists(path)
    if err != nil {
        return false, err
    }
    return exists, nil
}

func (b *Backend) Delete(ctx context.Context, path string) error {
    if err := b.checkClosed(); err != nil {
        return err
    }

    err := b.client.Delete(path)
    if isNotFoundError(err) {
        return nil // Idempotent
    }
    return err
}

func (b *Backend) List(ctx context.Context, prefix string) ([]string, error) {
    if err := b.checkClosed(); err != nil {
        return nil, err
    }

    return b.client.List(prefix)
}

func (b *Backend) Close() error {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.closed {
        return nil
    }
    b.closed = true

    return b.client.Close()
}

func (b *Backend) checkClosed() error {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        return omnistorage.ErrBackendClosed
    }
    return nil
}
```

### Step 6: Register the Backend

```go
func init() {
    omnistorage.Register("mycloud", NewFromConfig)
}

func NewFromConfig(config map[string]string) (omnistorage.Backend, error) {
    return New(Config{
        Bucket:   config["bucket"],
        APIKey:   config["api_key"],
        Endpoint: config["endpoint"],
    })
}
```

## Extended Backend

For advanced features, implement `ExtendedBackend`:

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

### Implementing Stat

```go
func (b *Backend) Stat(ctx context.Context, path string) (omnistorage.ObjectInfo, error) {
    if err := b.checkClosed(); err != nil {
        return nil, err
    }

    meta, err := b.client.GetMetadata(path)
    if err != nil {
        if isNotFoundError(err) {
            return nil, omnistorage.ErrNotFound
        }
        return nil, err
    }

    return &objectInfo{
        name:    path,
        size:    meta.Size,
        modTime: meta.ModTime,
    }, nil
}

type objectInfo struct {
    name    string
    size    int64
    modTime time.Time
}

func (o *objectInfo) Name() string              { return o.name }
func (o *objectInfo) Size() int64               { return o.size }
func (o *objectInfo) ModTime() time.Time        { return o.modTime }
func (o *objectInfo) IsDir() bool               { return false }
func (o *objectInfo) Hash(t omnistorage.HashType) string { return "" }
func (o *objectInfo) MimeType() string          { return "" }
func (o *objectInfo) Metadata() map[string]string { return nil }
```

### Implementing Features

```go
func (b *Backend) Features() omnistorage.Features {
    return omnistorage.Features{
        Copy:           true,  // Server-side copy supported
        Move:           true,  // Server-side move supported
        Purge:          false, // Recursive delete not supported
        SetModTime:     false,
        CustomMetadata: true,
    }
}
```

## Testing

Create conformance tests:

```go
func TestBackendConformance(t *testing.T) {
    backend, _ := mycloud.New(testConfig)
    defer backend.Close()

    ctx := context.Background()

    t.Run("WriteRead", func(t *testing.T) {
        data := []byte("test data")

        w, _ := backend.NewWriter(ctx, "test.txt")
        w.Write(data)
        w.Close()

        r, _ := backend.NewReader(ctx, "test.txt")
        result, _ := io.ReadAll(r)
        r.Close()

        if !bytes.Equal(result, data) {
            t.Errorf("got %q, want %q", result, data)
        }
    })

    // More tests...
}
```

## Best Practices

1. **Handle context cancellation** - Check `ctx.Err()` in long operations
2. **Use standard errors** - Return `omnistorage.ErrNotFound`, etc.
3. **Make delete idempotent** - Return nil for non-existent paths
4. **Implement proper closing** - Release resources in `Close()`
5. **Thread safety** - Use mutexes for shared state
6. **Register in init()** - For automatic registration
