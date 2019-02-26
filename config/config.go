// Copyright 2017-2019, Square, Inc.

// Package config provides structs describing Request Manager and Job Runner YAML config files.
// The top-level structs are RequestManager and JobRunner.
package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

// Default values are used when corresponding values are not specified in a config file.
// These are sufficient for development but not production.
const (
	DEFAULT_ADDR_REQUEST_MANAGER = "127.0.0.1:32308"
	DEFAULT_ADDR_JOB_RUNNER      = "127.0.0.1:32307"
	DEFAULT_MYSQL_DSN            = "root:@tcp(localhost:3306)/spincycle_development"
	DEFAULT_SPECS_DIR            = "specs/"
)

// Load loads a config file into the struct pointed to by configStruct.
func Load(cfgFile string, configStruct interface{}) error {
	var required bool
	if cfgFile != "" {
		required = true // required if specified
	} else if len(os.Args) > 1 {
		cfgFile = os.Args[1]
		required = true // required if specified
	} else {
		// Default config file not required
		switch os.Getenv("ENVIRONMENT") {
		case "staging":
			cfgFile = "config/staging.yaml"
		case "production":
			cfgFile = "config/production.yaml"
		default:
			cfgFile = "config/development.yaml"
		}
	}
	log.Printf("Loading config file %s", cfgFile)

	// Read the file.
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		if os.IsNotExist(err) && !required {
			log.Printf("Config file %s does not exist, using defaults", cfgFile)
			return nil
		}
		return err
	}

	// Unmarshal the contents of the file into the provided struct.
	return yaml.Unmarshal(data, configStruct)
}

// NewTLSConfig creates a tls.Config from the given cert, key, and ca files.
func NewTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("tls.LoadX509KeyPair: %s", err)
	}

	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

// Defaults returns a RequestManager and JobRunner with all default values.
func Defaults() (RequestManager, JobRunner) {
	rmCfg := RequestManager{
		Server: Server{
			Addr: DEFAULT_ADDR_REQUEST_MANAGER,
		},
		MySQL: MySQL{
			DSN: DEFAULT_MYSQL_DSN,
		},
		Specs: Specs{
			Dir: DEFAULT_SPECS_DIR,
		},
		JRClient: HTTPClient{
			ServerURL: "http://" + DEFAULT_ADDR_JOB_RUNNER,
		},
	}
	jrCfg := JobRunner{
		Server: Server{
			Addr: DEFAULT_ADDR_JOB_RUNNER,
		},
		RMClient: HTTPClient{
			ServerURL: "http://" + DEFAULT_ADDR_REQUEST_MANAGER,
		},
	}
	return rmCfg, jrCfg
}

// Env returns the envar value if set, else the default value.
func Env(envar, def string) string {
	val := os.Getenv(envar)
	if val != "" {
		return val
	}
	return def
}

// --------------------------------------------------------------------------

// Request Manager represents the top-level layout for a Request Manager (RM)
// YAML config file. An RM config file looks like:
//
//   ---
//   server:
//     addr: 10.0.0.50:32308
//     tls:
//       cert_file: myorg.crt
//       key_file: myorg.key
//       ca_file: myorg.ca
//   mysql:
//     dsn: "spincycle@tcp(spin-mysql.local:3306)/spincycle_production"
//   specs:
//     dir: /data/app/spin-rm/specs/
//   auth:
//     admin_roles: ["dba"]
//     strict: true
//   jr_client:
//     url: https://spincycle-jr.myorg.local:32307
//     tls:
//       cert_file: myorg.crt
//       key_file: myorg.key
//       ca_file: myorg.ca
//
// The reciprocal top-level config is JobRunner.
type RequestManager struct {
	Server   Server     `yaml:"server"`    // API addr and TLS
	MySQL    MySQL      `yaml:"mysql"`     // MySQL database
	Specs    Specs      `yaml:"specs"`     // request specs
	Auth     Auth       `yaml:"auth"`      // auth plugin
	JRClient HTTPClient `yaml:"jr_client"` // RM to JR internal communication
}

