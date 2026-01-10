# Installation

## Requirements

- Go 1.21 or later

## Install

```bash
go get github.com/grokify/omnistorage
```

## Backend-Specific Dependencies

The core package has minimal dependencies. Backend-specific packages bring in their own dependencies:

### S3 Backend

```bash
go get github.com/grokify/omnistorage/backend/s3
```

This brings in the AWS SDK v2.

### Google Drive Backend

Google backends are in a separate repository to keep the core lightweight:

```bash
go get github.com/grokify/omnistorage-google
```

### Zstandard Compression

```bash
go get github.com/grokify/omnistorage/compress/zstd
```

This brings in `github.com/klauspost/compress`.

## Import Patterns

### Direct Backend Usage

```go
import (
    "github.com/grokify/omnistorage/backend/file"
    "github.com/grokify/omnistorage/backend/s3"
)

// Use backends directly
fileBackend := file.New(file.Config{Root: "/data"})
s3Backend, _ := s3.New(s3.Config{Bucket: "my-bucket"})
```

### Registry Pattern

```go
import (
    "github.com/grokify/omnistorage"

    // Side-effect imports register backends
    _ "github.com/grokify/omnistorage/backend/file"
    _ "github.com/grokify/omnistorage/backend/s3"
)

// Open by name from configuration
backend, _ := omnistorage.Open("s3", map[string]string{
    "bucket": "my-bucket",
    "region": "us-east-1",
})
```

## Verify Installation

```go
package main

import (
    "fmt"
    "github.com/grokify/omnistorage"
    _ "github.com/grokify/omnistorage/backend/file"
    _ "github.com/grokify/omnistorage/backend/memory"
)

func main() {
    backends := omnistorage.Backends()
    fmt.Println("Registered backends:", backends)
    // Output: Registered backends: [file memory]
}
```

## Next Steps

- [Quick Start](quick-start.md) - Learn the basics
- [Concepts](concepts.md) - Understand the architecture
