# Backends Overview

Omnistorage supports multiple storage backends through a unified interface. Each backend implements the core `Backend` interface, and many also implement `ExtendedBackend` for additional capabilities.

## Available Backends

| Backend | Package | Extended | Description |
|---------|---------|----------|-------------|
| [File](file.md) | `backend/file` | Yes | Local filesystem |
| [Memory](memory.md) | `backend/memory` | Yes | In-memory storage |
| [S3](s3.md) | `backend/s3` | Yes | S3-compatible storage |
| [SFTP](sftp.md) | `backend/sftp` | Yes | SSH file transfer |
| [Channel](channel.md) | `backend/channel` | No | Go channel for streaming |

## External Backends

Some backends are in separate repositories to minimize dependencies:

| Backend | Repository | Description |
|---------|------------|-------------|
| Google Drive | [omnistorage-google](https://github.com/grokify/omnistorage-google) | Google Drive API |
| Google Cloud Storage | [omnistorage-google](https://github.com/grokify/omnistorage-google) | GCS (planned) |

## Backend Capabilities

Each backend has different capabilities:

| Feature | File | Memory | S3 | SFTP | Channel |
|---------|------|--------|-----|------|---------|
| Read/Write | Yes | Yes | Yes | Yes | Yes |
| Stat | Yes | Yes | Yes | Yes | No |
| Copy | Yes | Yes | Yes | Yes | No |
| Move | Yes | Yes | Yes | Yes | No |
| Mkdir | Yes | Yes | Yes | Yes | No |
| Range Read | Yes | No | Yes | Yes | No |
| Streaming | Yes | Yes | Yes | Yes | Yes |

## Using the Registry

Backends register themselves automatically when imported:

```go
import (
    "github.com/grokify/omnistorage"

    // Side-effect imports register backends
    _ "github.com/grokify/omnistorage/backend/file"
    _ "github.com/grokify/omnistorage/backend/s3"
)

// Open by name
backend, err := omnistorage.Open("file", map[string]string{
    "root": "/data",
})
```

## Configuration-Driven Selection

Select backends at runtime from configuration:

```go
backendType := os.Getenv("STORAGE_BACKEND")
config := map[string]string{
    "root":     os.Getenv("STORAGE_ROOT"),
    "bucket":   os.Getenv("STORAGE_BUCKET"),
    "region":   os.Getenv("STORAGE_REGION"),
    "endpoint": os.Getenv("STORAGE_ENDPOINT"),
}

backend, err := omnistorage.Open(backendType, config)
```

## Implementing a Custom Backend

See [Custom Backend Guide](../guides/custom-backend.md) for how to implement your own backend.
