# File Backend

The file backend provides local filesystem storage.

## Installation

```go
import "github.com/grokify/omnistorage/backend/file"
```

## Usage

### Basic Usage

```go
backend := file.New(file.Config{
    Root: "/data",  // Base directory for all operations
})
defer backend.Close()

// Write
w, _ := backend.NewWriter(ctx, "logs/app.log")
w.Write([]byte("log entry"))
w.Close()

// Read
r, _ := backend.NewReader(ctx, "logs/app.log")
data, _ := io.ReadAll(r)
r.Close()
```

### Using the Registry

```go
import (
    "github.com/grokify/omnistorage"
    _ "github.com/grokify/omnistorage/backend/file"
)

backend, _ := omnistorage.Open("file", map[string]string{
    "root": "/data",
})
```

## Configuration

### Config Struct

```go
type Config struct {
    Root string // Base directory (required)
}
```

### Registry Config

| Key | Description | Required |
|-----|-------------|----------|
| `root` | Base directory path | Yes |

## Features

The file backend implements `ExtendedBackend`:

| Feature | Supported | Notes |
|---------|-----------|-------|
| Stat | Yes | Full file metadata |
| Copy | Yes | Uses `os.Link` or copy |
| Move | Yes | Uses `os.Rename` |
| Mkdir | Yes | Creates directories |
| Rmdir | Yes | Removes empty directories |

## Extended Operations

```go
ext := backend.(*file.Backend)

// Get file metadata
info, _ := ext.Stat(ctx, "file.txt")
fmt.Printf("Size: %d bytes\n", info.Size())
fmt.Printf("Modified: %s\n", info.ModTime())

// Server-side operations
ext.Copy(ctx, "src.txt", "dst.txt")
ext.Move(ctx, "old.txt", "new.txt")

// Directory operations
ext.Mkdir(ctx, "new-folder")
ext.Rmdir(ctx, "empty-folder")
```

## Path Handling

- All paths are relative to the `Root` directory
- Parent directories are created automatically on write
- Path separators are normalized for the OS

```go
backend := file.New(file.Config{Root: "/data"})

// This writes to /data/logs/2024/01/app.log
w, _ := backend.NewWriter(ctx, "logs/2024/01/app.log")
```

## Error Handling

```go
r, err := backend.NewReader(ctx, "missing.txt")
if errors.Is(err, omnistorage.ErrNotFound) {
    log.Println("File not found")
}
```

## Permissions

- New files are created with mode `0644`
- New directories are created with mode `0755`

## Best Practices

1. **Use absolute paths for Root** - Relative paths depend on working directory
2. **Close writers promptly** - Data is flushed on close
3. **Handle ErrNotFound** - Check for missing files before reading
