// Copyright 2017-2018, Square, Inc.

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

func (m *memoryRepo) Get(id string) (*Chain, error) {
	val, exists := m.ConcurrentMap.Get(id)
	if !exists {
		return nil, ErrNotFound
	}

	chain, ok := val.(*Chain)
	if !ok {
		return nil, ErrNotFound
	}

	return chain, nil
}

func (m *memoryRepo) GetAll() ([]Chain, error) {
	chains := []Chain{}
	for _, v := range m.ConcurrentMap.Items() {
		chain := v.(*Chain)
		chains = append(chains, *chain)
	}
	return chains, nil
}

func (m *memoryRepo) Add(chain *Chain) error {
	wasAbsent := m.ConcurrentMap.SetIfAbsent(chain.RequestId(), chain)
	if !wasAbsent {
		return ErrConflict
	}
	return nil
}

func (m *memoryRepo) Set(chain *Chain) error {
	m.ConcurrentMap.Set(chain.RequestId(), chain)
	return nil
}

func (m *memoryRepo) Remove(id string) error {
	m.ConcurrentMap.Remove(id)
	return nil
}
