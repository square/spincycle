// Copyright 2017, Square, Inc.

package chain

import (
	"github.com/orcaman/concurrent-map"
)

type memoryRepo struct {
	cmap.ConcurrentMap
}

// NewMemoryRepo returns a repo that is backed by a thread-safe map in memory.
func NewMemoryRepo() *memoryRepo {
	return &memoryRepo{
		cmap.New(),
	}
}

func (m *memoryRepo) Get(id string) (*chain, error) {
	val, exists := m.ConcurrentMap.Get(id)
	if !exists {
		return nil, ErrNotFound
	}

	chain, ok := val.(*chain)
	if !ok {
		return nil, ErrNotFound
	}

	return chain, nil
}

func (m *memoryRepo) GetAll() ([]chain, error) {
	chains := []chain{}
	for _, v := range m.ConcurrentMap.Items() {
		chain := v.(*chain)
		chains = append(chains, *chain)
	}
	return chains, nil
}

func (m *memoryRepo) Add(chain *chain) error {
	wasAbsent := m.ConcurrentMap.SetIfAbsent(chain.RequestId(), chain)
	if !wasAbsent {
		return ErrConflict
	}
	return nil
}

func (m *memoryRepo) Set(chain *chain) error {
	m.ConcurrentMap.Set(chain.RequestId(), chain)
	return nil
}

func (m *memoryRepo) Remove(id string) error {
	m.ConcurrentMap.Remove(id)
	return nil
}
