// Copyright 2017, Square, Inc.

package test

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
)

var (
	SpecPath   string // Where test grapher spec files are stored.
	DataPath   string // Where test data is stored.
	SchemaFile string // The path to the default db schema.
	MySQLDSN   string // MySQL DSN
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	SpecPath, _ = filepath.Abs(path.Join(filepath.Dir(filename), "specs/"))
	DataPath, _ = filepath.Abs(path.Join(filepath.Dir(filename), "data/"))
	SchemaFile, _ = filepath.Abs("../resources/request_manager_schema.sql")

	MySQLDSN = os.Getenv("SPINCYCLE_TEST_MYSQL_DSN")
	if MySQLDSN == "" {
		MySQLDSN = "root:@tcp(localhost:3306)/request_manager_test?parseTime=true"
	}
}
