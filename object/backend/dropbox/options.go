package dropbox

import (
	"errors"
	"os"
)

// Errors specific to the Dropbox backend.
var (
	ErrTokenRequired = errors.New("dropbox: access token is required")
)

// Config holds configuration for the Dropbox backend.
type Config struct {
	// AccessToken is the OAuth2 access token (required).
	AccessToken string

	// RefreshToken is the OAuth2 refresh token for long-lived sessions.
	RefreshToken string

	// AppKey is the Dropbox app key (for token refresh).
	AppKey string

	// AppSecret is the Dropbox app secret (for token refresh).
	AppSecret string

	// Root is the base path on Dropbox.
	// All paths are relative to this directory.
	// Use "" for the root of the app folder or full Dropbox.
	Root string

	// ChunkSize is the size in bytes for chunked uploads.
	// Default: 150 MB (Dropbox recommends this for stability).
	ChunkSize int64

	// Concurrency is the maximum number of concurrent operations.
	// Default: 4.
	Concurrency int
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		ChunkSize:   150 * 1024 * 1024, // 150 MB
		Concurrency: 4,
	}
}

// ConfigFromEnv creates a Config from environment variables.
// Environment variables:
//   - OMNISTORAGE_DROPBOX_ACCESS_TOKEN: OAuth2 access token
//   - OMNISTORAGE_DROPBOX_REFRESH_TOKEN: OAuth2 refresh token
//   - OMNISTORAGE_DROPBOX_APP_KEY: Dropbox app key
//   - OMNISTORAGE_DROPBOX_APP_SECRET: Dropbox app secret
//   - OMNISTORAGE_DROPBOX_ROOT: Base path
func ConfigFromEnv() Config {
	config := DefaultConfig()

	if v := os.Getenv("OMNISTORAGE_DROPBOX_ACCESS_TOKEN"); v != "" {
		config.AccessToken = v
	}
	if v := os.Getenv("OMNISTORAGE_DROPBOX_REFRESH_TOKEN"); v != "" {
		config.RefreshToken = v
	}
	if v := os.Getenv("OMNISTORAGE_DROPBOX_APP_KEY"); v != "" {
		config.AppKey = v
	}
	if v := os.Getenv("OMNISTORAGE_DROPBOX_APP_SECRET"); v != "" {
		config.AppSecret = v
	}
	if v := os.Getenv("OMNISTORAGE_DROPBOX_ROOT"); v != "" {
		config.Root = v
	}

	return config
}

// ConfigFromMap creates a Config from a string map.
// Supported keys:
//   - access_token: OAuth2 access token (required)
//   - refresh_token: OAuth2 refresh token
//   - app_key: Dropbox app key
//   - app_secret: Dropbox app secret
//   - root: Base path
func ConfigFromMap(m map[string]string) Config {
	config := DefaultConfig()

	if v, ok := m["access_token"]; ok {
		config.AccessToken = v
	}
	if v, ok := m["refresh_token"]; ok {
		config.RefreshToken = v
	}
	if v, ok := m["app_key"]; ok {
		config.AppKey = v
	}
	if v, ok := m["app_secret"]; ok {
		config.AppSecret = v
	}
	if v, ok := m["root"]; ok {
		config.Root = v
	}

	return config
}

// Validate checks if the configuration is valid.
func (c Config) Validate() error {
	if c.AccessToken == "" {
		return ErrTokenRequired
	}
	return nil
}
