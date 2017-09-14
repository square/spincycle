// Copyright 2017, Square, Inc.

package db

import (
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/square/spincycle/config"
	rmtest "github.com/square/spincycle/request-manager/test"
	"github.com/square/spincycle/test"
)

type manager struct {
	defaultDSNConfig *mysql.Config
	dbType           string
	defaultCmdArgs   []string
	cliPath          string
	//
	dbs map[string]*sql.DB // db name => *sql.Db
	*sync.Mutex
}

// A Manager manages the lifecycle of test databases. It is safe to use
// with multiple databases concurrently.
type Manager interface {
	// Create creates a new db and populates it the the given data file.
	// It returns the name of the db it creates.
	Create(dataFile string) (string, error)

	// Connect takes a db name and returns a *sql.DB for it.
	Connect(dbName string) (*sql.DB, error)

	// Destroy closes a db's connection and then drops the db.
	Destroy(dbName string) error
}

func NewManager() (Manager, error) {
	// Get db connection info from the test config.
	var cfg config.RequestManager
	err := config.Load(rmtest.ConfigFile, &cfg)
	if err != nil {
		return nil, err
	}
	defaultDSNConfig, err := mysql.ParseDSN(cfg.Db.DSN + "?parseTime=true") // parseTime=true always has to be set
	if err != nil {
		return nil, err
	}

	// Create default args for running the mysql cli with exec.Command.
	dca := defaultCmdArgs(defaultDSNConfig)

	var cliPath string
	if cfg.Db.CLIPath == "" {
		cliPath = "mysql" // default to current path
	}

	return &manager{
		defaultDSNConfig: defaultDSNConfig,
		dbType:           cfg.Db.Type,
		defaultCmdArgs:   dca,
		cliPath:          cliPath,
		dbs:              make(map[string]*sql.DB),
		Mutex:            &sync.Mutex{},
	}, nil
}

func (m *manager) Create(dataFile string) (string, error) {
	dbName := m.defaultDSNConfig.DBName + "_" + test.RandSeq(6)

	// Create a fresh db.
	if err := m.dropDB(dbName); err != nil {
		return dbName, err
	}
	if err := m.createDB(dbName); err != nil {
		return dbName, err
	}

	// Source the schema.
	if err := m.sourceFile(dbName, rmtest.SchemaFile); err != nil {
		return dbName, err
	}

	// Source test data.
	if dataFile != "" {
		if err := m.sourceFile(dbName, dataFile); err != nil {
			return dbName, err
		}
	}

	// Add the db to the dbs map.
	m.Lock()
	m.dbs[dbName] = nil
	m.Unlock()

	return dbName, nil
}

func (m *manager) Connect(dbName string) (*sql.DB, error) {
	m.Lock()
	defer m.Unlock()
	db, ok := m.dbs[dbName]
	if !ok {
		return nil, fmt.Errorf("db has not been setup")
	}

	if db == nil {
		// Override the db in the default dsn, and connect to the db.
		curDsn := *m.defaultDSNConfig
		curDsn.DBName = dbName
		db, err := sql.Open(m.dbType, curDsn.FormatDSN())
		if err != nil {
			return db, err
		}
		if err = db.Ping(); err != nil {
			return db, err
		}

		m.dbs[dbName] = db
	}

	return m.dbs[dbName], nil
}

func (m *manager) Destroy(dbName string) error {
	m.Lock()
	defer m.Unlock()
	if db, ok := m.dbs[dbName]; ok {
		if db != nil {
			db.Close()
		}

		// Drop the db.
		if err := m.dropDB(dbName); err != nil {
			return err
		}
	}

	delete(m.dbs, dbName)

	return nil
}

// -------------------------------------------------------------------------- //

func (m *manager) dropDB(dbName string) error {
	q := "DROP DATABASE IF EXISTS " + dbName
	err := m.execCLIQuery(q, nil)
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) createDB(dbName string) error {
	q := "CREATE DATABASE " + dbName
	err := m.execCLIQuery(q, nil)
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) sourceFile(dbName, file string) error {
	q := "SOURCE " + file
	extraArgs := []string{dbName} // pass this to the cli so that we source the file into the correct db
	err := m.execCLIQuery(q, extraArgs)
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) execCLIQuery(query string, extraArgs []string) error {
	cmdArgs := append(m.defaultCmdArgs, "-e", query)
	if extraArgs != nil {
		cmdArgs = append(cmdArgs, extraArgs...)
	}
	cmd := exec.Command(m.cliPath, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, output)
	}
	return nil
}

// Given a dsn, enerate default args for running the mysql cli with exec.Command.
func defaultCmdArgs(dsnConfig *mysql.Config) []string {
	addrSplit := strings.Split(dsnConfig.Addr, ":")
	var host, port string
	if len(addrSplit) > 0 {
		host = addrSplit[0]
	}
	if len(addrSplit) > 1 {
		port = addrSplit[1]
	}

	dca := []string{}
	if host != "" {
		dca = append(dca, "-h", host)
	}
	if port != "" {
		dca = append(dca, "-P", port)
	}
	if dsnConfig.User != "" {
		dca = append(dca, "-u", dsnConfig.User)
	}
	if dsnConfig.Passwd != "" {
		dca = append(dca, "-p", dsnConfig.Passwd)
	}

	return dca
}
