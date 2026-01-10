# Transfer Controls

Control transfer behavior with bandwidth limiting, parallel transfers, and retry configuration.

## Parallel Transfers

Control the number of concurrent file transfers:

```go
result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Concurrency: 8, // 8 parallel transfers (default: 4)
})
```

### Guidelines

| Network | Recommended Concurrency |
|---------|------------------------|
| Local/LAN | 8-16 |
| High-speed Internet | 4-8 |
| Slow Internet | 2-4 |
| Rate-limited APIs | 1-2 |

## Bandwidth Limiting

Limit transfer speed with a token bucket rate limiter:

```go
result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    BandwidthLimit: 1024 * 1024, // 1 MB/s
})
```

### Common Limits

```go
// 1 MB/s
BandwidthLimit: 1 * 1024 * 1024

// 10 MB/s
BandwidthLimit: 10 * 1024 * 1024

// 100 KB/s
BandwidthLimit: 100 * 1024
```

### How It Works

The bandwidth limiter uses a token bucket algorithm:

- Tokens represent bytes that can be transferred
- Tokens are added at the specified rate
- Transfers wait for tokens when the bucket is empty
- The bucket can burst up to the limit

## Retry Configuration

Configure automatic retries for failed operations:

```go
retryConfig := sync.DefaultRetryConfig()
retryConfig.MaxRetries = 5
retryConfig.InitialDelay = time.Second
retryConfig.MaxDelay = 30 * time.Second

result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Retry: &retryConfig,
})
```

### RetryConfig

```go
type RetryConfig struct {
    MaxRetries   int           // Maximum retry attempts (default: 3)
    InitialDelay time.Duration // Initial delay (default: 1s)
    MaxDelay     time.Duration // Maximum delay (default: 30s)
    Multiplier   float64       // Delay multiplier (default: 2.0)
    Jitter       float64       // Random jitter (default: 0.1)
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

### Exponential Backoff

Delays increase exponentially with jitter:

| Attempt | Base Delay | With Jitter (±10%) |
|---------|-----------|-------------------|
| 1 | 1s | 0.9s - 1.1s |
| 2 | 2s | 1.8s - 2.2s |
| 3 | 4s | 3.6s - 4.4s |
| 4 | 8s | 7.2s - 8.8s |
| 5 | 16s | 14.4s - 17.6s |

### Retryable Errors

By default, these errors are retried:

- Network timeouts
- Connection resets
- HTTP 429 (Too Many Requests)
- HTTP 500, 502, 503, 504 (Server errors)

## Max Errors

Stop sync after a number of errors:

```go
result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    MaxErrors: 10, // Stop after 10 errors (0 = stop on first)
})
```

## Progress Tracking

Monitor transfer progress:

```go
result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Progress: func(p sync.Progress) {
        percent := float64(p.BytesTransferred) / float64(p.TotalBytes) * 100
        fmt.Printf("\r[%s] %.1f%% (%d/%d files)",
            p.Phase, percent, p.FilesTransferred, p.TotalFiles)
    },
})
```

### Progress Fields

```go
type Progress struct {
    Phase            string // "scanning", "copying", "deleting"
    CurrentFile      string // Current file being processed
    TotalFiles       int    // Total files to process
    FilesTransferred int    // Files completed
    TotalBytes       int64  // Total bytes to transfer
    BytesTransferred int64  // Bytes transferred
}
```

## Metadata Preservation

Preserve file metadata during transfers:

```go
result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    PreserveMetadata: &sync.MetadataOptions{
        ContentType:    true, // Preserve MIME type
        CustomMetadata: true, // Preserve custom metadata
        ModTime:        true, // Preserve modification time
    },
})
```

### Default Metadata Options

```go
func DefaultMetadataOptions() MetadataOptions {
    return MetadataOptions{
        ContentType:    true,
        CustomMetadata: true,
        ModTime:        false, // Requires ExtendedBackend with SetModTime
    }
}
```

## Combined Example

```go
retryConfig := sync.DefaultRetryConfig()
retryConfig.MaxRetries = 5

result, err := sync.Sync(ctx, src, dst, "data/", "backup/", sync.Options{
    // Transfer controls
    Concurrency:    8,              // 8 parallel transfers
    BandwidthLimit: 10 * 1024 * 1024, // 10 MB/s limit
    Retry:          &retryConfig,   // Retry configuration
    MaxErrors:      100,            // Continue despite errors

    // Progress
    Progress: func(p sync.Progress) {
        fmt.Printf("[%s] %d/%d files\n", p.Phase, p.FilesTransferred, p.TotalFiles)
    },

    // Metadata
    PreserveMetadata: sync.DefaultMetadataOptions(),
})
```
