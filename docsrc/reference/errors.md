# Errors Reference

This page documents all error types in omnistorage.

## Standard Errors

Omnistorage defines standard errors for common cases:

```go
var (
    // ErrNotFound is returned when a path does not exist.
    ErrNotFound = errors.New("omnistorage: not found")

    // ErrAlreadyExists is returned when a path already exists (if applicable).
    ErrAlreadyExists = errors.New("omnistorage: already exists")

    // ErrPermissionDenied is returned when access is denied.
    ErrPermissionDenied = errors.New("omnistorage: permission denied")

    // ErrBackendClosed is returned when operating on a closed backend.
    ErrBackendClosed = errors.New("omnistorage: backend closed")

    // ErrInvalidPath is returned when a path is invalid or empty.
    ErrInvalidPath = errors.New("omnistorage: invalid path")

    // ErrWriterClosed is returned when writing to a closed writer.
    ErrWriterClosed = errors.New("omnistorage: writer closed")

    // ErrReaderClosed is returned when reading from a closed reader.
    ErrReaderClosed = errors.New("omnistorage: reader closed")
)
```

## Checking Errors

Use `errors.Is()` to check for specific errors:

```go
r, err := backend.NewReader(ctx, "file.txt")
if err != nil {
    if errors.Is(err, omnistorage.ErrNotFound) {
        log.Println("File not found")
        return nil
    }
    if errors.Is(err, omnistorage.ErrPermissionDenied) {
        log.Println("Access denied")
        return err
    }
    return err
}
```

## Error Scenarios

### ErrNotFound

Returned when a file or path doesn't exist:

```go
// Reading a non-existent file
r, err := backend.NewReader(ctx, "missing.txt")
// err == omnistorage.ErrNotFound

// Stat on non-existent path
info, err := ext.Stat(ctx, "missing.txt")
// err == omnistorage.ErrNotFound
```

### ErrBackendClosed

Returned when operating on a closed backend:

```go
backend.Close()

// Any operation after close
_, err := backend.NewWriter(ctx, "file.txt")
// err == omnistorage.ErrBackendClosed
```

### ErrInvalidPath

Returned when a path is empty or invalid:

```go
// Empty path
_, err := backend.NewWriter(ctx, "")
// err == omnistorage.ErrInvalidPath
```

### ErrWriterClosed

Returned when writing to a closed writer:

```go
w, _ := backend.NewWriter(ctx, "file.txt")
w.Close()

_, err := w.Write([]byte("data"))
// err == omnistorage.ErrWriterClosed
```

### ErrReaderClosed

Returned when reading from a closed reader:

```go
r, _ := backend.NewReader(ctx, "file.txt")
r.Close()

_, err := r.Read(buf)
// err == omnistorage.ErrReaderClosed
```

## Multi-Writer Errors

The multi-writer returns `*MultiError` when multiple errors occur:

```go
type MultiError struct {
    Errors []error
}

func (e *MultiError) Error() string   // First error message
func (e *MultiError) Unwrap() error   // First error (for errors.Is)
func (e *MultiError) All() []error    // All errors
```

### Handling MultiError

```go
w, err := mw.NewWriter(ctx, "file.txt")
if err != nil {
    if me, ok := err.(*multi.MultiError); ok {
        fmt.Println("Multiple errors occurred:")
        for _, e := range me.All() {
            fmt.Printf("  - %v\n", e)
        }
    }
    return err
}
```

### Checking Wrapped Errors

```go
// errors.Is works with the first wrapped error
if errors.Is(err, omnistorage.ErrPermissionDenied) {
    // First error was permission denied
}
```

## Sync Errors

### RetryError

Returned when all retries are exhausted:

```go
type RetryError struct {
    Attempts int
    LastErr  error
}

func (e *RetryError) Error() string
func (e *RetryError) Unwrap() error
```

### Handling Retry Errors

```go
result, err := sync.Sync(ctx, src, dst, "", "", opts)
if err != nil {
    var retryErr *sync.RetryError
    if errors.As(err, &retryErr) {
        fmt.Printf("Failed after %d attempts: %v\n",
            retryErr.Attempts, retryErr.LastErr)
    }
}
```

## Context Errors

Backend operations respect context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Operation may return context.DeadlineExceeded
r, err := backend.NewReader(ctx, "large-file.dat")
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Operation timed out")
}

// Or context.Canceled if cancelled
if errors.Is(err, context.Canceled) {
    log.Println("Operation was cancelled")
}
```

## Best Practices

1. **Use errors.Is()** - Don't compare errors directly with `==`
2. **Check specific errors first** - Handle ErrNotFound before generic errors
3. **Wrap errors with context** - Use `fmt.Errorf("failed to read: %w", err)`
4. **Handle MultiError** - Check `All()` for complete error list
5. **Respect context** - Pass context to all operations
