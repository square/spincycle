// Copyright 2017, Square, Inc.

package kv

import (
	"errors"
	"sync"
)

var (
	ErrKeyNotFound = errors.New("key not found")
	ErrConflict    = errors.New("key already exists in kv store")
)

// A Store is a key-value store that saves things in memory. Keys must be strings,
// but the values can be any interface.
type Store interface {
	// Add adds an interface to the store. It takes a string argument (the
	// key), and an interface argument (the value). If the key already exists,
	// it will return an error.
	Add(string, interface{}) error

	// Set adds or updates an interface in the store. It takes a string
	// argument (the key), and an interface argument (the value).
	Set(string, interface{})

	// Get looks up an interface from the store, using the supplied key. It
	// returns an error if the database+key does not exist.
	Get(string) (interface{}, error)

	// GetAll returns a map of all keys and interfaces in a store. It returns
	// an empty map if nothing exists in the store.
	GetAll() map[string]interface{}

	// Delete removes an interface from the store, using the supplied key. It
	// does nothing if the key does not exist.
	Delete(string)
}

// store keeps track of keys/values in memory.
type store struct {
	data map[string]interface{}
	// --
	*sync.RWMutex // guards data
}

// NewStore returns a new Store.
func NewStore() Store {
	return &store{
		data:    make(map[string]interface{}),
		RWMutex: &sync.RWMutex{},
	}
}

func (s *store) Add(key string, val interface{}) error {
	s.Lock()
	defer s.Unlock()

	// If the key already exists, return an error.
	if _, exists := s.data[key]; exists {
		return ErrConflict
	}

	s.data[key] = val
	return nil
}

func (s *store) Set(key string, val interface{}) {
	s.Lock()
	s.data[key] = val
	s.Unlock()
}

func (s *store) Get(key string) (interface{}, error) {
	s.RLock()
	defer s.RUnlock()

	i, ok := s.data[key]
	// If the key is not in the store, return an error.
	if !ok {
		return nil, ErrKeyNotFound
	}
	return i, nil
}

func (s *store) GetAll() map[string]interface{} {
	s.RLock()
	defer s.RUnlock()

	// Return a copy of the map to avoid subjecting clients to potential
	// race conditions.
	resp := make(map[string]interface{})
	for k, v := range s.data {
		resp[k] = v
	}

	return resp
}

func (s *store) Delete(key string) {
	s.Lock()
	delete(s.data, key)
	s.Unlock()
}
