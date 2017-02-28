// Copyright 2017, Square, Inc.

package chain

import (
	"testing"

	"github.com/garyburd/redigo/redis"
	"github.com/go-test/deep"
	"github.com/square/spincycle/proto"
)

func TestAdd(t *testing.T) {
	repo := getCleanRepo()

	conn := repo.connectionPool.Get()
	defer conn.Close()

	chain := buildEmptyChain()

	var err error

	err = repo.Add(chain)
	if err != nil {
		t.Errorf("error in Add: %v", err)
	}

	// # keys should be 1 after Add
	keys, _ := redis.Strings(conn.Do("KEYS", "*"))
	if len(keys) == 0 {
		t.Error("No new keys created in Redis")
	}

	// should err when a Chain already exists
	err = repo.Add(chain)
	if err != ErrKeyExists {
		t.Error("Did not return expected duplicate key error")
	}
}

func TestGetSet(t *testing.T) {
	repo := getCleanRepo()

	chain := buildEmptyChain()

	var err error

	// Should not exist before Set
	ret, _ := repo.Get(chain.JobChain.RequestId)
	if ret != nil {
		t.Error("request id exists before SET is executed")
	}

	err = repo.Set(chain)
	if err != nil {
		t.Errorf("error in Set: %v", err)
	}

	c, err := repo.Get(chain.JobChain.RequestId)
	if err != nil {
		t.Errorf("error in Get: %v", err)
	}

	diff := deep.Equal(chain, c)
	if diff != nil {
		t.Error(diff)
	}
}

func TestRemove(t *testing.T) {
	repo := getCleanRepo()

	conn := repo.connectionPool.Get()
	defer conn.Close()

	var err error

	chain := buildEmptyChain()

	repo.Set(chain)

	err = repo.Remove(chain.JobChain.RequestId)
	if err != nil {
		t.Errorf("error in Remove: %v", err)
	}

	// # keys in redis should be back to 0
	keys, err := redis.Strings(conn.Do("KEYS", "*"))
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys in redis after remove, had %v", len(keys))
	}

	// We expect an error if the key doesn't exist
	err = repo.Remove(chain.JobChain.RequestId)
	if err != ErrKeyNotFound {
		t.Error("Did not return expected key not found error")
	}
}

// buildEmptyChain returns a correctly-formed chain with a simple JobChain
func buildEmptyChain() *chain {
	jc := &proto.JobChain{
		RequestId: 1,
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
		},
	}

	return &chain{
		JobChain: jc,
	}
}

// getCleanRepo returns a *RedisRepo with an empty Redis db behind it for
// integration testing
func getCleanRepo() *RedisRepo {
	conf := NewRedisRepoConfig()
	conf.Server = "localhost"

	repo, _ := NewRedisRepo(conf)

	conn := repo.connectionPool.Get()
	defer conn.Close()

	conn.Do("FLUSHDB")

	return repo
}
