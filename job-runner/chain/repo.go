// Copyright 2017, Square, Inc.

package chain

type Repo interface {
	Get(uint) (*chain, error)
	Add(*chain) error
	Remove(uint) error
	Set(*chain) error
}

type FakeRepo struct{}

func (f *FakeRepo) Get(id uint) (*chain, error) {
	return nil, nil
}

func (f *FakeRepo) Add(chain *chain) error {
	return nil
}

func (f *FakeRepo) Remove(id uint) error {
	return nil
}

func (f *FakeRepo) Set(chain *chain) error {
	return nil
}
