// Copyright 2017, Square, Inc.

package chain_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/garyburd/redigo/redis"
	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/test/mock"
)

var (
	// These are shared between tests and need to be reset before each test
	// is run. This happens automatically in initRepo.
	recvCmd  string
	recvArgs []interface{}
	closed   bool
)

// Init a repo which will return the provided reply value and reply error to the
// Do method of the redis connection that it returns.
func initRepo(replyValue interface{}, replyErr error) *chain.RedisRepo {
	// Reset vars.
	recvCmd = ""
	recvArgs = []interface{}{}
	closed = false

	// Mock a redis connection.
	conn := &mock.Conn{
		DoFunc: func(commandName string, args ...interface{}) (reply interface{}, err error) {
			recvCmd = commandName
			recvArgs = args
			return replyValue, replyErr
		},
		CloseFunc: func() error {
			closed = true
			return nil
		},
	}

	// Create a mock pool that will return the mocked connection.
	pool := &mock.Pool{
		GetFunc: func() redis.Conn { return conn },
	}

	// Create a repo that uses the mocked pool.
	return &chain.RedisRepo{
		ConnectionPool: pool,
		Conf: chain.RedisRepoConfig{
			Prefix: "the_prefix",
		},
	}

}

// Init a job chain.
func initJc() *proto.JobChain {
	return &proto.JobChain{
		RequestId: 1,
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
		},
	}
}

func TestAddSuccess(t *testing.T) {
	repo := initRepo(int64(1), nil)
	c := chain.NewChain(initJc())

	// Add the chain to the repo.
	err := repo.Add(c)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	// Make sure the command is what we expect.
	expectedRecvCmd := "SETNX"
	if recvCmd != expectedRecvCmd {
		t.Errorf("received command = %s, expected = %s", recvCmd, expectedRecvCmd)
	}

	// Make sure the args to the command are what we expect.
	serializedC, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	expectedRecvArgs := []interface{}{
		fmt.Sprintf("%s::%s::%d", repo.Conf.Prefix, chain.CHAIN_KEY, c.RequestId()),
		serializedC,
	}
	diff := deep.Equal(recvArgs, expectedRecvArgs)
	if diff != nil {
		t.Error(diff)
	}

	// Make sure the connection is closed.
	if closed == false {
		t.Errorf("connection was not closed")
	}
}

func TestAddError(t *testing.T) {
	// Init a repo that will return an error.
	c := chain.NewChain(initJc())
	repo := initRepo(int64(1), mock.ErrRedis)

	// Add the chain to the repo.
	err := repo.Add(c)
	if err != mock.ErrRedis {
		t.Errorf("error = nil, expected %s", mock.ErrRedis)
	}

	// Make sure the connection is closed.
	if closed == false {
		t.Errorf("connection was not closed")
	}

	// Init a repo that will return an unexpected reply.
	repo = initRepo(int64(2), nil)

	// Add the chain to the repo.
	err = repo.Add(c)
	if err != chain.ErrKeyExists {
		t.Errorf("error = nil, expected %s", chain.ErrKeyExists)
	}
}

func TestSetSuccess(t *testing.T) {
	repo := initRepo(int64(1), nil)
	c := chain.NewChain(initJc())

	// Set the chain in the repo.
	err := repo.Set(c)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	// Make sure the command is what we expect.
	expectedRecvCmd := "SET"
	if recvCmd != expectedRecvCmd {
		t.Errorf("received command = %s, expected = %s", recvCmd, expectedRecvCmd)
	}

	// Make sure the args to the command are what we expect.
	serializedC, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	expectedRecvArgs := []interface{}{
		fmt.Sprintf("%s::%s::%d", repo.Conf.Prefix, chain.CHAIN_KEY, c.RequestId()),
		serializedC,
	}
	diff := deep.Equal(recvArgs, expectedRecvArgs)
	if diff != nil {
		t.Error(diff)
	}

	// Make sure the connection is closed.
	if closed == false {
		t.Errorf("connection was not closed")
	}
}

