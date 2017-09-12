// Copyright 2017, Square, Inc.

package chain_test

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/garyburd/redigo/redis"
	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/test"
)

var mredis *miniredis.Miniredis
var repo *chain.RedisRepo
var conn redis.Conn

func setup() {
	var err error

	addr := os.Getenv("SPINCYCLE_REDIS_ADDR")
	if addr == "" {
		mredis, err = miniredis.Run()
		if err != nil {
			log.Fatal(err)
		}
		port, err := strconv.ParseUint(mredis.Port(), 10, 0)
		if err != nil {
			log.Fatal(err)
		}
		addr = fmt.Sprintf("%s:%d", mredis.Host(), port)
	} else if conn == nil {
		log.Println("Using local Redis instance", addr)
		conn, err = redis.Dial("tcp", addr)
		if err != nil {
			log.Fatal(err)
		}
	}

	conf := chain.RedisRepoConfig{
		Network: "tcp",
		Address: addr,
		Prefix:  "SpinCycle::ChainRepo",
	}

	repo, err = chain.NewRedisRepo(conf)
	if err != nil {
		log.Fatal(err)
	}
}

func teardown() {
	if mredis != nil {
		mredis.Close()
	} else {
		conn.Do("FLUSHDB")
	}
}

func keys() []string {
	var keys []string
	if mredis != nil {
		keys = mredis.Keys()
	} else {
		keys, _ = redis.Strings(conn.Do("KEYS", "*"))
	}
	return keys
}

// --------------------------------------------------------------------------

func initJc() *proto.JobChain {
	return &proto.JobChain{
		RequestId: "abc",
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
		},
		Jobs: map[string]proto.Job{
			"j1": proto.Job{
				Id:    "j1",
				Type:  "something",
				State: proto.STATE_PENDING,
				Args: map[string]interface{}{
					"k": "v",
				},
				Data: map[string]interface{}{
					"some": "thing",
				},
			},
		},
	}
}

func TestAdd(t *testing.T) {
	setup()
	defer teardown()

	c := chain.NewChain(initJc())

	err := repo.Add(c)
	if err != nil {
		t.Errorf("error in Add: %v", err)
	}

	// # keys should be 1 after Add
	keys := keys()
	if len(keys) != 1 {
		t.Errorf("got %d keys, expected 1", len(keys))
	}

	// Should err when a Chain already exists
	err = repo.Add(c)
	if err != chain.ErrConflict {
		t.Error("Did not return expected duplicate key error")
	}
}

func TestGetSet(t *testing.T) {
	setup()
	defer teardown()

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
	setup()
	defer teardown()

	c := chain.NewChain(initJc())

	repo.Set(c)

	err := repo.Remove(c.JobChain.RequestId)
	if err != nil {
		t.Errorf("error in Remove: %v", err)
	}

	// # keys in redis should be back to 0
	keys := keys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys in redis after remove, had %v", len(keys))
	}

	// We expect an error if the key doesn't exist
	err = repo.Remove(c.JobChain.RequestId)
	if err != chain.ErrNotFound {
		t.Error("Did not return expected key not found error")
	}
}

func TestGetAll(t *testing.T) {
	setup()
	defer teardown()

	jc1 := &proto.JobChain{
		RequestId: "chain1",
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
		},
		Jobs: map[string]proto.Job{
			"job1": proto.Job{
				Id:    "job1",
				Type:  "type1",
				State: proto.STATE_PENDING,
			},
		},
	}
	c1 := chain.NewChain(jc1)
	c1.SetJobState("job1", proto.STATE_PENDING)

	jc2 := &proto.JobChain{
		RequestId: "chain2",
		AdjacencyList: map[string][]string{
			"jobA": []string{"jobB", "jobC"},
		},
		Jobs: map[string]proto.Job{
			"jobA": proto.Job{
				Id:    "jobA",
				Type:  "type2",
				State: proto.STATE_RUNNING,
			},
		},
	}
	c2 := chain.NewChain(jc2)
	c2.SetJobState("jobA", proto.STATE_RUNNING)

	if err := repo.Add(c1); err != nil {
		t.Fatalf("error in Add: %v", err)
	}
	if err := repo.Add(c2); err != nil {
		t.Fatalf("error in Add: %v", err)
	}

	keys := keys()
	if len(keys) != 2 {
		t.Errorf("got %d keys, expected 2", len(keys))
	}

	chains, err := repo.GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(chains) != 2 {
		t.Errorf("got %d chains, expected 2", len(chains))
	}

	for _, c := range chains {
		var expect *proto.JobChain
		var job string
		var expectRunning bool
		switch c.RequestId() {
		case "chain1":
			expect = jc1
			job = "job1"
			expectRunning = false
		case "chain2":
			expect = jc2
			job = "jobA"
			expectRunning = true
		default:
			t.Fatalf("got chain with nonexistent RequestId: %s", c.RequestId)
		}
		if diff := deep.Equal(c.JobChain, expect); diff != nil {
			t.Error(c.RequestId, diff)
		}
		if expectRunning {
			if len(c.Running) != 1 {
				test.Dump(c)
				t.Errorf("chain.Running has %d keys, expected 1", len(c.Running))
			}
			j, ok := c.Running[job]
			if !ok {
				t.Errorf("%s not set in chain.Running, expected it to be set", job)
			}
			if j.StartTs <= 0 {
				t.Errorf("chain.Running[%s].StartTs = %d, expected > 0", job, j.StartTs)
			}
		} else {
			if len(c.Running) != 0 {
				test.Dump(c)
				t.Errorf("chain.Running has %d keys, expected 0", len(c.Running))
			}
			_, ok := c.Running[job]
			if ok {
				t.Errorf("%s set in chain.Running, expected Running map to be empty", job)
			}
		}
	}
}
