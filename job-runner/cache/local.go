// Copyright 2017, Square, Inc.

package cache

import (
	"sync"
)

// LocalCache is a Cacher that stores things in local memory in the following
// data structure: map[string]map[string]interface{}.
type LocalCache struct {
	data map[string]map[string]interface{}
	// --
	*sync.RWMutex // guards data
}

// NewLocalCache returns a new LocalCache.
func NewLocalCache() *LocalCache {
	return &LocalCache{
		data:    make(map[string]map[string]interface{}),
		RWMutex: &sync.RWMutex{},
	}
}

func (c *LocalCache) Add(db, key string, val interface{}) error {
	c.Lock()
	defer c.Unlock()
	if _, exists := c.data[db]; !exists {
		c.data[db] = make(map[string]interface{})
	}

	// If the key already exists, return an error.
	if _, exists := c.data[db][key]; exists {
		return ErrConflict
	}

	c.data[db][key] = val
	return nil
}

func (c *LocalCache) Get(db, key string) (interface{}, error) {
	c.RLock()
	defer c.RUnlock()
	if _, exists := c.data[db]; !exists {
		return nil, ErrDbNotFound
	}

	i, ok := c.data[db][key]
	// If the key is not in the cache, return an error.
	if !ok {
		return nil, ErrKeyNotFound
	}
	return i, nil
}

func (c *LocalCache) GetAll(db string) map[string]interface{} {
	c.RLock()
	defer c.RUnlock()
	if _, exists := c.data[db]; !exists {
		return nil
	}

	// Return a copy of the map to avoid subjecting clients to potential
	// race conditions.
	resp := make(map[string]interface{})
	for k, v := range c.data[db] {
		resp[k] = v
	}

	return resp
}

func (c *LocalCache) Delete(db, key string) {
	c.Lock()
	defer c.Unlock()
	if _, exists := c.data[db]; !exists {
		return
	}

	delete(c.data[db], key)
}
