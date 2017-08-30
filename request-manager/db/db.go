// Copyright 2017, Square, Inc.

// Package db provides database connections and helper functions.
package db

import (
	"crypto/tls"
	"database/sql"
	"sync"

	"github.com/go-sql-driver/mysql"
)

// A Connector provides a database connection. It encapsulates logic about
// where and how to connect, like the DSN and TLS config, so that code using
// a Connector does not need to know this logic. Implementations can also
// vary wrt connection pooling.
type Connector interface {
	Connect() (*sql.DB, error)
	Close()
}

// A ConnectionPool represents a standard sql.DB connection with max open > 0.
type ConnectionPool struct {
	maxOpen int
	maxIdle int
	dsn     string
	// --
	db *sql.DB
	*sync.Mutex
}

func NewConnectionPool(maxOpen, maxIdle int, dsn string, tlsConfig *tls.Config) *ConnectionPool {
	params := "?parseTime=true" // always needs to be set
	if tlsConfig != nil {
		mysql.RegisterTLSConfig("custom", tlsConfig)
		params += "&tls=custom"
	}
	dsn += params

	c := &ConnectionPool{
		maxOpen: maxOpen,
		maxIdle: maxIdle,
		dsn:     dsn,
		// --
		Mutex: &sync.Mutex{},
	}
	return c
}

func (c *ConnectionPool) Connect() (*sql.DB, error) {
	c.Lock()
	defer c.Unlock()

	if c.db != nil {
		if err := c.db.Ping(); err != nil {
			return c.db, nil // use existing conneciton
		}
	}

	// Make new conneciton
	db, err := sql.Open("mysql", c.dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(c.maxOpen)
	db.SetMaxIdleConns(c.maxIdle)

	c.db = db

	return c.db, nil
}

func (c *ConnectionPool) Close() {
	c.Lock()
	defer c.Unlock()

	if c.db == nil {
		return
	}
	c.db.Close()
	c.db = nil
}

// ["a","b"] -> "'a','b'"
func IN(vals []string) string {
	in := ""
	n := len(vals) - 1
	for i, val := range vals {
		in += "'" + val + "'"
		if i < n {
			in += ","
		}
	}
	return in
}
