package storage

import (
	"fmt"
	"sync"
)

// Manager provides named access to multiple Bucket instances.
type Manager struct {
	mu      sync.RWMutex
	buckets map[string]Bucket
	primary string
}

// NewManager creates an empty Manager.
func NewManager() *Manager {
	return &Manager{
		buckets: make(map[string]Bucket),
	}
}

// Register adds a named bucket to the manager.
func (m *Manager) Register(name string, bucket Bucket) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets[name] = bucket
	if m.primary == "" {
		m.primary = name
	}
}

// SetPrimary sets the default bucket name.
func (m *Manager) SetPrimary(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.primary = name
}

// Get returns the bucket registered under the given name.
func (m *Manager) Get(name string) (Bucket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.buckets[name]
	if !ok {
		return nil, fmt.Errorf("storage: bucket %q not registered", name)
	}
	return b, nil
}

// Primary returns the default bucket.
func (m *Manager) Primary() Bucket {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.buckets[m.primary]
}

// Names returns all registered bucket names.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.buckets))
	for name := range m.buckets {
		names = append(names, name)
	}
	return names
}
