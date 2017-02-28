// Copyright 2017, Square, Inc.

package chain

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"
)

var (
	ErrKeyExists           = errors.New("key already exists in redis")
	ErrKeyNotFound         = errors.New("key not found in redis")
	ErrMultipleKeysDeleted = errors.New("multiple keys deleted from redis")
)

// RedisRepoConfig contains all info necessary to build a RedisRepo. Use
// NewRedisRepoConfig() to create one for use.
type RedisRepoConfig struct {
	Server      string        // Redis server name/ip
	Port        uint          // Redis server port
	Prefix      string        // Prefix for redis keys
	MaxIdle     int           // passed to redis.Pool
	IdleTimeout time.Duration // passed to redis.Pool
}

// NewRedisRepoConfig provides a RedisRepoConfig with defaults set. You must
// set Server
func NewRedisRepoConfig() RedisRepoConfig {
	return RedisRepoConfig{
		Port:        6379,
		Prefix:      "SpinCycle::ChainRepo",
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
	}
}

type RedisRepo struct {
	connectionPool *redis.Pool      // Redis connection pool
	conf           *RedisRepoConfig // config this Repo was built with
}

// NewRedisRepo builds a new Repo backed by redis
//func NewRedisRepo(c RedisRepoConfig) (Repo, error) {
func NewRedisRepo(c RedisRepoConfig) (*RedisRepo, error) {
	// build connection pool
	addr := c.Server + ":" + strconv.FormatUint(uint64(c.Port), 10)

	pool := &redis.Pool{
		MaxIdle:     c.MaxIdle,
		IdleTimeout: c.IdleTimeout,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", addr) },

		// ping if connection's old and tear down if there's an error
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}

	// is a ping test even worth it since we define TestOnBorrow above?
	r := &RedisRepo{
		connectionPool: pool,
		conf:           &c,
	}

	err := r.ping()

	return r, err
}

// Add adds a chain to redis and returns any error encountered.  It returns an
// error if there is already a Chain with the same RequestId. Keys are of the
// form "#{RedisRepo.conf.Prefix}::#{ChainKey}::#{RequestId}"
func (r *RedisRepo) Add(chain *chain) error {
	conn := r.connectionPool.Get()
	defer conn.Close()

	marshalled, err := json.Marshal(chain)
	if err != nil {
		return err
	}

	key := r.fmtChainKey(chain)

	ct, err := redis.Uint64(conn.Do("SETNX", key, marshalled))
	if err != nil {
		return err
	}

	if ct != 1 {
		return ErrKeyExists
	}

	return nil
}

// Set writes a chain to redis, overwriting any if it exists. Returns any
// errors encountered
func (r *RedisRepo) Set(chain *chain) error {
	conn := r.connectionPool.Get()
	defer conn.Close()

	marshalled, err := json.Marshal(chain)
	if err != nil {
		return err
	}

	key := r.fmtChainKey(chain)

	_, err = conn.Do("SET", key, marshalled)
	return err
}

// Get takes a Chain RequestId and retrieves that Chain from redis
func (r *RedisRepo) Get(id uint) (*chain, error) {
	conn := r.connectionPool.Get()
	defer conn.Close()

	key := r.fmtIdKey(id)

	data, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		return nil, err
	}

	var chain *chain

	err = json.Unmarshal(data, &chain)
	if err != nil {
		return nil, err
	}

	return chain, nil
}

// Remove takes a Chain RequestId and deletes that Chain from redis
func (r *RedisRepo) Remove(id uint) error {
	conn := r.connectionPool.Get()
	defer conn.Close()

	key := r.fmtIdKey(id)

	num, err := redis.Uint64(conn.Do("DEL", key))
	if err != nil {
		return err
	}

	switch num {
	case 0:
		return ErrKeyNotFound
	case 1:
		return nil // Success!
	default:
		// It's bad if we ever reach this
		return ErrMultipleKeysDeleted
	}
}

// ping grabs a single connection and runs a PING against the redis server
func (r *RedisRepo) ping() error {
	conn := r.connectionPool.Get()
	defer conn.Close()

	_, err := conn.Do("PING")
	return err
}

// ChainKey is the keyspace for serialized Chains
const ChainKey = "ChainById"

// fmtIdKey takes a Chain RequestId and returns the key where that Chain is
// stored in redis
func (r *RedisRepo) fmtIdKey(id uint) string {
	s := strconv.FormatUint(uint64(id), 10)
	return r.conf.Prefix + "::" + ChainKey + "::" + s
}

// fmtIdKey takes a Chain and returns the key where that Chain is stored in
// redis
func (r *RedisRepo) fmtChainKey(chain *chain) string {
	return r.fmtIdKey(chain.JobChain.RequestId)
}
