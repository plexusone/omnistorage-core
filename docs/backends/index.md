# Backends Overview

OmniStorage supports multiple storage backends through a unified interface. Each backend implements the core `Backend` interface, and many also implement `ExtendedBackend` for additional capabilities.

## Core Backends

These backends are included in omnistorage-core with minimal dependencies:

| Backend | Package | Extended | Description |
|---------|---------|----------|-------------|
| [File](file.md) | `object/backend/file` | Yes | Local filesystem |
| [Memory](memory.md) | `object/backend/memory` | Yes | In-memory storage |
| [SFTP](sftp.md) | `object/backend/sftp` | Yes | SSH file transfer |
| [Channel](channel.md) | `object/backend/channel` | No | Go channel for streaming |

## External Backends (Separate Packages)

Cloud backends with vendor SDKs are in separate repositories to keep the core lightweight:

| Backend | Repository | Description |
|---------|------------|-------------|
| S3 | [omni-aws](https://github.com/plexusone/omni-aws) | AWS S3, R2, MinIO, Wasabi |
| DynamoDB | [omni-aws](https://github.com/plexusone/omni-aws) | AWS DynamoDB (KVS) |
| Google Cloud Storage | [omni-google](https://github.com/plexusone/omni-google) | GCS |
| Google Drive | [omni-google](https://github.com/plexusone/omni-google) | Google Drive API |
| GitHub | [omni-github](https://github.com/plexusone/omni-github) | GitHub API storage |

## Backend Capabilities

Each backend has different capabilities:

| Feature | File | Memory | SFTP | Channel |
|---------|------|--------|------|---------|
| Read/Write | Yes | Yes | Yes | Yes |
| Stat | Yes | Yes | Yes | No |
| Copy | Yes | Yes | Yes | No |
| Move | Yes | Yes | Yes | No |
| Mkdir | Yes | Yes | Yes | No |
| Range Read | Yes | No | Yes | No |
| Streaming | Yes | Yes | Yes | Yes |

## Using the Registry

Backends register themselves automatically when imported:

```go
import (
    "github.com/plexusone/omnistorage-core/object"

    // Side-effect imports register backends
    _ "github.com/plexusone/omnistorage-core/object/backend/file"
    _ "github.com/plexusone/omnistorage-core/object/backend/memory"
)

// Open by name
backend, err := object.Open("file", map[string]string{
    "root": "/data",
})
```

## Configuration-Driven Selection

Select backends at runtime from configuration:

```go
backendType := os.Getenv("STORAGE_BACKEND")
config := map[string]string{
    "root": os.Getenv("STORAGE_ROOT"),
}

backend, err := object.Open(backendType, config)
```

## Implementing a Custom Backend

See [Custom Backend Guide](../guides/custom-backend.md) for how to implement your own backend.
