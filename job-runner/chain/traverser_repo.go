// Copyright 2017, Square, Inc.

package chain

import (
	"errors"

	"github.com/square/spincycle/job-runner/kv"
)

var (
	ErrInvalidTraverser = errors.New("invalid traverser in the repo")
)

type TraverserRepo interface {
	Add(string, Traverser) error
	Get(string) (Traverser, error)
	Remove(string)
}

type traverserRepo struct {
	kv.Store
}

func NewTraverserRepo() *traverserRepo {
	return &traverserRepo{
		kv.NewStore(),
	}
}

func (r *traverserRepo) Add(requestId string, traverser Traverser) error {
	return r.Store.Add(requestId, traverser)
}

func (r *traverserRepo) Get(requestId string) (Traverser, error) {
	val, err := r.Store.Get(requestId)
	if err != nil {
		return nil, err
	}

	traverser, ok := val.(Traverser)
	if !ok {
		return nil, ErrInvalidTraverser
	}

	return traverser, nil
}

func (r *traverserRepo) Remove(requestId string) {
	r.Store.Delete(requestId)
}
