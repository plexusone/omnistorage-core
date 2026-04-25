# Installation

## Requirements

- Go 1.21 or later

## Install

```bash
go get github.com/plexusone/omnistorage-core
```

## Backend-Specific Dependencies

The core package has minimal dependencies. Cloud backends are in separate packages:

### Core Backends (Included)

These backends have minimal or no external dependencies:

- `object/backend/file` - Local filesystem (no external deps)
- `object/backend/memory` - In-memory storage (no external deps)
- `object/backend/channel` - Go channels (no external deps)
- `object/backend/sftp` - SSH file transfer (uses `pkg/sftp`)

### Cloud Backends (Separate Packages)

Cloud backends with vendor SDKs are in separate repositories:

```bash
# AWS S3 backend
go get github.com/plexusone/omni-aws/omnistorage/s3

# Google Cloud Storage
go get github.com/plexusone/omni-google/omnistorage/gcs

# Google Drive
go get github.com/plexusone/omni-google/omnistorage/drive
```

### Compression

Zstandard compression is included in the core:

```go
import "github.com/plexusone/omnistorage-core/object/compress/zstd"
```

This uses `github.com/klauspost/compress`.

## Import Patterns

### Direct Backend Usage

```go
import (
    "github.com/plexusone/omnistorage-core/object/backend/file"
    "github.com/plexusone/omnistorage-core/object/backend/memory"
)

// Use backends directly
fileBackend := file.New(file.Config{Root: "/data"})
memBackend := memory.New()
```

### Registry Pattern

```go
import (
    "github.com/plexusone/omnistorage-core/object"

    // Side-effect imports register backends
    _ "github.com/plexusone/omnistorage-core/object/backend/file"
    _ "github.com/plexusone/omnistorage-core/object/backend/memory"
)

// Open by name from configuration
backend, _ := object.Open("file", map[string]string{
    "root": "/data",
})
```

## Verify Installation

```go
package main

import (
    "fmt"
    "github.com/plexusone/omnistorage-core/object"
    _ "github.com/plexusone/omnistorage-core/object/backend/file"
    _ "github.com/plexusone/omnistorage-core/object/backend/memory"
)

func main() {
    backends := object.Backends()
    fmt.Println("Registered backends:", backends)
    // Output: Registered backends: [file memory]
}
```

## Next Steps

- [Quick Start](quick-start.md) - Learn the basics
- [Concepts](concepts.md) - Understand the architecture
