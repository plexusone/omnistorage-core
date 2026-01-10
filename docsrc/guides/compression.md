# Compression Guide

OmniStorage provides compression layers that wrap io.Writer and io.Reader.

## Available Compressors

| Format | Package | Use Case |
|--------|---------|----------|
| Gzip | `compress/gzip` | Universal compatibility |
| Zstandard | `compress/zstd` | Better compression ratio and speed |

## Gzip Compression

### Writing Compressed Data

```go
import (
    "github.com/grokify/omnistorage/backend/file"
    "github.com/grokify/omnistorage/compress/gzip"
)

backend := file.New(file.Config{Root: "/data"})

// Create the writer stack
fileWriter, _ := backend.NewWriter(ctx, "data.txt.gz")
gzipWriter, _ := gzip.NewWriter(fileWriter)

// Write data
gzipWriter.Write([]byte("compressed content"))
gzipWriter.Close() // Important: closes both gzip and file writers
```

### Reading Compressed Data

```go
fileReader, _ := backend.NewReader(ctx, "data.txt.gz")
gzipReader, _ := gzip.NewReader(fileReader)
defer gzipReader.Close()

data, _ := io.ReadAll(gzipReader)
```

### Compression Level

```go
// Default compression
gzipWriter, _ := gzip.NewWriter(fileWriter)

// Best compression (slower)
gzipWriter, _ := gzip.NewWriterLevel(fileWriter, gzip.BestCompression)

// Best speed (larger files)
gzipWriter, _ := gzip.NewWriterLevel(fileWriter, gzip.BestSpeed)

// No compression (for testing)
gzipWriter, _ := gzip.NewWriterLevel(fileWriter, gzip.NoCompression)
```

## Zstandard Compression

Zstandard (zstd) provides better compression ratio and faster decompression than gzip.

### Writing with Zstd

```go
import "github.com/grokify/omnistorage/compress/zstd"

fileWriter, _ := backend.NewWriter(ctx, "data.txt.zst")
zstdWriter, _ := zstd.NewWriter(fileWriter)

zstdWriter.Write([]byte("compressed content"))
zstdWriter.Close()
```

### Reading with Zstd

```go
fileReader, _ := backend.NewReader(ctx, "data.txt.zst")
zstdReader, _ := zstd.NewReader(fileReader)
defer zstdReader.Close()

data, _ := io.ReadAll(zstdReader)
```

### Compression Level

```go
// Default level
zstdWriter, _ := zstd.NewWriter(fileWriter)

// Custom level (1-22, default is 3)
zstdWriter, _ := zstd.NewWriterLevel(fileWriter, 10)
```

## Combining with Format Layers

Stack compression with format layers:

```go
import (
    "github.com/grokify/omnistorage/backend/s3"
    "github.com/grokify/omnistorage/compress/gzip"
    "github.com/grokify/omnistorage/format/ndjson"
)

s3Backend, _ := s3.New(s3Config)

// Create writer stack: S3 -> Gzip -> NDJSON
raw, _ := s3Backend.NewWriter(ctx, "logs/2024-01-08.ndjson.gz")
compressed := gzip.NewWriter(raw)
writer := ndjson.NewWriter(compressed)

// Write records
for _, record := range records {
    data, _ := json.Marshal(record)
    writer.Write(data)
}
writer.Close() // Closes entire stack
```

### Reading the Stack

```go
raw, _ := s3Backend.NewReader(ctx, "logs/2024-01-08.ndjson.gz")
decompressed, _ := gzip.NewReader(raw)
reader := ndjson.NewReader(decompressed)

for {
    record, err := reader.Read()
    if err == io.EOF {
        break
    }
    process(record)
}
reader.Close()
```

## Choosing a Compressor

| Factor | Gzip | Zstd |
|--------|------|------|
| Compatibility | Universal | Growing |
| Compression ratio | Good | Better |
| Compression speed | Moderate | Fast |
| Decompression speed | Moderate | Very fast |
| Memory usage | Low | Low-Medium |

### When to Use Gzip

- Compatibility is important (web servers, browsers)
- Files will be served over HTTP
- Working with legacy systems

### When to Use Zstd

- Better compression is important
- Fast decompression is needed
- Processing large data volumes
- Internal/controlled environments

## File Extensions

Follow conventions for file extensions:

| Format | Extension |
|--------|-----------|
| Gzip | `.gz` |
| Zstd | `.zst` or `.zstd` |

Combine with format extensions:

- `data.json.gz` - Gzip-compressed JSON
- `logs.ndjson.zst` - Zstd-compressed NDJSON

## Error Handling

```go
gzipReader, err := gzip.NewReader(fileReader)
if err != nil {
    // Invalid gzip header or corrupted data
    return fmt.Errorf("failed to create gzip reader: %w", err)
}
```

## Best Practices

1. **Close writers in reverse order** - Or just close the outermost writer
2. **Use appropriate compression level** - Balance speed vs size
3. **Follow naming conventions** - Use `.gz` or `.zst` extensions
4. **Stream large files** - Don't load entire files into memory
