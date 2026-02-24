package persistence

import (
	"fmt"

	"github.com/xiaomi388/beancount-automation/pkg/types"
)

// Store abstracts owner data persistence.
type Store interface {
	LoadOwners() ([]types.Owner, error)
	DumpOwners(owners []types.Owner) error
	Close() error
}

// NewStoreWithBackend creates a Store for the given backend and optional path.
func NewStoreWithBackend(backend, path string) (Store, error) {
	return NewStore(types.StorageConfig{Backend: backend, Path: path})
}

// NewStore creates a Store based on the storage configuration.
func NewStore(cfg types.StorageConfig) (Store, error) {
	backend := cfg.Backend
	if backend == "" {
		backend = "json" // default to json for backward compatibility
	}

	switch backend {
	case "json":
		path := cfg.Path
		if path == "" {
			path = DefaultOwnerPath
		}
		return NewJSONStore(path), nil
	case "sqlite":
		path := cfg.Path
		if path == "" {
			path = DefaultSQLitePath
		}
		return NewSQLiteStore(path)
	default:
		return nil, fmt.Errorf("unknown storage backend: %s", backend)
	}
}
