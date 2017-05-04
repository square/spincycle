// Copyright 2017, Square, Inc.

package chain

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
)

// A RedisConnectionPool is used for pooling redis connections.
type RedisConnectionPool interface {
	ActiveCount() int
	Close() error
	Get() redis.Conn
}

// RedisRepoConfig contains all info necessary to build a RedisRepo.
type RedisRepoConfig struct {
	Server      string        // Redis server name/ip
	Port        uint          // Redis server port
	Prefix      string        // Prefix for redis keys
	MaxIdle     int           // passed to redis.Pool
	IdleTimeout time.Duration // passed to redis.Pool
}

type RedisRepo struct {
	ConnectionPool RedisConnectionPool // connection pool
	Conf           RedisRepoConfig     // config this Repo was built with
}

// NewRedisRepo builds a new Repo backed by redis
func NewRedisRepo(c RedisRepoConfig) (*RedisRepo, error) {
	// build connection pool
	addr := fmt.Sprintf("%s:%d", c.Server, c.Port)

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
		ConnectionPool: pool,
		Conf:           c,
	}

	err := r.ping()

	return r, err
}

// Add adds a chain to redis and returns any error encountered.  It returns an
// error if there is already a Chain with the same RequestId. Keys are of the
// form "#{RedisRepo.conf.Prefix}::#{CHAIN_KEY}::#{RequestId}"
func (r *RedisRepo) Add(chain *chain) error {
	conn := r.ConnectionPool.Get()
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
		return ErrConflict
	}

	return nil
}

// Set writes a chain to redis, overwriting any if it exists. Returns any
// errors encountered
func (r *RedisRepo) Set(chain *chain) error {
	conn := r.ConnectionPool.Get()
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
	conn := r.ConnectionPool.Get()
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
	chain.RWMutex = &sync.RWMutex{} // need to initialize the mutex

	return chain, nil
}

// Remove takes a Chain RequestId and deletes that Chain from redis
func (r *RedisRepo) Remove(id uint) error {
	conn := r.ConnectionPool.Get()
	defer conn.Close()

	key := r.fmtIdKey(id)

	num, err := redis.Uint64(conn.Do("DEL", key))
	if err != nil {
		return err
	}

	switch num {
	case 0:
		return ErrNotFound
	case 1:
		return nil // Success!
	default:
		// It's bad if we ever reach this
		return ErrMultipleDeleted
	}
}

// ping grabs a single connection and runs a PING against the redis server
func (r *RedisRepo) ping() error {
	conn := r.ConnectionPool.Get()
	defer conn.Close()

	_, err := conn.Do("PING")
	return err
}

// fmtIdKey takes a Chain RequestId and returns the key where that Chain is
// stored in redis
func (r *RedisRepo) fmtIdKey(id uint) string {
	return fmt.Sprintf("%s::%s::%d", r.Conf.Prefix, CHAIN_KEY, id)
}

// fmtChainKey takes a Chain and returns the key where that Chain is stored in
// redis
func (r *RedisRepo) fmtChainKey(chain *chain) string {
	return r.fmtIdKey(chain.JobChain.RequestId)
}
