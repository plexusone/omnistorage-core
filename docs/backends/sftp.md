# SFTP Backend

The SFTP backend provides access to remote servers via SSH File Transfer Protocol (SFTP). It supports both password and SSH key authentication.

## Installation

```go
import "github.com/plexusone/omnistorage-core/object/object/backend/sftp"
```

## Usage

### Password Authentication

```go
backend, err := sftp.New(sftp.Config{
    Host:     "example.com",
    User:     "username",
    Password: "password",
})
defer backend.Close()
```

### SSH Key Authentication

```go
backend, err := sftp.New(sftp.Config{
    Host:    "example.com",
    User:    "username",
    KeyFile: "/path/to/id_rsa",
})
```

### With Encrypted Key

```go
backend, err := sftp.New(sftp.Config{
    Host:          "example.com",
    User:          "username",
    KeyFile:       "/path/to/id_rsa",
    KeyPassphrase: "keypassword",
})
```

### From Environment Variables

```go
backend, err := sftp.New(sftp.ConfigFromEnv())
```

Environment variables:

- `OMNISTORAGE_SFTP_HOST` - Server hostname
- `OMNISTORAGE_SFTP_PORT` - SSH port (default: 22)
- `OMNISTORAGE_SFTP_USER` - Username
- `OMNISTORAGE_SFTP_PASSWORD` - Password
- `OMNISTORAGE_SFTP_KEY_FILE` - Path to private key
- `OMNISTORAGE_SFTP_KEY_PASSPHRASE` - Key passphrase
- `OMNISTORAGE_SFTP_ROOT` - Base directory
- `OMNISTORAGE_SFTP_KNOWN_HOSTS` - Path to known_hosts file
- `OMNISTORAGE_SFTP_TIMEOUT` - Connection timeout in seconds

### Using the Registry

```go
import (
    "github.com/plexusone/omnistorage-core/object"
    _ "github.com/plexusone/omnistorage-core/object/object/backend/sftp"
)

backend, err := object.Open("sftp", map[string]string{
    "host":     "example.com",
    "user":     "username",
    "password": "password",
    "root":     "/data",
})
```

## Configuration

### Config Struct

```go
type Config struct {
    Host           string // Server hostname (required)
    Port           int    // SSH port (default: 22)
    User           string // Username (required)
    Password       string // Password auth
    KeyFile        string // Path to private key
    KeyPassphrase  string // Passphrase for encrypted keys
    Root           string // Base directory for operations
    KnownHostsFile string // Path to known_hosts file
    Timeout        int    // Connection timeout in seconds (default: 30)
    Concurrency    int    // Max concurrent operations (default: 5)
}
```

### Registry Config

| Key | Description | Required |
|-----|-------------|----------|
| `host` | Server hostname | Yes |
| `port` | SSH port | No (default: 22) |
| `user` | Username | Yes |
| `password` | Password | No* |
| `key_file` | Path to private key | No* |
| `key_passphrase` | Key passphrase | No |
| `root` | Base directory | No |
| `known_hosts` | Path to known_hosts file | No |
| `timeout` | Timeout in seconds | No |

\* Either `password` or `key_file` is required.

## Features

The SFTP backend implements `ExtendedBackend`:

| Feature | Supported | Notes |
|---------|-----------|-------|
| Stat | Yes | Returns file metadata |
| Copy | Yes | Client-side streaming copy |
| Move | Yes | Uses rename or copy+delete |
| Mkdir | Yes | Creates directories recursively |
| Rmdir | Yes | Removes empty directories |
| Range Read | Yes | Offset and limit supported |

## Operations

### Write

```go
w, err := backend.NewWriter(ctx, "data/file.json")
if err != nil {
    return err
}
w.Write([]byte(`{"key": "value"}`))
w.Close()
```

### Read

```go
r, err := backend.NewReader(ctx, "data/file.json")
if err != nil {
    return err
}
defer r.Close()
data, _ := io.ReadAll(r)
```

### Range Read

```go
r, err := backend.NewReader(ctx, "large-file.bin",
    object.WithOffset(1000),
    object.WithLimit(500))
```

### List

```go
files, err := backend.List(ctx, "data/")
for _, f := range files {
    fmt.Println(f)
}
```

### Extended Operations

```go
ext := backend.(*sftp.Backend)

// Get file metadata
info, _ := ext.Stat(ctx, "file.txt")
fmt.Printf("Size: %d bytes\n", info.Size())
fmt.Printf("Modified: %s\n", info.ModTime())

// Create directory
ext.Mkdir(ctx, "new-folder")

// Copy file
ext.Copy(ctx, "source.txt", "dest.txt")

// Move file (uses rename if on same filesystem)
ext.Move(ctx, "old.txt", "new.txt")
```

## Authentication

### Password vs Key Authentication

Password authentication is simpler but less secure. SSH key authentication is recommended for production:

```go
// Production: Use SSH key
backend, _ := sftp.New(sftp.Config{
    Host:    "prod.example.com",
    User:    "deploy",
    KeyFile: "/home/app/.ssh/id_ed25519",
})
```

### Host Key Verification

By default, host key verification is disabled for development convenience. For production, specify a known_hosts file:

```go
backend, _ := sftp.New(sftp.Config{
    Host:           "prod.example.com",
    User:           "deploy",
    KeyFile:        "/home/app/.ssh/id_ed25519",
    KnownHostsFile: "/home/app/.ssh/known_hosts",
})
```

## Error Handling

```go
r, err := backend.NewReader(ctx, "missing.txt")
if errors.Is(err, object.ErrNotFound) {
    log.Println("File not found")
}

if errors.Is(err, object.ErrPermissionDenied) {
    log.Println("Permission denied")
}
```

## Best Practices

1. **Use SSH keys** - More secure than passwords
2. **Set a root directory** - Avoid path traversal issues
3. **Enable host key verification** - Required for production
4. **Handle connection errors** - Network issues are common
5. **Close the backend** - Releases SSH connection
