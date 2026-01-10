# Options Reference

This page documents all option types in omnistorage.

## WriterOption

Options for creating writers.

```go
type WriterOption func(*WriterConfig)

type WriterConfig struct {
    BufferSize  int               // Buffer size in bytes (0 = default)
    ContentType string            // MIME type hint
    Metadata    map[string]string // Backend-specific metadata
}
```

### Available Options

```go
// Set content type
omnistorage.WithContentType("application/json")

// Set custom metadata
omnistorage.WithMetadata(map[string]string{
    "author": "john",
    "version": "1.0",
})

// Set buffer size
omnistorage.WithBufferSize(64 * 1024) // 64 KB
```

### Usage

```go
w, _ := backend.NewWriter(ctx, "data.json",
    omnistorage.WithContentType("application/json"),
    omnistorage.WithMetadata(map[string]string{
        "source": "api",
    }),
)
```

## ReaderOption

Options for creating readers.

```go
type ReaderOption func(*ReaderConfig)

type ReaderConfig struct {
    BufferSize int   // Buffer size in bytes (0 = default)
    Offset     int64 // Start reading from offset (if supported)
    Limit      int64 // Maximum bytes to read (0 = no limit)
}
```

### Available Options

```go
// Set buffer size
omnistorage.WithReaderBufferSize(64 * 1024)

// Set offset (range read)
omnistorage.WithOffset(1024)

// Set limit
omnistorage.WithLimit(4096)
```

### Usage

```go
// Read bytes 1024-5120
r, _ := backend.NewReader(ctx, "large-file.dat",
    omnistorage.WithOffset(1024),
    omnistorage.WithLimit(4096),
)
```

## Sync Options

Options for sync operations.

```go
type Options struct {
    // Comparison
    DeleteExtra   bool // Delete files in dst not in src
    Checksum      bool // Compare by checksum vs modtime/size
    SizeOnly      bool // Compare by size only
    IgnoreTime    bool // Ignore modification time
    IgnoreSize    bool // Ignore size differences

    // Behavior
    DryRun         bool // Report changes without making them
    IgnoreExisting bool // Skip files that exist in destination
    MaxErrors      int  // Stop after N errors (0 = first error)

    // Transfer controls
    Concurrency    int              // Parallel transfers (default: 4)
    BandwidthLimit int64            // Rate limit in bytes/second
    Retry          *RetryConfig     // Retry configuration
    Progress       func(Progress)   // Progress callback

    // Filtering
    Filter         *filter.Filter   // Include/exclude filter
    DeleteExcluded bool             // Delete excluded files from dst

    // Metadata
    PreserveMetadata *MetadataOptions // Metadata preservation
}
```

### Common Configurations

```go
// Mirror sync (make dst match src exactly)
sync.Options{
    DeleteExtra: true,
}

// Safe copy (don't delete, skip existing)
sync.Options{
    IgnoreExisting: true,
}

// Checksum verification
sync.Options{
    Checksum: true,
}

// Dry run preview
sync.Options{
    DryRun: true,
}

// Full featured
sync.Options{
    DeleteExtra:    true,
    Checksum:       true,
    Concurrency:    8,
    BandwidthLimit: 10 * 1024 * 1024, // 10 MB/s
    Progress: func(p sync.Progress) {
        fmt.Printf("%s: %d/%d\n", p.Phase, p.FilesTransferred, p.TotalFiles)
    },
}
```

## RetryConfig

Configuration for automatic retries.

```go
type RetryConfig struct {
    MaxRetries   int           // Maximum retry attempts
    InitialDelay time.Duration // Initial delay between retries
    MaxDelay     time.Duration // Maximum delay
    Multiplier   float64       // Delay multiplier for exponential backoff
    Jitter       float64       // Random jitter factor (0-1)
}
```

### Default Configuration

```go
func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxRetries:   3,
        InitialDelay: time.Second,
        MaxDelay:     30 * time.Second,
        Multiplier:   2.0,
        Jitter:       0.1,
    }
}
```

### Usage

```go
retryConfig := sync.DefaultRetryConfig()
retryConfig.MaxRetries = 5
retryConfig.MaxDelay = time.Minute

result, _ := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Retry: &retryConfig,
})
```

## MetadataOptions

Options for metadata preservation.

```go
type MetadataOptions struct {
    ContentType    bool // Preserve MIME type
    CustomMetadata bool // Preserve custom metadata
    ModTime        bool // Preserve modification time
}
```

### Default Configuration

```go
func DefaultMetadataOptions() MetadataOptions {
    return MetadataOptions{
        ContentType:    true,
        CustomMetadata: true,
        ModTime:        false,
    }
}
```

### Usage

```go
result, _ := sync.Sync(ctx, src, dst, "", "", sync.Options{
    PreserveMetadata: &sync.MetadataOptions{
        ContentType:    true,
        CustomMetadata: true,
        ModTime:        true, // Requires SetModTime support
    },
})
```

## Filter Options

Options for creating filters.

```go
// Pattern matching
filter.Include("*.json")
filter.Exclude("*.tmp")

// Size filters
filter.MinSize(1024)       // Minimum 1 KB
filter.MaxSize(100 * MB)   // Maximum 100 MB

// Age filters
filter.MinAge(24 * time.Hour)  // Older than 1 day
filter.MaxAge(7 * 24 * time.Hour) // Newer than 7 days
```

### Usage

```go
f := filter.New(
    filter.Include("*.json"),
    filter.Exclude("test_*.json"),
    filter.MinSize(100),
    filter.MaxAge(30 * 24 * time.Hour),
)

result, _ := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Filter: f,
})
```

## Channel Backend Options

Options for the channel backend.

```go
// Set channel buffer size
channel.WithBufferSize(100) // Default: 100

// Enable persistence (buffer for late readers)
channel.WithPersistence(true) // Default: false
```

### Usage

```go
backend := channel.New(
    channel.WithBufferSize(50),
    channel.WithPersistence(true),
)
```

## Multi-Writer Options

Options for multi-writer.

```go
type WriteMode int

const (
    WriteAll        WriteMode = iota // All backends must succeed
    WriteBestEffort                   // Continue on failure
    WriteQuorum                       // Majority must succeed
)

// Set write mode
multi.WithMode(multi.WriteQuorum)
```

### Usage

```go
mw, _ := multi.NewWriterWithOptions(
    []omnistorage.Backend{b1, b2, b3},
    multi.WithMode(multi.WriteBestEffort),
)
```
