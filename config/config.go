// Copyright 2017, Square, Inc.

package config

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

///////////////////////////////////////////////////////////////////////////////
// High-Level Config Structs
///////////////////////////////////////////////////////////////////////////////

// The config used by the Request Manager. this is read from in
// request-manager/bin/main.go
type RequestManager struct {
	// The config that the RM web server will run with.
	Server

	// The config that the RM will use to connect to its database.
	Db SQLDb `yaml:"db"`

	// The config that the RM will use to make a JR client with. The JR
	// client is used for communicating with the JR.
	JRClient HTTPClient `yaml:"jr_client"`

	// The directory that holds all of the grapher spec files. This dir
	// should not contain anything other than spec files.
	SpecFileDir string `yaml:"spec_file_dir"`

	// Auth specifies auth plugin options. If a user-provide auth plugin is not
	// given, these options are ignored and all users and apps have admin access.
	Auth Auth `yaml:"auth"`
}

// The config used by the Job Runner. This is read from in
// job-runner/bin/main.go
type JobRunner struct {
	// The config that the JR web server will run with.
	Server

	// The config that the JR will use to make a RM client with. The RM
	// client is used for communicating with the RM.
	RMClient HTTPClient `yaml:"rm_client"`

	// The type of backend to use for the chain repo. Choices are: memory,
	// redis. If this is set to redis, the config for the redis instance
	// will be retrieved from the RedisDb struct.
	ChainRepoType string `yaml:"chain_repo_type"`

	// The config that the JR will use to connect to redis (if
	// ChainRepoType is set to "redis").
	Redis RedisDb
}

///////////////////////////////////////////////////////////////////////////////
// Config Components
///////////////////////////////////////////////////////////////////////////////

// Configuration for a web server.
type Server struct {
	// The address the server will listen on (ex: "127.0.0.1:80").
	ListenAddress string `yaml:"listen_address"`

	// The TLS config used by the server.
	TLS `yaml:"tls_config"`
}

// Configuration for an HTTP client.
type HTTPClient struct {
	// The base URL of the service that this client communicates with
	// (ex: https://127.0.0.1:9340). If running multiple instances of the
	// service (Job Runner or Request Manager), this URL should point to a
	// load balancer.
	ServerURL string `yaml:"server_url"`

	// The TLS config used by the client.
	TLS `yaml:"tls_config"`
}

// Configuration for a SQL database.
type SQLDb struct {
	// The driverName that is passed to sql.Open() (ex: "mysql").
	Type string

	// The full Data Source Name (DSN) of the sql database (see
	// https://github.com/go-sql-driver/mysql#dsn-data-source-name for
	// MySQL documentation, or https://godoc.org/github.com/lib/pq for
	// PostgreSQL documentation).
	//
	// Note: if a TLS config is specified within the SQLDb struct, it
	// will automatically get appended to the DSN (you don't have to
	// include it in the string). Also, "parseTime=true" will always be
	// appended to the DSN, so you don't need to add that either.
	DSN string

	// The TLS config used to connect to the sql database..
	TLS `yaml:"tls_config"`

	// The path to the database's CLI tool. This is only used for
	// testing. Ex: /usr/bin/mysql
	CLIPath string `yaml:"cli_path"`
}

// Configuration for a Redis database.
type RedisDb struct {
	// The network for the redis server (ex: "tcp", "unix")
	Network string

	// The address for the redis server (ex: "localhost:6379", "/path/to/redis.sock")
	Address string

	// The prefix used for redis keys.
	Prefix string

	// The timeout lenght (in seconds) for connections.
	IdleTimeout int `yaml:"idle_timeout"`

	// The maximum number of idle connections in the redis pool.
	MaxIdle int `yaml:"max_idle"`
}

// TLS configuration.
type TLS struct {
	// The certificate file to use.
	CertFile string `yaml:"cert_file"`

	// The key file to use.
	KeyFile string `yaml:"key_file"`

	// The CA file to use.
	CAFile string `yaml:"ca_file"`
}

// Auth configuration.
type Auth struct {
	// Callers with one of these roles are admins (allowed all ops) for all requests.
	AdminRoles []string `yaml:"admin_roles"`

	// Strict requires all requests to have ACLs, else callers are denied unless
	// they have an admin role. Strict is disabled by default which, with the default
	// auth plugin, allows all callers (no auth).
	Strict bool `yaml:"strict"`
}

///////////////////////////////////////////////////////////////////////////////
// Loading Config
///////////////////////////////////////////////////////////////////////////////

// Load loads a configuration file into the struct pointed to by the
// configStruct argument.
func Load(configFile string, configStruct interface{}) error {
	// Make sure the file exists.
	_, err := os.Stat(configFile)
	if err != nil {
		return err
	}

	// Read the file.
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}

	// Unmarshal the contents of the file into the provided struct.
	err = yaml.Unmarshal(data, configStruct)
	if err != nil {
		return err
	}

	return nil
}
