# Memory Backend

The memory backend provides in-memory storage, ideal for testing and temporary data.

## Installation

```go
import "github.com/grokify/omnistorage/backend/memory"
```

## Usage

### Basic Usage

```go
backend := memory.New()
defer backend.Close()

// Write
w, _ := backend.NewWriter(ctx, "test/data.json")
w.Write([]byte(`{"key": "value"}`))
w.Close()

// Read
r, _ := backend.NewReader(ctx, "test/data.json")
data, _ := io.ReadAll(r)
r.Close()
```

### Using the Registry

```go
import (
    "github.com/grokify/omnistorage"
    _ "github.com/grokify/omnistorage/backend/memory"
)

backend, _ := omnistorage.Open("memory", nil)
```

## Features

The memory backend implements `ExtendedBackend`:

| Feature | Supported | Notes |
|---------|-----------|-------|
| Stat | Yes | Full metadata |
| Copy | Yes | In-memory copy |
| Move | Yes | Rename + delete |
| Mkdir | Yes | Virtual directories |
| Rmdir | Yes | Removes empty directories |

## Use Cases

### Testing

```go
func TestMyFunction(t *testing.T) {
    backend := memory.New()
    defer backend.Close()

    // Use backend in tests
    err := myFunction(backend)
    if err != nil {
        t.Fatal(err)
    }

    // Verify results
    r, _ := backend.NewReader(ctx, "output.txt")
    data, _ := io.ReadAll(r)
    r.Close()

    if string(data) != "expected" {
        t.Errorf("got %q, want %q", data, "expected")
    }
}
```

### Temporary Storage

```go
// Use memory backend for intermediate processing
mem := memory.New()
defer mem.Close()

// Process data through memory
w, _ := mem.NewWriter(ctx, "temp.json")
encoder := json.NewEncoder(w)
encoder.Encode(data)
w.Close()

// Read and send elsewhere
r, _ := mem.NewReader(ctx, "temp.json")
io.Copy(destination, r)
r.Close()
```

### Sync Testing

```go
// Test sync between memory backends
src := memory.New()
dst := memory.New()

// Populate source
w, _ := src.NewWriter(ctx, "file.txt")
w.Write([]byte("content"))
w.Close()

// Sync
result, _ := sync.Sync(ctx, src, dst, "", "", sync.Options{})

// Verify
exists, _ := dst.Exists(ctx, "file.txt")
// exists == true
```

## Extended Operations

```go
ext := backend.(*memory.Backend)

// Get metadata
info, _ := ext.Stat(ctx, "file.txt")
fmt.Printf("Size: %d\n", info.Size())

// Copy in memory
ext.Copy(ctx, "src.txt", "dst.txt")

// Move (rename)
ext.Move(ctx, "old.txt", "new.txt")
```

## Memory Considerations

- Data is stored in memory as `[]byte` slices
- No persistence - data is lost when the backend is closed
- Suitable for testing and temporary data
- Not suitable for large files or production storage

## Thread Safety

The memory backend is thread-safe for concurrent read/write operations.