// JobRunner represents the top-level layout for a Job Runner (JR) YAML config file.
// A JR config file looks like:
//
//   ---
//   server:
//     addr: 10.0.0.55:32307
//     tls:
//       cert_file: myorg.crt
//       key_file: myorg.key
//       ca_file: myorg.ca
//   rm_client:
//     url: https://spincycle-rm.myorg.local:32308
//     tls:
//       cert_file: myorg.crt
//       key_file: myorg.key
//       ca_file: myorg.ca
//
// The reciprocal top-level config is RequestManager.
type JobRunner struct {
	Server   Server     `yaml:"server"`    // API addr and TLS
	RMClient HTTPClient `yaml:"rm_client"` // JR to RM internal communication
}

// --------------------------------------------------------------------------

// The auth section of RequestManager configures role-based authentication.
// To enable auth, you must provide an auth plugin.  Else, the default is no auth
// and these options are ignored.
type Auth struct {
	// Callers with one of these roles are admins (allowed all ops) for all requests.
	AdminRoles []string `yaml:"admin_roles"`

	// Strict requires all requests to have ACLs, else callers are denied unless
	// they have an admin role. Strict is disabled by default which, with the default
	// auth plugin, allows all callers (no auth).
	Strict bool `yaml:"strict"`
}

// The specs section of RequestManager configures the request specs.
type Specs struct {
	// Directory where all request specs are located. Subdirectories are ignored.
	//
	// The default is DEFAULT_SPECS_DIR.
	Dir string `yaml:"dir"`
}

// The server section configures the server and API. Both RequestManager and
// JobRunner have a server section.
type Server struct {
	// Addr is the network address ("IP:port") to listen on. For development,
	// this is the address clients connect to. But for production, the API is
	// usually behind load balancers or some type of NAT, so this is only the
	// local bind address.
	//
	// Specify ":port" to listen on all interface for the given port.
	//
	// The default is DEFAULT_ADDR_REQUEST_MANAGER or DEFAULT_ADDR_JOB_RUNNER.
	Addr string `yaml:"addr"`

	// TLS specifies certificate, key, and CA files to enable TLS connections.
	// If enabled, clients must connect using HTTPS.
	//
	// The default is not using TLS.
	TLS TLS `yaml:"tls"`
}

// HTTPClient represents sections jr_client (RequestManager.JRClient) and rm_client
// (JobRunner.RMClient) for configuring Job Runner and Request Manager HTTP clients,
// respectively.
type HTTPClient struct {
	// ServerURL is the base URL of the destination API (e.g. http://IP:port).
	// For development, this is Server.Addr. But for production, each API is
	// usually behind load balancers or some type of NAT, so this is probably
	// the load balancer/gateway address in front of the destination API.
	//
	// The default is the other API default address: DEFAULT_ADDR_REQUEST_MANAGER
	// or DEFAULT_ADDR_JOB_RUNNER.
	ServerURL string `yaml:"url"`

	// TLS specifies certificate, key, and CA files to enable TLS connections
	// to the destination API. The destation API must also be configured to use
	// TLS by specifying Server.TLS.
	//
	// The default is not using TLS.
	TLS `yaml:"tls"`
}

// Configuration for a SQL database.
type MySQL struct {
	// DSN is the data source name for connecting to MySQL. See
	// https://github.com/go-sql-driver/mysql#dsn-data-source-name for the
	// full syntax. The "parseTime=true" parameter is automatically appended
	// to the DSN.
	//
	// The DSN must end with "/db" where db is the database name where the
	// Request Manager schema has been loaded. Full privileges should be granted
	// on this database to the MySQL user specified by the DSN.
	//
	// To enable TLS, configure the TLS options, do not use the "tls" DSN parameter.
	//
	// The default is DEFAULT_MYSQL_DSN.
	DSN string `yaml:"dsn"`

	// TLS specifies certificate, key, and CA files to enable TLS connections
	// to MySQL. If enabled, the "tls" DSN parameter is used automatically.
	//
	// The default is no TLS.
	TLS `yaml:"tls"`

	// Path to mysql CLI. This is only used for testing.
	CLIPath string `yaml:"cli_path"`
}

// TLS represents the tls sections for Server, HTTPClient, and MySQL. Each tls
// section is unique, allowing different TLS files for each section.
//
// There are no defaults. Specify all files, or none.
type TLS struct {
	// The certificate file to use.
	CertFile string `yaml:"cert_file"`

	// The key file to use.
	KeyFile string `yaml:"key_file"`

	// The CA file to use.
	CAFile string `yaml:"ca_file"`
}
