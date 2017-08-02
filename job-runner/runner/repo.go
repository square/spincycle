// Copyright 2017, Square, Inc.

package runner

import (
	"fmt"

	"github.com/orcaman/concurrent-map"
)

// Repo is a small wrapper around a concurrent map that provides the ability to
// store and retreive Runners in a thread-safe way.
type Repo interface {
	Set(key string, value Runner)
	Remove(key string)
	Items() (map[string]Runner, error)
}

type repo struct {
	c cmap.ConcurrentMap
}

func NewRepo() Repo {
	return &repo{
		c: cmap.New(),
	}
}

// Set sets a Runner in the repo.
func (r *repo) Set(key string, value Runner) {
	r.c.Set(key, value)
}

// Remove removes a runner from the repo.
func (r *repo) Remove(key string) {
	r.c.Remove(key)
}

// Items returns a map of key => Runner with all the Runners in the repo.
func (r *repo) Items() (map[string]Runner, error) {
	runners := map[string]Runner{} // key => runner
	vals := r.c.Items()
	for key, val := range vals {
		runner, ok := val.(Runner)
		if !ok {
			return runners, fmt.Errorf("invalid runner in repo for key=%s", key) // should be impossible
		}
		runners[key] = runner
	}

	return runners, nil
}
