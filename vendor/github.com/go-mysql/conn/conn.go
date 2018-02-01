// Package conn provides a safe MySQL connection pool with human-friendly error
// and metrics.
package conn

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	// ErrConnCannotConnect is returned when a connection cannot be made because
	// MySQL is down or unreachable for any reason. This is usually because
	// MySQL is down, but it could also indicate a network issue. If this error
	// occurs frequently, verify the network connection and MySQL address. If it
	// occurs infrequently, it could indicate a transient state that will
	// recover automatically; for example: a failover. Connector.Error returns true
	// with this error.
	ErrConnCannotConnect = errors.New("cannot connect")

	// ErrConnLost is returned when the connection is lost, closed, or killed
	// for any reason. This could mean MySQL crashed, or the KILL command was used
	// to kill the connection. Lost is distinct from down: a connection can only
	// be lost if it was previously connected. If MySQL is up and ok, this could
	// indicate a transient state that will recover automatically. If not,
	// ErrConnCannotConnect will probably be returned next when the driver tries
	// but fails to reestablish the connection. Connector.Error returns true with
	// this error.
	ErrConnLost = errors.New("connection lost")

	// ErrQueryKilled is returned when the KILL QUERY command is used. This only
	// kills the currently active query; the connection is still ok. This error
	// is not connection-related, so Connector.Error returns false.
	ErrQueryKilled = errors.New("query killed")

	// ErrReadOnly is returned when MySQL read-only is enabled. This error is not
	// connection-related, so Connector.Error returns false.
	ErrReadOnly = errors.New("server is read-only")

	// ErrDupeKey is returned when a unique index prevents a value from being
	// inserted or updated. This error is not connection-related, so Connector.Error
	// returns false.
	ErrDupeKey = errors.New("duplicate key value")

	// ErrTimeout is returned when a context timeout happens. This error is not
	// connection-related, so Connector.Error returns false.
	ErrTimeout = errors.New("timeout")
)

// A Connector creates sql.Conn without exposing how connections are made.
// Code that connects to MySQL is passed a Connector and it opens and closes
// unique connections as needed. Every connection open must be closed, else
// connection will be leaked.
type Connector interface {
	// Open opens a sql.Conn from the pool. It returns the error from sql.Conn,
	// if any. The caller should pass the error to Error to return a high-level
	// error exported by this package.
	Open(context.Context) (*sql.Conn, error)

	// Close closes a sql.Conn by calling its Close method. The caller should call
	// this method instead of calling conn.Close directly. Do not call both, but
	// one or the other must be called to return the connection to the pool. It
	// returns the error from conn.Close, if any.
	Close(*sql.Conn) error

	// Error returns true if the given error indicates the connection was lost,
	// else it returns false. If true, the caller should wait and retry the
	// operation. The driver will attempt to reconnect, but it usually takes
	// one or two attempts after the first error before the driver reestablishes
	// the connection. This is a driver implementation detail that cannot be
	// changed by the caller.
	//
	// The returned error can be different than the given error. Certain low-level
	// MySQL errors are returned as high-level errors exported by this package,
	// like ErrQueryKilled and ErrReadOnly. These high-level errors are common
	// and usually require special handling or retries by the caller. The caller
	// should check for and handle these high-level errors. Other errors are
	// probably not recoverable and should be logged and reported as an internal
	// error or bug.
	//
	// If the given error is not a MySQL error or nil, the method does nothing
	// and returns false and the given error.
	//
	// The caller should always call this method with any error from any code
	// that used a sql.Conn. See packages docs for the canonical workflow.
	Error(error) (bool, error)
}

// Stats represents counters and gauges for connection-related events. A Pool
// saves and exposes stats.
type Stats struct {
	*sync.Mutex
	Ts        int64 // Unix timestamp when stats were returned
	Open      int   // sql.DBStats.OpenConnections, not always accurate (gauge)
	OpenCalls int   // All calls to Open (counter)
	Opened    uint  // Successful calls to Open (counter)
	Closed    uint  // All calls to close (counter)
	Timeout   uint  // Timeouts during calls to Open (counter)
	Lost      uint  // Lost connections (counter)
	Down      uint  // ErrConnCannotConnect errors (counter)
}

// Pool represents a MySQL connection pool. It implements the Connector interface
// and exposes Stats.
type Pool struct {
	db    *sql.DB
	stats *Stats
}

// NewPool creates a new pool using the given, pre-configured sql.DB.
func NewPool(db *sql.DB) *Pool {
	return &Pool{
		db: db,
		stats: &Stats{
			Mutex: &sync.Mutex{},
		},
	}
}

