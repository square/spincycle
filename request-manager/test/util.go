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
	ConfigFile string // The path to the config file used in tests.
	SchemaFile string // The path to the default db schema.
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	SpecPath, _ = filepath.Abs(path.Join(filepath.Dir(filename), "specs/"))
	DataPath, _ = filepath.Abs(path.Join(filepath.Dir(filename), "data/"))
	if os.Getenv("CI") == "true" {
		ConfigFile, _ = filepath.Abs("../config/test-ci.yaml")
	} else {
		ConfigFile, _ = filepath.Abs("../config/test-local.yaml")
	}
	SchemaFile, _ = filepath.Abs("../resources/request_manager_schema.sql")
}
