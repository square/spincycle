# Production-Ready MySQL Connection Pool

[![Go Report Card](https://goreportcard.com/badge/github.com/go-mysql/conn)](https://goreportcard.com/report/github.com/go-mysql/conn) [![GoDoc](https://godoc.org/github.com/go-mysql/conn?status.svg)](https://godoc.org/github.com/go-mysql/conn)

This package provides a production-ready MySQL connection pool for Go 1.9. It is a thin but useful wrapper around database/sql.DB that emphasizes sql.Conn and error handling. Specifically, this package has three features which are necessary in well-design production systems:

1. `Connector` interface
1. Human-friendly error handling
1. Stats for metrics and monitoring

More on all this later once this repo has an official release.

## Testing

Requires [MySQL Sandbox](https://github.com/datacharmer/mysql-sandbox). Install and export `MYSQL_SANDBOX_DIR` env var. For example: `MYSQL_SANDBOX_DIR=/Users/daniel/sandboxes/msb_5_7_21/` on a Mac. Tests take ~15s because the MySQL sandbox is restarted several times. Current test coverage: 100%.