// Open opens a database connection. See Connector.Open for more details.
func (p *Pool) Open(ctx context.Context) (*sql.Conn, error) {
	p.stats.Lock()
	p.stats.OpenCalls++
	p.stats.Unlock()

	conn, err := p.db.Conn(ctx)
	if err != nil {
		return conn, err
	}

	p.stats.Lock()
	p.stats.Opened++
	p.stats.Unlock()

	return conn, err
}

// Close closes a database connection. See Connector.Close for more details.
func (p *Pool) Close(conn *sql.Conn) error {
	err := conn.Close()

	p.stats.Lock()
	p.stats.Closed++
	p.stats.Unlock()

	return err
}

// Error determine if the given error is connection-related and possibly
// transforms it into a higher-level error exported by this package. See
// Connector.Error for more details.
func (p *Pool) Error(err error) (bool, error) {
	if err == nil {
		return false, nil
	}

	p.stats.Lock()
	defer p.stats.Unlock()

	if Lost(err) {
		p.stats.Lost++
		return true, ErrConnLost
	}

	if Down(err) {
		p.stats.Down++
		return true, ErrConnCannotConnect
	}

	// Not connection issues (return false)
	if errCode := MySQLErrorCode(err); errCode != 0 {
		switch errCode {
		case 1317: // ER_QUERY_INTERRUPTED
			return false, ErrQueryKilled
		case 1290, 1836: // ER_OPTION_PREVENTS_STATEMENT, ER_READ_ONLY_MODE
			return false, ErrReadOnly
		case 1062: // ER_DUP_ENTRY
			return false, ErrDupeKey
		}
	}

	if err == context.DeadlineExceeded {
		p.stats.Timeout++
		return false, ErrTimeout
	}

	// Not a MySQL error we handle, or some other type of error
	return false, err
}

// Stats returns the stats since the last call to Stats. Stats are never
// reset. The Open stat is not always accurate. The driver updates it lazily,
// so it can be higher than the actual number of currently open connections.
//
// The caller is expected to poll Stats at regular intervals. It is safe to
// safe to call concurrently.
func (p *Pool) Stats() Stats {
	// Set ts and copy stats
	p.stats.Lock()
	defer p.stats.Unlock()
	dbstats := p.db.Stats()
	return Stats{
		Ts:        time.Now().Unix(),
		Open:      dbstats.OpenConnections,
		OpenCalls: p.stats.OpenCalls,
		Opened:    p.stats.Opened,
		Closed:    p.stats.Closed,
		Lost:      p.stats.Lost,
		Timeout:   p.stats.Timeout,
		Down:      p.stats.Down,
	}
}

// //////////////////////////////////////////////////////////////////////////
// Helper functions
// //////////////////////////////////////////////////////////////////////////

// Lost returns true if the error indicates the MySQL connection was lost for
// any reason. Lost is distinct from down: a connection can only be lost if it
// was previously connected. A connection can be lost for many reasons. MySQL
// could be up and ok, but particular connections were lost. For example, the
// KILL command causes a lost connection.
func Lost(err error) bool {
	// mysql.ErrInvalidConn is returned for sql.DB functions. driver.ErrBadConn
	// is returned for sql.Conn functions. These are the normal errors when
	// MySQL is lost.
	if err == mysql.ErrInvalidConn || err == driver.ErrBadConn {
		return true
	}

	// Server shutdown in progress is a special case: the conn will be lost
	// soon. The next call will most likely return in the block above ^.
	if errCode := MySQLErrorCode(err); errCode == 1053 { // ER_SERVER_SHUTDOWN
		return true
	}

	return false
}

// Down returns true if the error indicates MySQL cannot be reached for any
// reason, probably because it's not running. Down is distinct from lost: it's
// a network-level error that means MySQL cannot be reached. MySQL could be up
// and ok, but particular connections are failing, so from the program's point
// of view MySQL is down.
func Down(err error) bool {
	// Being unable to reach MySQL is a network issue, so we get a net.OpError.
	// If MySQL is reachable, then we'd get a mysql.* or driver.* error instead.
	_, ok := err.(*net.OpError)
	return ok
}

// MySQLErrorCode returns the MySQL server error code for the error, or zero
// if the error is not a MySQL error.
func MySQLErrorCode(err error) uint16 {
	if val, ok := err.(*mysql.MySQLError); ok {
		return val.Number
	}
	return 0 // not a mysql error
}
