// Copyright 2017, Square, Inc.

package chain

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
)

// RedisRepoConfig contains all info necessary to build a RedisRepo.
type RedisRepoConfig struct {
	Network     string        // Network for the redis server (ex: "tcp", "unix")
	Address     string        // Address for the redis server (ex: "localhost:6379", "/path/to/redis.sock")
	Prefix      string        // Prefix for redis keys
	MaxIdle     int           // passed to redis.Pool
	IdleTimeout time.Duration // passed to redis.Pool
}

type RedisRepo struct {
	ConnectionPool *redis.Pool     // redis connection pool
	Conf           RedisRepoConfig // config this Repo was built with
}

// NewRedisRepo builds a new Repo backed by redis
func NewRedisRepo(c RedisRepoConfig) (*RedisRepo, error) {
	// Build connection pool.
	pool := &redis.Pool{
		MaxIdle:     c.MaxIdle,
		IdleTimeout: c.IdleTimeout,
		Dial:        func() (redis.Conn, error) { return redis.Dial(c.Network, c.Address) },

		// Ping if connection's old and tear down if there's an error.
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}

	r := &RedisRepo{
		ConnectionPool: pool,
		Conf:           c,
	}

	return r, r.ping() // Make sure we can ping the redis.
}

// Add adds a chain to redis and returns any error encountered.  It returns an
// error if there is already a Chain with the same RequestId. Keys are of the
// form "#{RedisRepo.conf.Prefix}::#{CHAIN_KEY}::#{RequestId}".
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
// errors encountered.
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

// Get takes a Chain RequestId and retrieves that Chain from redis.
func (r *RedisRepo) Get(id string) (*chain, error) {
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
	chain.RWMutex = &sync.RWMutex{} // Need to initialize the mutex

	return chain, nil
}

// Get all job chains, even those owned/running on other JR instances.
func (r *RedisRepo) GetAll() ([]chain, error) {
	conn := r.ConnectionPool.Get()
	defer conn.Close()

	pattern := r.fmtIdKey("*")
	allIds := []interface{}{}
	cursor := 0
	for {
		vals, err := redis.MultiBulk(conn.Do("SCAN", cursor, "MATCH", pattern))
		if err != nil {
			return nil, err
		}
		cursor, _ = redis.Int(vals[0], nil)
		ids, _ := redis.Strings(vals[1], nil)
		if len(ids) > 0 {
			// SCAN applies MATCH last, so a set can be empy
			for _, id := range ids {
				allIds = append(allIds, id)
			}
		}
		if cursor == 0 {
			break
		}
	}
	vals, err := redis.Values(conn.Do("MGET", allIds...))
	if err != nil {
		return nil, err
	}
	chains := []chain{}
	for _, val := range vals {
		if val == nil {
			// Chain removed between SCAN and MGET
			continue
		}
		var c chain
		if err := json.Unmarshal(val.([]byte), &c); err != nil {
			return nil, err
		}
		chains = append(chains, c)
	}

	return chains, nil
}

// Remove takes a Chain RequestId and deletes that Chain from redis.
func (r *RedisRepo) Remove(id string) error {
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

// ping grabs a single connection and runs a PING against the redis server.
func (r *RedisRepo) ping() error {
	conn := r.ConnectionPool.Get()
	defer conn.Close()

	_, err := conn.Do("PING")
	return err
}

// fmtIdKey takes a Chain RequestId and returns the key where that Chain is
// stored in redis.
func (r *RedisRepo) fmtIdKey(id string) string {
	return fmt.Sprintf("%s:%s", r.Conf.Prefix, id)
}

// fmtChainKey takes a Chain and returns the key where that Chain is stored in
// redis.
func (r *RedisRepo) fmtChainKey(chain *chain) string {
	return r.fmtIdKey(chain.JobChain.RequestId)
}
