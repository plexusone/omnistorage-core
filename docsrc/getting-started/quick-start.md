# Quick Start

This guide covers the most common omnistorage operations.

## Basic Read/Write

```go
package main

import (
    "context"
    "io"
    "log"

    "github.com/grokify/omnistorage/backend/file"
)

func main() {
    ctx := context.Background()

    // Create a file backend
    backend := file.New(file.Config{Root: "/data"})
    defer backend.Close()

    // Write a file
    w, err := backend.NewWriter(ctx, "hello.txt")
    if err != nil {
        log.Fatal(err)
    }
    w.Write([]byte("Hello, World!"))
    w.Close()

    // Read it back
    r, err := backend.NewReader(ctx, "hello.txt")
    if err != nil {
        log.Fatal(err)
    }
    data, _ := io.ReadAll(r)
    r.Close()

    log.Println(string(data)) // "Hello, World!"
}
```

## With Compression

```go
import (
    "github.com/grokify/omnistorage/backend/file"
    "github.com/grokify/omnistorage/compress/gzip"
)

// Write compressed data
fileWriter, _ := backend.NewWriter(ctx, "data.txt.gz")
gzipWriter, _ := gzip.NewWriter(fileWriter)
gzipWriter.Write([]byte("compressed content"))
gzipWriter.Close()

// Read compressed data
fileReader, _ := backend.NewReader(ctx, "data.txt.gz")
gzipReader, _ := gzip.NewReader(fileReader)
data, _ := io.ReadAll(gzipReader)
gzipReader.Close()
```

## With NDJSON Records

```go
import (
    "github.com/grokify/omnistorage/backend/file"
    "github.com/grokify/omnistorage/format/ndjson"
)

// Write NDJSON records
w, _ := backend.NewWriter(ctx, "records.ndjson")
ndjsonWriter := ndjson.NewWriter(w)
ndjsonWriter.Write([]byte(`{"id":1,"name":"alice"}`))
ndjsonWriter.Write([]byte(`{"id":2,"name":"bob"}`))
ndjsonWriter.Close()

// Read NDJSON records
r, _ := backend.NewReader(ctx, "records.ndjson")
ndjsonReader := ndjson.NewReader(r)
for {
    record, err := ndjsonReader.Read()
    if err == io.EOF {
        break
    }
    log.Println(string(record))
}
ndjsonReader.Close()
```

## Using the Registry

```go
import (
    "github.com/grokify/omnistorage"
    _ "github.com/grokify/omnistorage/backend/file"
    _ "github.com/grokify/omnistorage/backend/s3"
)

// Open backend by name
backend, _ := omnistorage.Open("file", map[string]string{
    "root": "/data",
})
defer backend.Close()

// List registered backends
backends := omnistorage.Backends() // ["file", "memory", "s3"]
```

## Sync Between Backends

```go
import "github.com/grokify/omnistorage/sync"

srcBackend := file.New(file.Config{Root: "/local"})
dstBackend, _ := s3.New(s3.Config{Bucket: "my-bucket"})

// Sync local to S3
result, err := sync.Sync(ctx, srcBackend, dstBackend, "data/", "backup/", sync.Options{
    DeleteExtra: true,  // Delete files in dst not in src
})

fmt.Printf("Copied: %d, Updated: %d, Deleted: %d\n",
    result.Copied, result.Updated, result.Deleted)
```

## Check File Existence

```go
exists, err := backend.Exists(ctx, "hello.txt")
if err != nil {
    log.Fatal(err)
}
if exists {
    log.Println("File exists")
}
```

## List Files

```go
// List all files with prefix
files, err := backend.List(ctx, "logs/")
if err != nil {
    log.Fatal(err)
}
for _, f := range files {
    log.Println(f)
}
```

## Delete Files

```go
err := backend.Delete(ctx, "hello.txt")
if err != nil {
    log.Fatal(err)
}
```

## Extended Backend Features

```go
// Check if backend supports extended operations
if ext, ok := omnistorage.AsExtended(backend); ok {
    // Get file metadata
    info, _ := ext.Stat(ctx, "file.txt")
    fmt.Printf("Size: %d, Modified: %s\n", info.Size(), info.ModTime())

    // Server-side copy (no download/upload)
    if ext.Features().Copy {
        ext.Copy(ctx, "source.txt", "dest.txt")
    }

    // Directory operations
    ext.Mkdir(ctx, "new-folder")
}
```

## Next Steps

- [Concepts](concepts.md) - Understand the architecture
- [Backends](../backends/index.md) - Learn about specific backends
- [Sync Engine](../sync/index.md) - Advanced sync operations
