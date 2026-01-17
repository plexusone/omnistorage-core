// Package sftp provides an SFTP backend for omnistorage.
//
// Basic usage with password authentication:
//
//	backend, err := sftp.New(sftp.Config{
//	    Host:     "example.com",
//	    User:     "username",
//	    Password: "password",
//	})
//
// With SSH key authentication:
//
//	backend, err := sftp.New(sftp.Config{
//	    Host:    "example.com",
//	    User:    "username",
//	    KeyFile: "/path/to/id_rsa",
//	})
package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/grokify/omnistorage"
)

func init() {
	omnistorage.Register("sftp", NewFromConfig)
}

// Backend implements omnistorage.ExtendedBackend for SFTP.
type Backend struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	config     Config
	closed     bool
	mu         sync.RWMutex
}

// New creates a new SFTP backend with the given configuration.
func New(cfg Config) (*Backend, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30
	}

	// Build SSH auth methods
	var authMethods []ssh.AuthMethod

	// Password auth
	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	// Key file auth
	if cfg.KeyFile != "" {
		keyAuth, err := keyFileAuth(cfg.KeyFile, cfg.KeyPassphrase)
		if err != nil {
			return nil, fmt.Errorf("sftp: loading key file: %w", err)
		}
		authMethods = append(authMethods, keyAuth)
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("sftp: no authentication method provided (password or key_file required)")
	}

	// Build SSH config.
	// Host key verification is performed using the user's known_hosts file.
	// This prevents man-in-the-middle attacks by ensuring the server's host
	// key matches a trusted entry.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("sftp: could not determine user home directory for known_hosts: %w", err)
	}
	knownHostsPath := path.Join(homeDir, ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("sftp: could not load known_hosts file (%s): %w", knownHostsPath, err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		Timeout:         time.Duration(cfg.Timeout) * time.Second,
		HostKeyCallback: hostKeyCallback,
	}

	// Connect
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("sftp: SSH connection failed: %w", err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		if closeErr := sshClient.Close(); closeErr != nil {
			return nil, fmt.Errorf("sftp: SFTP session failed: %w (also failed to close SSH: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("sftp: SFTP session failed: %w", err)
	}

	return &Backend{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		config:     cfg,
	}, nil
}

// NewFromConfig creates a new SFTP backend from a config map.
// This is used by the omnistorage registry.
func NewFromConfig(configMap map[string]string) (omnistorage.Backend, error) {
	cfg := ConfigFromMap(configMap)
	return New(cfg)
}

// keyFileAuth creates an SSH auth method from a private key file.
func keyFileAuth(keyFile, passphrase string) (ssh.AuthMethod, error) {
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}

	var signer ssh.Signer
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}

// NewWriter creates a writer for the given path.
func (b *Backend) NewWriter(ctx context.Context, p string, opts ...omnistorage.WriterOption) (io.WriteCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fullPath := b.fullPath(p)

	// Ensure parent directory exists
	dir := path.Dir(fullPath)
	if err := b.sftpClient.MkdirAll(dir); err != nil {
		return nil, fmt.Errorf("sftp: creating directory: %w", err)
	}

	// Create or truncate file
	f, err := b.sftpClient.Create(fullPath)
	if err != nil {
		return nil, b.translateError(err, p)
	}

	return f, nil
}

// NewReader creates a reader for the given path.
func (b *Backend) NewReader(ctx context.Context, p string, opts ...omnistorage.ReaderOption) (io.ReadCloser, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fullPath := b.fullPath(p)
	cfg := omnistorage.ApplyReaderOptions(opts...)

	f, err := b.sftpClient.Open(fullPath)
	if err != nil {
		return nil, b.translateError(err, p)
	}

	// Handle offset
	if cfg.Offset > 0 {
		if _, err := f.Seek(cfg.Offset, io.SeekStart); err != nil {
			if closeErr := f.Close(); closeErr != nil {
				return nil, fmt.Errorf("sftp: seeking to offset: %w (also failed to close: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("sftp: seeking to offset: %w", err)
		}
	}

	// Handle limit
	if cfg.Limit > 0 {
		return &limitedReader{f, cfg.Limit}, nil
	}

	return f, nil
}

// limitedReader wraps a reader with a byte limit.
type limitedReader struct {
	r         io.ReadCloser
	remaining int64
}

func (lr *limitedReader) Read(p []byte) (n int, err error) {
	if lr.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > lr.remaining {
		p = p[:lr.remaining]
	}
	n, err = lr.r.Read(p)
	lr.remaining -= int64(n)
	return
}

func (lr *limitedReader) Close() error {
	return lr.r.Close()
}

// Exists checks if a path exists.
func (b *Backend) Exists(ctx context.Context, p string) (bool, error) {
	if err := b.checkClosed(); err != nil {
		return false, err
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	fullPath := b.fullPath(p)
	_, err := b.sftpClient.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, b.translateError(err, p)
	}
	return true, nil
}

// Delete removes a path.
func (b *Backend) Delete(ctx context.Context, p string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	fullPath := b.fullPath(p)
	err := b.sftpClient.Remove(fullPath)
	if err != nil {
		// Delete is idempotent
		if os.IsNotExist(err) {
			return nil
		}
		return b.translateError(err, p)
	}
	return nil
}

// List lists paths with the given prefix.
func (b *Backend) List(ctx context.Context, prefix string) ([]string, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Determine the directory to list
	fullPrefix := b.fullPath(prefix)
	dir := fullPrefix
	namePrefix := ""

	// If prefix is not a directory, use parent dir and filter by name
	info, err := b.sftpClient.Stat(fullPrefix)
	if err != nil || !info.IsDir() {
		dir = path.Dir(fullPrefix)
		namePrefix = path.Base(fullPrefix)
	}

	var paths []string
	err = b.walkDir(ctx, dir, namePrefix, &paths)
	if err != nil {
		return nil, err
	}

	return paths, nil
}

func (b *Backend) walkDir(ctx context.Context, dir, namePrefix string, paths *[]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	entries, err := b.sftpClient.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("sftp: listing directory: %w", err)
	}

	for _, entry := range entries {
		if namePrefix != "" && !strings.HasPrefix(entry.Name(), namePrefix) {
			continue
		}

		entryPath := path.Join(dir, entry.Name())
		relPath := strings.TrimPrefix(entryPath, b.config.Root)
		relPath = strings.TrimPrefix(relPath, "/")

		if entry.IsDir() {
			// Recurse into subdirectories
			if err := b.walkDir(ctx, entryPath, "", paths); err != nil {
				return err
			}
		} else {
			*paths = append(*paths, relPath)
		}
	}

	return nil
}

// Close releases any resources held by the backend.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	var errs []error
	if b.sftpClient != nil {
		if err := b.sftpClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if b.sshClient != nil {
		if err := b.sshClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("sftp: close errors: %v", errs)
	}
	return nil
}

// Stat returns metadata about an object.
func (b *Backend) Stat(ctx context.Context, p string) (omnistorage.ObjectInfo, error) {
	if err := b.checkClosed(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fullPath := b.fullPath(p)
	info, err := b.sftpClient.Stat(fullPath)
	if err != nil {
		return nil, b.translateError(err, p)
	}

	return &omnistorage.BasicObjectInfo{
		ObjectPath:    p,
		ObjectSize:    info.Size(),
		ObjectModTime: info.ModTime(),
		ObjectIsDir:   info.IsDir(),
	}, nil
}

// Mkdir creates a directory.
func (b *Backend) Mkdir(ctx context.Context, p string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	fullPath := b.fullPath(p)
	err := b.sftpClient.MkdirAll(fullPath)
	if err != nil {
		return fmt.Errorf("sftp: creating directory: %w", err)
	}
	return nil
}

// Rmdir removes an empty directory.
func (b *Backend) Rmdir(ctx context.Context, p string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	fullPath := b.fullPath(p)

	// Check if directory is empty
	entries, err := b.sftpClient.ReadDir(fullPath)
	if err != nil {
		return b.translateError(err, p)
	}
	if len(entries) > 0 {
		return fmt.Errorf("sftp: directory not empty: %s", p)
	}

	err = b.sftpClient.RemoveDirectory(fullPath)
	if err != nil {
		return b.translateError(err, p)
	}
	return nil
}

// Copy copies a file.
func (b *Backend) Copy(ctx context.Context, src, dst string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// SFTP doesn't have server-side copy, so we stream through the client
	srcPath := b.fullPath(src)
	dstPath := b.fullPath(dst)

	// Ensure parent directory exists
	if err := b.sftpClient.MkdirAll(path.Dir(dstPath)); err != nil {
		return fmt.Errorf("sftp: creating directory: %w", err)
	}

	srcFile, err := b.sftpClient.Open(srcPath)
	if err != nil {
		return b.translateError(err, src)
	}

	dstFile, err := b.sftpClient.Create(dstPath)
	if err != nil {
		if closeErr := srcFile.Close(); closeErr != nil {
			return fmt.Errorf("sftp: %w (also failed to close source: %v)", b.translateError(err, dst), closeErr)
		}
		return b.translateError(err, dst)
	}

	_, copyErr := io.Copy(dstFile, srcFile)

	// Close both files, collecting any errors
	srcCloseErr := srcFile.Close()
	dstCloseErr := dstFile.Close()

	// Return the first error encountered
	if copyErr != nil {
		return fmt.Errorf("sftp: copying file: %w", copyErr)
	}
	if dstCloseErr != nil {
		return fmt.Errorf("sftp: closing destination file: %w", dstCloseErr)
	}
	if srcCloseErr != nil {
		return fmt.Errorf("sftp: closing source file: %w", srcCloseErr)
	}

	return nil
}

// Move moves a file.
func (b *Backend) Move(ctx context.Context, src, dst string) error {
	if err := b.checkClosed(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	srcPath := b.fullPath(src)
	dstPath := b.fullPath(dst)

	// Ensure parent directory exists
	if err := b.sftpClient.MkdirAll(path.Dir(dstPath)); err != nil {
		return fmt.Errorf("sftp: creating directory: %w", err)
	}

	// Try rename first (same filesystem)
	err := b.sftpClient.Rename(srcPath, dstPath)
	if err != nil {
		// Fall back to copy+delete
		if copyErr := b.Copy(ctx, src, dst); copyErr != nil {
			return copyErr
		}
		return b.Delete(ctx, src)
	}

	return nil
}

// Features returns the capabilities of the SFTP backend.
func (b *Backend) Features() omnistorage.Features {
	return omnistorage.Features{
		Copy:       true, // Implemented as streaming copy
		Move:       true, // Uses rename or copy+delete
		Mkdir:      true,
		Rmdir:      true,
		Stat:       true,
		RangeRead:  true,
		ListPrefix: true,
	}
}

// fullPath returns the full remote path.
func (b *Backend) fullPath(p string) string {
	if b.config.Root == "" {
		return p
	}
	return path.Join(b.config.Root, p)
}

// checkClosed returns an error if the backend is closed.
func (b *Backend) checkClosed() error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return omnistorage.ErrBackendClosed
	}
	return nil
}

// translateError converts SFTP errors to omnistorage errors.
// The path parameter provides context for error messages.
func (b *Backend) translateError(err error, p string) error {
	if err == nil {
		return nil
	}

	if os.IsNotExist(err) {
		return omnistorage.ErrNotFound
	}

	if os.IsPermission(err) {
		return omnistorage.ErrPermissionDenied
	}

	// Check for path error
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		if os.IsNotExist(pathErr.Err) {
			return omnistorage.ErrNotFound
		}
		if os.IsPermission(pathErr.Err) {
			return omnistorage.ErrPermissionDenied
		}
	}

	// Check for net errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("sftp: network error for %q: %w", p, err)
	}

	return fmt.Errorf("sftp: error for %q: %w", p, err)
}

// Ensure Backend implements omnistorage.ExtendedBackend
var _ omnistorage.ExtendedBackend = (*Backend)(nil)
