// Copyright 2017-2019, Square, Inc.

package runner

import (
	"github.com/orcaman/concurrent-map"
)

// Repo is a small wrapper around a concurrent map that provides the ability to
// store and retrieve Runners in a thread-safe way.
type Repo interface {
	Set(jobId string, runner Runner)
	Get(jobId string) Runner
	Remove(jobId string)
	Items() map[string]Runner
	Count() int
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
func (r *repo) Set(jobId string, runner Runner) {
	r.c.Set(jobId, runner)
}

func (r *repo) Get(jobId string) Runner {
	v, ok := r.c.Get(jobId)
	if !ok {
		return nil
	}
	return v.(Runner)
}

// Remove removes a runner from the repo.
func (r *repo) Remove(jobId string) {
	r.c.Remove(jobId)
}

// Items returns a map of jobId => Runner with all the Runners in the repo.
func (r *repo) Items() map[string]Runner {
	runners := map[string]Runner{} // jobId => runner
	for jobId, v := range r.c.Items() {
		runner, ok := v.(Runner)
		if !ok {
			panic("runner for job ID " + jobId + " is not type Runner") // should be impossible
		}
		runners[jobId] = runner
	}
	return runners
}

// Count returns the number of Runners in the repo.
func (r *repo) Count() int {
	return r.c.Count()
}
