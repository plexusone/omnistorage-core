package sftp

import (
	"errors"
	"os"
	"strconv"
)

// Errors specific to the SFTP backend.
var (
	ErrHostRequired = errors.New("sftp: host is required")
	ErrUserRequired = errors.New("sftp: user is required")
)

// Config holds configuration for the SFTP backend.
type Config struct {
	// Host is the SFTP server hostname or IP address (required).
	Host string

	// Port is the SSH port. Default: 22.
	Port int

	// User is the SSH username (required).
	User string

	// Password is the SSH password.
	// Either Password or KeyFile must be provided.
	Password string

	// KeyFile is the path to an SSH private key file.
	// Either Password or KeyFile must be provided.
	KeyFile string

	// KeyPassphrase is the passphrase for encrypted private keys.
	KeyPassphrase string

	// Root is the base directory on the remote server.
	// All paths are relative to this directory.
	Root string

	// KnownHostsFile is the path to the known_hosts file.
	// If empty, host key verification is disabled (insecure).
	KnownHostsFile string

	// Timeout is the connection timeout in seconds.
	// Default: 30.
	Timeout int

	// Concurrency is the maximum number of concurrent operations.
	// Default: 5.
	Concurrency int
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Port:        22,
		Timeout:     30,
		Concurrency: 5,
	}
}

// ConfigFromEnv creates a Config from environment variables.
// Environment variables:
//   - OMNISTORAGE_SFTP_HOST: server hostname
//   - OMNISTORAGE_SFTP_PORT: SSH port (default: 22)
//   - OMNISTORAGE_SFTP_USER: username
//   - OMNISTORAGE_SFTP_PASSWORD: password
//   - OMNISTORAGE_SFTP_KEY_FILE: path to private key
//   - OMNISTORAGE_SFTP_KEY_PASSPHRASE: passphrase for encrypted key
//   - OMNISTORAGE_SFTP_ROOT: base directory
//   - OMNISTORAGE_SFTP_KNOWN_HOSTS: path to known_hosts file
//   - OMNISTORAGE_SFTP_TIMEOUT: connection timeout in seconds
func ConfigFromEnv() Config {
	config := DefaultConfig()

	if v := os.Getenv("OMNISTORAGE_SFTP_HOST"); v != "" {
		config.Host = v
	}
	if v := os.Getenv("OMNISTORAGE_SFTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			config.Port = port
		}
	}
	if v := os.Getenv("OMNISTORAGE_SFTP_USER"); v != "" {
		config.User = v
	}
	if v := os.Getenv("OMNISTORAGE_SFTP_PASSWORD"); v != "" {
		config.Password = v
	}
	if v := os.Getenv("OMNISTORAGE_SFTP_KEY_FILE"); v != "" {
		config.KeyFile = v
	}
	if v := os.Getenv("OMNISTORAGE_SFTP_KEY_PASSPHRASE"); v != "" {
		config.KeyPassphrase = v
	}
	if v := os.Getenv("OMNISTORAGE_SFTP_ROOT"); v != "" {
		config.Root = v
	}
	if v := os.Getenv("OMNISTORAGE_SFTP_KNOWN_HOSTS"); v != "" {
		config.KnownHostsFile = v
	}
	if v := os.Getenv("OMNISTORAGE_SFTP_TIMEOUT"); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil && timeout > 0 {
			config.Timeout = timeout
		}
	}

	return config
}

// ConfigFromMap creates a Config from a string map.
// Supported keys:
//   - host: server hostname (required)
//   - port: SSH port (default: 22)
//   - user: username (required)
//   - pass or password: password
//   - key_file: path to private key
//   - key_passphrase: passphrase for encrypted key
//   - root: base directory
//   - known_hosts: path to known_hosts file
//   - timeout: connection timeout in seconds
//   - concurrency: maximum concurrent operations
func ConfigFromMap(m map[string]string) Config {
	config := DefaultConfig()

	if v, ok := m["host"]; ok {
		config.Host = v
	}
	if v, ok := m["port"]; ok {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			config.Port = port
		}
	}
	if v, ok := m["user"]; ok {
		config.User = v
	}
	if v, ok := m["pass"]; ok {
		config.Password = v
	}
	if v, ok := m["password"]; ok {
		config.Password = v
	}
	if v, ok := m["key_file"]; ok {
		config.KeyFile = v
	}
	if v, ok := m["key_passphrase"]; ok {
		config.KeyPassphrase = v
	}
	if v, ok := m["root"]; ok {
		config.Root = v
	}
	if v, ok := m["known_hosts"]; ok {
		config.KnownHostsFile = v
	}
	if v, ok := m["timeout"]; ok {
		if timeout, err := strconv.Atoi(v); err == nil && timeout > 0 {
			config.Timeout = timeout
		}
	}
	if v, ok := m["concurrency"]; ok {
		if c, err := strconv.Atoi(v); err == nil && c > 0 {
			config.Concurrency = c
		}
	}

	return config
}

// Validate checks if the configuration is valid.
func (c Config) Validate() error {
	if c.Host == "" {
		return ErrHostRequired
	}
	if c.User == "" {
		return ErrUserRequired
	}
	return nil
}
