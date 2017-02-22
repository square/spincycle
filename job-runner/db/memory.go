// Copyright 2017, Square, Inc.

package db

import (
	"sync"
)

// Memory is a Driver that stores things in local memory in the following data
// structure: map[string]map[string]interface{}.
type Memory struct {
	data map[string]map[string]interface{}
	// --
	*sync.RWMutex // guards data
}

// NewMemory returns a new LocalCache.
func NewMemory() *Memory {
	return &Memory{
		data:    make(map[string]map[string]interface{}),
		RWMutex: &sync.RWMutex{},
	}
}

func (m *Memory) Add(db, key string, val interface{}) error {
	m.Lock()
	defer m.Unlock()
	if _, exists := m.data[db]; !exists {
		m.data[db] = make(map[string]interface{})
	}

	// If the key already exists, return an error.
	if _, exists := m.data[db][key]; exists {
		return ErrConflict
	}

	m.data[db][key] = val
	return nil
}

func (m *Memory) Set(db, key string, val interface{}) error {
	m.Lock()
	defer m.Unlock()
	if _, exists := m.data[db]; !exists {
		m.data[db] = make(map[string]interface{})
	}

	m.data[db][key] = val
	return nil
}

func (m *Memory) Get(db, key string) (interface{}, error) {
	m.RLock()
	defer m.RUnlock()
	if _, exists := m.data[db]; !exists {
		return nil, ErrDbNotFound
	}

	i, ok := m.data[db][key]
	// If the key is not in the cache, return an error.
	if !ok {
		return nil, ErrKeyNotFound
	}
	return i, nil
}

func (m *Memory) GetAll(db string) (map[string]interface{}, error) {
	m.RLock()
	defer m.RUnlock()
	if _, exists := m.data[db]; !exists {
		return nil, nil
	}

	// Return a copy of the map to avoid subjecting clients to potential
	// race conditions.
	resp := make(map[string]interface{})
	for k, v := range m.data[db] {
		resp[k] = v
	}

	return resp, nil
}

func (m *Memory) Delete(db, key string) error {
	m.Lock()
	defer m.Unlock()
	if _, exists := m.data[db]; !exists {
		return nil
	}

	delete(m.data[db], key)
	return nil
}
