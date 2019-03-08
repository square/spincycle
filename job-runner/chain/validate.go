// Copyright 2017-2019, Square, Inc.

package chain

import (
	"github.com/square/spincycle/proto"
)

// ErrInvalidChain is the error returned when a chain is not valid.
type ErrInvalidChain struct {
	Message string
}

func (e ErrInvalidChain) Error() string {
	return e.Message
}

// Validate checks if a job chain is valid. It returns an error if it's not.
func Validate(jobChain proto.JobChain) error {
	// Make sure the adjacency list is valid.
	if !adjacencyListIsValid(jobChain) {
		return ErrInvalidChain{
			Message: "invalid adjacency list: some jobs exist in " +
				"chain.AdjacencyList but not chain.Jobs",
		}
	}

	// Make sure there is one first job.
	if !hasFirstJob(jobChain) {
		return ErrInvalidChain{
			Message: "job chain has more than one start job (node with indegree count > 0)",
		}
	}

	// Make sure there is one last job.
	if !hasLastJob(jobChain) {
		return ErrInvalidChain{
			Message: "job chain has more than one start job (node with indegree count > 0)",
		}
	}

	// Make sure there are no cycles.
	if !isAcyclic(jobChain) {
		return ErrInvalidChain{Message: "chain is cyclic"}
	}

	return nil
}

// adjacencyListIsValid returns whether or not the chain's adjacency list is
// not valid. An adjacency list is not valid if any of the jobs in it do not
// exist in chain.Jobs.
func adjacencyListIsValid(jobChain proto.JobChain) bool {
	for job, adjJobs := range jobChain.AdjacencyList {
		if _, ok := jobChain.Jobs[job]; !ok {
			return false
		}

		for _, adjJob := range adjJobs {
			if _, ok := jobChain.Jobs[adjJob]; !ok {
				return false
			}
		}
	}
	return true
}

// hasFirstJob finds the job in the chain with indegree 0. If there is not
// exactly one of these jobs, it returns an error.
func hasFirstJob(jobChain proto.JobChain) bool {
	n := 0
	for _, count := range indegreeCounts(jobChain) {
		if count == 0 {
			n++
		}
		if n > 1 {
			return false
		}
	}
	return true
}

// indegreeCounts finds the indegree for each job in the chain.
func indegreeCounts(jobChain proto.JobChain) map[string]int {
	indegreeCounts := make(map[string]int)
	for job := range jobChain.Jobs {
		indegreeCounts[job] = 0
	}
	for _, nextJobs := range jobChain.AdjacencyList {
		for _, nextJob := range nextJobs {
			if _, ok := indegreeCounts[nextJob]; ok {
				indegreeCounts[nextJob] += 1
			}
		}
	}
	return indegreeCounts
}

// hasLastJob finds the job in the chain with outdegree 0. If there is not
// exactly one of these jobs, it returns an error.
func hasLastJob(jobChain proto.JobChain) bool {
	n := 0
	for _, count := range outdegreeCounts(jobChain) {
		if count == 0 {
			n++
		}
		if n > 1 {
			return false
		}
	}
	return true
}

// outdegreeCounts finds the outdegree for each job in the chain.
func outdegreeCounts(jobChain proto.JobChain) map[string]int {
	outdegreeCounts := make(map[string]int)
	for job := range jobChain.Jobs {
		outdegreeCounts[job] = len(jobChain.AdjacencyList[job])
	}
	return outdegreeCounts
}

// isAcyclic returns whether or not a job chain is acyclic. It essentially
// works by moving through the job chain from the top (the first job)
// down to the bottom (the last job), and if there are any cycles in the
// chain (dependencies that go in the opposite direction...i.e., bottom to
// top), it returns false.
func isAcyclic(jobChain proto.JobChain) bool {
	indegreeCounts := indegreeCounts(jobChain)
	queue := make(map[string]struct{})

	// Add all of the first jobs to the queue (in reality there should
	// only be 1).
	for job, indegreeCount := range indegreeCounts {
		if indegreeCount == 0 {
			queue[job] = struct{}{}
		}
	}

	jobsVisited := 0
	for {
		// Break when there are no more jobs in the queue. This happens
		// when either there are no first jobs, or when a cycle
		// prevents us from enqueuing a job below.
		if len(queue) == 0 {
			break
		}

		// Get a job from the queue.
		var curJob string
		for k := range queue {
			curJob = k
		}
		delete(queue, curJob)

		// Visit each job adjacent to the current job and decrement
		// their indegree count by 1. When a job's indegree count
		// becomes 0, add it to the queue.
		//
		// If there is a cycle somewhere, at least one jobs indegree
		// count will never reach 0, and therefore it will never be
		// enqueued and visited.
		for _, adjJob := range jobChain.AdjacencyList[curJob] {
			indegreeCounts[adjJob] -= 1
			if indegreeCounts[adjJob] == 0 {
				queue[adjJob] = struct{}{}
			}
		}

		// Keep track of the number of jobs we've visited. If there is
		// a cycle in the chain, we won't end up visiting some jobs.
		jobsVisited += 1
	}

	if jobsVisited != len(jobChain.Jobs) {
		return false
	}

	return true
}
