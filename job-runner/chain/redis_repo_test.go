// Copyright 2017, Square, Inc.

package chain

import (
	"testing"

	"github.com/garyburd/redigo/redis"
	"github.com/square/spincycle/proto"
)

func TestAddIntegration(t *testing.T) {
	repo := getCleanRepo()

	conn := repo.connectionPool.Get()
	defer conn.Close()

	chain := buildEmptyChain()

	repo.Add(chain)

	// # keys should be 1 after Add
	keys, _ := redis.Strings(conn.Do("KEYS", "*"))

	ct := len(keys)
	if ct != 1 {
		t.Errorf("Expected to add 1 chain, got %d", ct)
	}

	// should err when a Chain already exists
	err := repo.Add(chain)
	if err == nil {
		t.Error("Expected duplicate error to be thrown")
	}
}

func TestGetIntegration(t *testing.T) {
}

func TestRemoveIntegration(t *testing.T) {
}

func TestSetIntegration(t *testing.T) {
}

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
