// Copyright 2017, Square, Inc.

package chain_test

import (
	"log"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
)

func initRedis() (*miniredis.Miniredis, *chain.RedisRepo) {
	redis, err := miniredis.Run()
	if err != nil {
		log.Fatal(err)
	}

	port, err := strconv.ParseUint(redis.Port(), 10, 0)
	if err != nil {
		log.Fatal(err)
	}

	conf := chain.RedisRepoConfig{
		Server: redis.Host(),
		Port:   uint(port),
		Prefix: "SpinCycle::ChainRepo",
	}

	repo, err := chain.NewRedisRepo(conf)
	if err != nil {
		log.Fatal(err)
	}

	return redis, repo
}

func initJc() *proto.JobChain {
	return &proto.JobChain{
		RequestId: 1,
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
		},
	}
}

func TestAdd(t *testing.T) {
	redis, repo := initRedis()
	defer redis.Close()

	c := chain.NewChain(initJc())

	err := repo.Add(c)
	if err != nil {
		t.Errorf("error in Add: %v", err)
	}

	// # keys should be 1 after Add
	keys := redis.Keys()
	if len(keys) == 0 {
		t.Error("No new keys created in Redis")
	}

	// Should err when a Chain already exists
	err = repo.Add(c)
	if err != chain.ErrConflict {
		t.Error("Did not return expected duplicate key error")
	}
}

func TestGetSet(t *testing.T) {
	redis, repo := initRedis()
	defer redis.Close()

	c := chain.NewChain(initJc())

	// Should not exist before Set
	ret, err := repo.Get(c.JobChain.RequestId)
	if ret != nil {
		t.Error("request id exists before SET is executed")
	}

	err = repo.Set(c)
	if err != nil {
		t.Errorf("error in Set: %v", err)
	}

	gotC, err := repo.Get(c.JobChain.RequestId)
	if err != nil {
		t.Errorf("error in Get: %v", err)
	}

	diff := deep.Equal(gotC, c)
	if diff != nil {
		t.Error(diff)
	}
}

func TestRemove(t *testing.T) {
	redis, repo := initRedis()
	defer redis.Close()

	c := chain.NewChain(initJc())

	repo.Set(c)

	err := repo.Remove(c.JobChain.RequestId)
	if err != nil {
		t.Errorf("error in Remove: %v", err)
	}

	// # keys in redis should be back to 0
	keys := redis.Keys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys in redis after remove, had %v", len(keys))
	}

	// We expect an error if the key doesn't exist
	err = repo.Remove(c.JobChain.RequestId)
	if err != chain.ErrNotFound {
		t.Error("Did not return expected key not found error")
	}
}