func TestSetError(t *testing.T) {
	// Init a repo that will return an error.
	c := chain.NewChain(initJc())
	repo := initRepo(int64(1), mock.ErrRedis)

	// Set the chain in the repo.
	err := repo.Set(c)
	if err != mock.ErrRedis {
		t.Errorf("error = nil, expected %s", mock.ErrRedis)
	}

	// Make sure the connection is closed.
	if closed == false {
		t.Errorf("connection was not closed")
	}
}

func TestGetSuccess(t *testing.T) {
	c := chain.NewChain(initJc())
	serializedC, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	repo := initRepo(serializedC, nil)

	// Get the chain from the repo.
	recvC, err := repo.Get(c.RequestId())
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}
	diff := deep.Equal(recvC, c)
	if diff != nil {
		t.Error(diff)
	}

	// Make sure the command is what we expect.
	expectedRecvCmd := "GET"
	if recvCmd != expectedRecvCmd {
		t.Errorf("received command = %s, expected = %s", recvCmd, expectedRecvCmd)
	}

	// Make sure the args to the command are what we expect.
	expectedRecvArgs := []interface{}{
		fmt.Sprintf("%s::%s::%d", repo.Conf.Prefix, chain.CHAIN_KEY, c.RequestId()),
	}
	diff = deep.Equal(recvArgs, expectedRecvArgs)
	if diff != nil {
		t.Error(diff)
	}

	// Make sure the connection is closed.
	if closed == false {
		t.Errorf("connection was not closed")
	}
}

func TestGetError(t *testing.T) {
	// Init a repo that will return an error.
	c := chain.NewChain(initJc())
	serializedC, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	repo := initRepo(serializedC, mock.ErrRedis)

	// Get the chain from the repo.
	_, err = repo.Get(c.RequestId())
	if err != mock.ErrRedis {
		t.Errorf("error = nil, expected %s", mock.ErrRedis)
	}

	// Make sure the connection is closed.
	if closed == false {
		t.Errorf("connection was not closed")
	}
}

func TestRemoveSuccess(t *testing.T) {
	repo := initRepo(int64(1), nil)
	c := chain.NewChain(initJc())

	// Remove the chain from the repo.
	err := repo.Remove(c.RequestId())
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	// Make sure the command is what we expect.
	expectedRecvCmd := "DEL"
	if recvCmd != expectedRecvCmd {
		t.Errorf("received command = %s, expected = %s", recvCmd, expectedRecvCmd)
	}

	// Make sure the args to the command are what we expect.
	expectedRecvArgs := []interface{}{
		fmt.Sprintf("%s::%s::%d", repo.Conf.Prefix, chain.CHAIN_KEY, c.RequestId()),
	}
	diff := deep.Equal(recvArgs, expectedRecvArgs)
	if diff != nil {
		t.Error(diff)
	}

	// Make sure the connection is closed.
	if closed == false {
		t.Errorf("connection was not closed")
	}
}

func TestRemoveError(t *testing.T) {
	// Init a repo that will return an error.
	c := chain.NewChain(initJc())
	repo := initRepo(int64(1), mock.ErrRedis)

	// Remove the chain from the repo.
	err := repo.Remove(c.RequestId())
	if err != mock.ErrRedis {
		t.Errorf("error = nil, expected %s", mock.ErrRedis)
	}

	// Make sure the connection is closed.
	if closed == false {
		t.Errorf("connection was not closed")
	}

	// Init a repo that will return an unexpected reply #1.
	repo = initRepo(int64(0), nil)

	// Remove the chain from the repo.
	err = repo.Remove(c.RequestId())
	if err != chain.ErrKeyNotFound {
		t.Errorf("error = nil, expected %s", chain.ErrKeyNotFound)
	}

	// Init a repo that will return an unexpected reply #2.
	repo = initRepo(int64(2), nil)

	// Remove the chain from the repo.
	err = repo.Remove(c.RequestId())
	if err != chain.ErrMultipleKeysDeleted {
		t.Errorf("error = nil, expected %s", chain.ErrMultipleKeysDeleted)
	}
}
