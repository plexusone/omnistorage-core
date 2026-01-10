# Channel Backend

The channel backend uses Go channels for inter-goroutine communication, useful for pipeline processing and streaming data between goroutines.

## Installation

```go
import "github.com/grokify/omnistorage/backend/channel"
```

## Usage

### Basic Usage

```go
backend := channel.New()

// Producer goroutine
go func() {
    w, _ := backend.NewWriter(ctx, "events")
    w.Write([]byte("event1"))
    w.Write([]byte("event2"))
    w.Close() // Signals end of stream
}()

// Consumer goroutine
r, _ := backend.NewReader(ctx, "events")
for {
    buf := make([]byte, 1024)
    n, err := r.Read(buf)
    if err == io.EOF {
        break
    }
    process(buf[:n])
}
r.Close()
```

### Using the Registry

```go
import (
    "github.com/grokify/omnistorage"
    _ "github.com/grokify/omnistorage/backend/channel"
)

backend, _ := omnistorage.Open("channel", map[string]string{
    "buffer_size": "100",
    "persistent":  "true",
})
```

## Configuration

### Options

```go
// Custom buffer size
backend := channel.New(channel.WithBufferSize(50))

// Persistent mode - buffers data for late readers
backend := channel.New(channel.WithPersistence(true))

// Combined
backend := channel.New(
    channel.WithBufferSize(100),
    channel.WithPersistence(true),
)
```

### Registry Config

| Key | Description | Default |
|-----|-------------|---------|
| `buffer_size` | Channel buffer size | 100 |
| `persistent` | Buffer data for late readers | false |

## Features

| Feature | Supported | Notes |
|---------|-----------|-------|
| Read/Write | Yes | Via Go channels |
| Stat | No | Not applicable |
| Copy/Move | No | Not applicable |
| Broadcast | Yes | Send to multiple channels |

## Use Cases

### Pipeline Processing

```go
backend := channel.New()

// Stage 1: Read from source
go func() {
    w, _ := backend.NewWriter(ctx, "stage1")
    for _, item := range sourceData {
        w.Write(processStage1(item))
    }
    w.Close()
}()

// Stage 2: Transform
go func() {
    r, _ := backend.NewReader(ctx, "stage1")
    w, _ := backend.NewWriter(ctx, "stage2")

    for {
        buf := make([]byte, 4096)
        n, err := r.Read(buf)
        if err == io.EOF {
            break
        }
        w.Write(processStage2(buf[:n]))
    }
    r.Close()
    w.Close()
}()

// Stage 3: Consume
r, _ := backend.NewReader(ctx, "stage2")
// ...
```

### Test Fixtures

```go
func TestProcessor(t *testing.T) {
    backend := channel.New(channel.WithPersistence(true))

    // Setup test data
    w, _ := backend.NewWriter(ctx, "input")
    w.Write([]byte("test data"))
    w.Close()

    // Readers can now get the data even after writer closed
    r, _ := backend.NewReader(ctx, "input")
    // ...
}
```

### Event Fan-Out

```go
backend := channel.New()

// Create subscriber channels
for i := 0; i < 3; i++ {
    path := fmt.Sprintf("subscriber/%d", i)
    go func(p string) {
        r, _ := backend.NewReader(ctx, p)
        defer r.Close()
        // Handle events...
    }(path)
}

// Broadcast events to all subscribers
backend.Broadcast(ctx, "subscriber/", []byte("event data"))
```

## Broadcast

Send data to all channels matching a prefix:

```go
// Send to all channels starting with "events/"
err := backend.Broadcast(ctx, "events/", []byte("broadcast message"))
```

## Persistent Mode

When persistence is enabled, data written to a channel is buffered and replayed to new readers:

```go
backend := channel.New(channel.WithPersistence(true))

// Write first
w, _ := backend.NewWriter(ctx, "data")
w.Write([]byte("message 1"))
w.Write([]byte("message 2"))
// Don't close yet

// Reader connects and gets buffered data
r, _ := backend.NewReader(ctx, "data")
// Reads "message 1", "message 2"
```

!!! warning "Memory Usage"
    Persistent mode stores all data in memory. Use with caution for high-volume data.

## Channel Count

Check the number of active channels:

```go
count := backend.ChannelCount()
fmt.Printf("Active channels: %d\n", count)
```

## Best Practices

1. **Close writers** - Signals EOF to readers
2. **Use buffered channels** - Default buffer of 100 prevents blocking
3. **Use persistent mode for testing** - Allows readers to connect after writes
4. **Use Broadcast for fan-out** - More efficient than multiple writes
