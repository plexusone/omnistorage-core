package object

import (
	"fmt"
	"sort"
	"sync"
)

var (
	backendsMu sync.RWMutex
	backends   = make(map[string]BackendFactory)
)

// BackendFactory creates a Backend from configuration.
// The config map contains backend-specific configuration keys.
type BackendFactory func(config map[string]string) (Backend, error)

// Register registers a backend factory under the given name.
// It is typically called from init() in backend packages.
//
// Register panics if:
//   - factory is nil
//   - a backend with the same name is already registered
//
// Example:
//
//	func init() {
//	    omnistorage.Register("mybackend", New)
//	}
func Register(name string, factory BackendFactory) {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	if factory == nil {
		panic("omnistorage: Register factory is nil")
	}
	if _, dup := backends[name]; dup {
		panic("omnistorage: Register called twice for backend " + name)
	}
	backends[name] = factory
}

// Open opens a backend by name with the given configuration.
// The config map is passed directly to the backend's factory function.
//
// Open returns ErrUnknownBackend if no backend with the given name is registered.
//
// Example:
//
//	backend, err := omnistorage.Open("s3", map[string]string{
//	    "bucket": "my-bucket",
//	    "region": "us-west-2",
//	})
func Open(name string, config map[string]string) (Backend, error) {
	backendsMu.RLock()
	factory, ok := backends[name]
	backendsMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownBackend, name)
	}
	return factory(config)
}

// Backends returns a sorted list of registered backend names.
func Backends() []string {
	backendsMu.RLock()
	defer backendsMu.RUnlock()

	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsRegistered returns true if a backend with the given name is registered.
func IsRegistered(name string) bool {
	backendsMu.RLock()
	defer backendsMu.RUnlock()
	_, ok := backends[name]
	return ok
}

// Unregister removes a registered backend.
// This is primarily useful for testing.
// Returns true if the backend was registered, false otherwise.
func Unregister(name string) bool {
	backendsMu.Lock()
	defer backendsMu.Unlock()

	if _, ok := backends[name]; ok {
		delete(backends, name)
		return true
	}
	return false
}
