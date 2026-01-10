# Interfaces Reference

This page documents all public interfaces in omnistorage.

## Backend

The core interface for all storage backends.

```go
type Backend interface {
    // NewWriter creates a writer for the given path/key.
    // The returned writer must be closed after use.
    NewWriter(ctx context.Context, path string, opts ...WriterOption) (io.WriteCloser, error)

    // NewReader creates a reader for the given path/key.
    // Returns ErrNotFound if the path does not exist.
    NewReader(ctx context.Context, path string, opts ...ReaderOption) (io.ReadCloser, error)

    // Exists checks if a path exists.
    Exists(ctx context.Context, path string) (bool, error)

    // Delete removes a path.
    // Returns nil if the path does not exist (idempotent).
    Delete(ctx context.Context, path string) error

    // List lists paths with the given prefix.
    // Returns an empty slice if no paths match.
    List(ctx context.Context, prefix string) ([]string, error)

    // Close releases any resources held by the backend.
    Close() error
}
```

### Usage

```go
backend := file.New(file.Config{Root: "/data"})
defer backend.Close()

// Write
w, _ := backend.NewWriter(ctx, "file.txt")
w.Write([]byte("data"))
w.Close()

// Read
r, _ := backend.NewReader(ctx, "file.txt")
data, _ := io.ReadAll(r)
r.Close()

// Check existence
exists, _ := backend.Exists(ctx, "file.txt")

// List
files, _ := backend.List(ctx, "prefix/")

// Delete
backend.Delete(ctx, "file.txt")
```

## ExtendedBackend

Extended interface for metadata and server-side operations.

```go
type ExtendedBackend interface {
    Backend

    // Stat returns metadata for the path.
    Stat(ctx context.Context, path string) (ObjectInfo, error)

    // Mkdir creates a directory.
    Mkdir(ctx context.Context, path string) error

    // Rmdir removes an empty directory.
    Rmdir(ctx context.Context, path string) error

    // Copy copies a file from src to dst (server-side when possible).
    Copy(ctx context.Context, src, dst string) error

    // Move moves a file from src to dst (server-side when possible).
    Move(ctx context.Context, src, dst string) error

    // Features returns the backend's capabilities.
    Features() Features
}
```

### Usage

```go
// Check if backend supports extended operations
if ext, ok := omnistorage.AsExtended(backend); ok {
    // Get metadata
    info, _ := ext.Stat(ctx, "file.txt")
    fmt.Printf("Size: %d\n", info.Size())

    // Server-side copy
    if ext.Features().Copy {
        ext.Copy(ctx, "src.txt", "dst.txt")
    }

    // Directory operations
    ext.Mkdir(ctx, "new-folder")
    ext.Rmdir(ctx, "empty-folder")
}
```

## ObjectInfo

Metadata for a file or object.

```go
type ObjectInfo interface {
    // Name returns the base name of the file.
    Name() string

    // Size returns the file size in bytes.
    Size() int64

    // ModTime returns the modification time.
    ModTime() time.Time

    // IsDir returns true if this is a directory.
    IsDir() bool

    // Hash returns the hash of the specified type, or empty string if unavailable.
    Hash(HashType) string

    // MimeType returns the MIME type, or empty string if unknown.
    MimeType() string

    // Metadata returns custom metadata key-value pairs.
    Metadata() map[string]string
}
```

### Usage

```go
info, _ := ext.Stat(ctx, "file.txt")

fmt.Printf("Name: %s\n", info.Name())
fmt.Printf("Size: %d bytes\n", info.Size())
fmt.Printf("Modified: %s\n", info.ModTime())
fmt.Printf("Is Directory: %v\n", info.IsDir())
fmt.Printf("MD5: %s\n", info.Hash(omnistorage.HashMD5))
fmt.Printf("Content-Type: %s\n", info.MimeType())
```

## Features

Backend capability flags.

```go
type Features struct {
    Copy           bool // Server-side copy
    Move           bool // Server-side move
    Purge          bool // Recursive delete
    SetModTime     bool // Set modification time
    CustomMetadata bool // Custom metadata support
}
```

### Usage

```go
features := ext.Features()

if features.Copy {
    // Use efficient server-side copy
    ext.Copy(ctx, src, dst)
} else {
    // Fall back to read + write
    omnistorage.CopyPath(ctx, backend, src, backend, dst)
}
```

## RecordWriter

For streaming record-oriented data.

```go
type RecordWriter interface {
    // Write writes a single record.
    Write(data []byte) error

    // Flush flushes buffered data.
    Flush() error

    // Close flushes and closes the writer.
    Close() error
}
```

### Usage

```go
import "github.com/grokify/omnistorage/format/ndjson"

w, _ := backend.NewWriter(ctx, "records.ndjson")
writer := ndjson.NewWriter(w)

writer.Write([]byte(`{"id":1}`))
writer.Write([]byte(`{"id":2}`))
writer.Flush() // Flush buffered data
writer.Close()
```

## RecordReader

For reading record-oriented data.

```go
type RecordReader interface {
    // Read reads the next record.
    // Returns io.EOF when no more records are available.
    Read() ([]byte, error)

    // Close closes the reader.
    Close() error
}
```

### Usage

```go
r, _ := backend.NewReader(ctx, "records.ndjson")
reader := ndjson.NewReader(r)
defer reader.Close()

for {
    record, err := reader.Read()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    process(record)
}
```

## HashType

Supported hash types for checksums.

```go
type HashType int

const (
    HashNone HashType = iota
    HashMD5
    HashSHA1
    HashSHA256
    HashCRC32
)
```

### Usage

```go
info, _ := ext.Stat(ctx, "file.txt")

md5 := info.Hash(omnistorage.HashMD5)
sha256 := info.Hash(omnistorage.HashSHA256)
```

## BackendFactory

Factory function for creating backends from configuration.

```go
type BackendFactory func(config map[string]string) (Backend, error)
```

### Usage

```go
// Register a factory
omnistorage.Register("mybackend", func(config map[string]string) (omnistorage.Backend, error) {
    return mybackend.New(mybackend.Config{
        Setting: config["setting"],
    })
})

// Open using the factory
backend, _ := omnistorage.Open("mybackend", map[string]string{
    "setting": "value",
})
```
