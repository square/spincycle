// Copyright 2017, Square, Inc.

package mock

import (
	"database/sql"
)

type Connector struct {
	ConnectFunc func() (*sql.DB, error)
	CloseFunc   func()
}

func (c *Connector) Connect() (*sql.DB, error) {
	if c.ConnectFunc != nil {
		return c.ConnectFunc()
	}
	return nil, nil
}

func (c *Connector) Close() {
	if c.CloseFunc != nil {
		c.CloseFunc()
	}
}
