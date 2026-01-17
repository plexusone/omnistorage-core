# OmniStorage

**Unified storage abstraction layer for Go**

[![Go Reference](https://pkg.go.dev/badge/github.com/grokify/omnistorage.svg)](https://pkg.go.dev/github.com/grokify/omnistorage)
[![Go Report Card](https://goreportcard.com/badge/github.com/grokify/omnistorage)](https://goreportcard.com/report/github.com/grokify/omnistorage)

OmniStorage provides a single interface for reading and writing to various storage backends with composable layers for compression and record framing. Inspired by [rclone](https://rclone.org/).

## Features

- **Single interface** for multiple storage backends (local files, S3, cloud drives, etc.)
- **Composable layers** for compression (gzip, zstd) and formatting (NDJSON)
- **Sync engine** for file synchronization between backends (like `rclone sync`)
- **Extended interface** for metadata, server-side copy/move, and capability discovery
- **Multi-writer** for fan-out to multiple backends simultaneously
- **Backend registration** allowing external packages to implement backends

## Quick Example

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
    w, _ := backend.NewWriter(ctx, "hello.txt")
    w.Write([]byte("Hello, World!"))
    w.Close()

    // Read it back
    r, _ := backend.NewReader(ctx, "hello.txt")
    data, _ := io.ReadAll(r)
    r.Close()

    log.Println(string(data)) // "Hello, World!"
}
```

## Why OmniStorage?

| Challenge | OmniStorage Solution |
|-----------|---------------------|
| Different APIs for each storage provider | Single `Backend` interface |
| Provider-specific code | Backend abstraction with registration |
| Duplicated compression/formatting logic | Composable layers |
| Hard to switch providers | Configuration-driven backend selection |
| No sync between backends | rclone-inspired sync engine |

## Supported Backends

| Backend | Package | Status |
|---------|---------|--------|
| Local Filesystem | `backend/file` | Stable |
| In-Memory | `backend/memory` | Stable |
| S3-Compatible | `backend/s3` | Stable |
| Go Channel | `backend/channel` | Stable |
| Google Drive | [omnistorage-google](https://github.com/grokify/omnistorage-google) | Stable |
| Google Cloud Storage | Planned | - |
| Azure Blob Storage | Planned | - |

## Getting Started

<div class="grid cards" markdown>

- :material-download: **[Installation](getting-started/installation.md)**

    Install omnistorage and get started in minutes

- :material-rocket-launch: **[Quick Start](getting-started/quick-start.md)**

    Learn the basics with hands-on examples

- :material-book-open: **[Concepts](getting-started/concepts.md)**

    Understand the architecture and design

</div>

## Documentation Sections

- **[Backends](backends/index.md)** - Storage backend documentation
- **[Sync Engine](sync/index.md)** - File synchronization (rclone-like)
- **[Guides](guides/compression.md)** - How-to guides and tutorials
- **[Reference](reference/interfaces.md)** - API reference and interfaces

## Related Projects

- [omnistorage-google](https://github.com/grokify/omnistorage-google) - Google Drive and GCS backends
- [rclone](https://github.com/rclone/rclone) - Inspiration for backend coverage and sync capabilities
- [go-cloud](https://github.com/google/go-cloud) - Google's portable cloud APIs
- [afero](https://github.com/spf13/afero) - Filesystem abstraction

## License

MIT License - see [LICENSE](https://github.com/grokify/omnistorage/blob/master/LICENSE) for details.
