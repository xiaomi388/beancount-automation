package persistence

import "github.com/xiaomi388/beancount-automation/pkg/types"

// JSONStore implements Store using a JSON file (the original owners.yaml format).
type JSONStore struct {
	path string
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

func (s *JSONStore) LoadOwners() ([]types.Owner, error) {
	return LoadOwners(s.path)
}

func (s *JSONStore) DumpOwners(owners []types.Owner) error {
	return DumpOwners(s.path, owners)
}

func (s *JSONStore) Close() error {
	return nil
}
