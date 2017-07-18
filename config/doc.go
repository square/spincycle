/*
Copyright 2017, Square, Inc.

Package config provides the ability to load config files into predefined
structures that are used by SpinCycle. Specifically, the Request Manager uses
the RequestManager struct in request-manager/bin/main.go, and the Job Runner
uses the JobRunner struct in job-runner/bin/main.go. These structs provide
all of the config information needed to run these two services.

Types of config structs provided by this package:

* RequestManager: all of the config needed to run the RM

* JobRunner: all of the config needed to run the JR

* Server: the configuration for running a webserver (ex: the listen address the
  server should run on, the TLS config the server should run with, etc.)

* SQLDb: the configuration for connecting to a SQL database (ex: the type of
  the database, the DSN of the database server, the TLS config to use when
  connecting to the server, etc.)

* HTTPClient: the configuration to use for an HTTP client that will make HTTP
  requests to external services (ex: the URL of the external service the client
  should talk to, the TLS config the client should use when connecting to the
  remote service, etc.)

* RedisDb: the configuration for connecting to a Redis database (ex: the
  address of the redis server, etc.)

* TLS: the configuration for constructing a Go tls.Config (ex: the CA cert file
  to use, the key file to use, etc.)
*/
package config
