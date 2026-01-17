# Multi-Writer Guide

The multi package provides fan-out writing to multiple backends simultaneously.

## Overview

Write the same data to multiple storage backends at once:

- Replication across storage providers
- Writing to both local and remote storage
- Backup during write operations
- Testing with multiple backends

## Basic Usage

```go
import "github.com/grokify/omnistorage/multi"

// Create backends
local := file.New(file.Config{Root: "/data"})
s3Backend, _ := s3.New(s3.Config{Bucket: "my-bucket"})
gcsBackend, _ := gcs.New(gcs.Config{Bucket: "my-bucket"})

// Create multi-writer
mw, err := multi.NewWriter(local, s3Backend, gcsBackend)
if err != nil {
    log.Fatal(err)
}

// Write to all backends simultaneously
w, _ := mw.NewWriter(ctx, "data/file.json")
w.Write([]byte(`{"key": "value"}`))
w.Close()
```

## Write Modes

### WriteAll (Default)

All backends must succeed. If any backend fails, the entire write fails.

```go
mw, _ := multi.NewWriterWithOptions(
    []omnistorage.Backend{b1, b2, b3},
    multi.WithMode(multi.WriteAll),
)
```

Use when: Data must be in all backends or none (strong consistency).

### WriteBestEffort

Write to all backends but continue on failure. Errors are collected and returned.

```go
mw, _ := multi.NewWriterWithOptions(
    []omnistorage.Backend{b1, b2, b3},
    multi.WithMode(multi.WriteBestEffort),
)

w, _ := mw.NewWriter(ctx, "file.txt")
_, _ = w.Write(data)
err := w.Close()

// Check for partial failures
if err != nil {
    if me, ok := err.(*multi.MultiError); ok {
        for _, e := range me.All() {
            log.Printf("Backend error: %v", e)
        }
    }
}
```

Use when: Some backends can fail without blocking the operation.

### WriteQuorum

Requires a majority of backends to succeed.

```go
mw, _ := multi.NewWriterWithOptions(
    []omnistorage.Backend{b1, b2, b3}, // 3 backends
    multi.WithMode(multi.WriteQuorum),
)

// Write succeeds if 2+ backends succeed
w, _ := mw.NewWriter(ctx, "file.txt")
```

Use when: Fault tolerance with majority agreement (similar to distributed systems).

## Error Handling

The multi-writer returns `*MultiError` when multiple errors occur:

```go
w, err := mw.NewWriter(ctx, "file.txt")
if err != nil {
    if me, ok := err.(*multi.MultiError); ok {
        // Multiple errors
        fmt.Printf("First error: %v\n", me.Error())
        for _, e := range me.All() {
            fmt.Printf("- %v\n", e)
        }
    }
    return err
}
```

### MultiError Methods

```go
type MultiError struct {
    Errors []error
}

func (e *MultiError) Error() string     // First error + "(and more errors)"
func (e *MultiError) Unwrap() error     // First error (for errors.Is/As)
func (e *MultiError) All() []error      // All errors
```

## Backend Count

Check the number of active backends:

```go
count := mw.Backends()
fmt.Printf("Writing to %d backends\n", count)
```

## Use Cases

### Local + Cloud Backup

Write to local storage and backup to cloud simultaneously:

```go
local := file.New(file.Config{Root: "/data"})
cloud, _ := s3.New(s3.Config{Bucket: "backups"})

mw, _ := multi.NewWriterWithOptions(
    []omnistorage.Backend{local, cloud},
    multi.WithMode(multi.WriteBestEffort), // Continue if cloud fails
)

// Data is written locally and backed up to cloud
w, _ := mw.NewWriter(ctx, "important.dat")
```

### Multi-Region Replication

Write to multiple regions for availability:

```go
usEast, _ := s3.New(s3.Config{Bucket: "data", Region: "us-east-1"})
usWest, _ := s3.New(s3.Config{Bucket: "data", Region: "us-west-2"})
euWest, _ := s3.New(s3.Config{Bucket: "data", Region: "eu-west-1"})

mw, _ := multi.NewWriterWithOptions(
    []omnistorage.Backend{usEast, usWest, euWest},
    multi.WithMode(multi.WriteQuorum), // 2 of 3 must succeed
)
```

### Test + Production

Write to both test and production backends:

```go
prod, _ := s3.New(prodConfig)
test := memory.New() // In-memory for inspection

mw, _ := multi.NewWriter(prod, test)

// After writing, can inspect test backend
```

## Nil Backend Handling

Nil backends are automatically filtered:

```go
var optionalBackend omnistorage.Backend // may be nil

mw, err := multi.NewWriter(
    requiredBackend,
    optionalBackend, // Ignored if nil
)
// mw has 1 backend if optionalBackend is nil
```

## Best Practices

1. **Choose the right mode** - WriteAll for consistency, WriteBestEffort for availability
2. **Handle MultiError** - Check for partial failures in best-effort mode
3. **Close writers** - Ensures all backends complete their writes
4. **Consider latency** - Writes complete when the slowest backend finishes
