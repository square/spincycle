---
layout: default
title: Deploy
parent: Operate
nav_order: 1
---

# Deploy

Spin Cycle has three deployables: Request Manager API (RM), Job Runner API (JR), and spinc (CLI). The APIs can be deployed on the same machine, but it is recommended to deploy them separately. Network connectivity between APIs is required: RM connects to JR, and JR connects to RM. 

Both APIs are horizontally scalable: you can deploy N-many and distribute access with load balancing. The RM is completely stateless, but the JR is stateful with respect to the requests its currently running. Each RM instance must be able to connect directly and indirectly to each JR instance. JR instances report [server.addr](/spincycle/v2.0/operate/configure#jr.server.addr) as their address, and RM connect directly to these addresses to get request status. When starting new requests, RM connect to any JR using [jr_client.url](/spincycle/v2.0/operate/configure#rm.jr_client.url). See [Networking](/spincycle/v2.0/learn-more/networking).

The CLI, spinc, is deployed wherever convenient for users and can reach the RM, if network security is a concern.

## System Requirements

* Go 1.10 or newer
* MySQL 5.6 or newer
* Network allowed between APIs
* Network allowed from spinc to RM

## Building

To build open-source Spin Cycle, you must symlink it to your [jobs repo](/spincycle/v2.0/learn-more/jobs-repo). Presuming your jobs repo is located at `$GOPATH/src/mycorp.local/spincycle/jobs/`:

```bash
# Change to open-source Spin Cycle repo root
$ cd $GOPATH/src/github.com/square/spincycle

# Remove the empty jobs repo
$ rm -rf jobs/

# Symlink jobs to your jobs repo
$ ln -s $GOPATH/src/mycorp.local/spincycle/jobs/

# Verify that jobs in open-source report points to your jobs repo
$ ls -l jobs
lrwxr-xr-x  1 user user  62 Feb 24 10:47 jobs -> /go/src/mycorp.local/spincycle/jobs/
```

Now when you compile Spin Cycle, it will compile in your jobs repo. To compile each deployable:

```bash
$ cd request-manager/bin/
$ go build -o request-manager

$ cd ../../job-runner/bin/
$ go build -o job-runner

$ cd ../../spinc/bin/
$ go build -o spinc
```

Of course, you will probably script this build process in a CI system.

[Building with extensions](/spincycle/v2.0/develop/extensions#building) requires a different process.

## Deploying

Spin Cycle has no specific deployment requirements, other than being [configured](/spincycle/v2.0/operate/configure) correctly. Simply run the compiled binaries, provide a config file or environment variables, and&mdash;for the Request Manager&mdash;provide the specs dir. For example, we use a Docker container with a simple layout:

```sh
$ ls /app/request-manager
bin/
  request-manager
specs/
  <spec files>.yaml
config/
  production.yaml
```

Then run `bin/request-manager` from the root dir (`/app/request-manager`) and it will default to reading `config/produciton.yaml` (if environment varaible `ENVIRONMENT=production`) and read specs from `specs/`. The Job Runner is deployed the same, minus the specs.


### MySQL

Be sure to create the MySQL database and [schemas](https://github.com/square/spincycle/blob/master/request-manager/resources/request_manager_schema.sql). The database is configured in the DSN: [mysql.dsn](/spincycle/v2.0/operate/configure#rm.mysql.dsn). We suggest `spincycle_production` for production.

Also create the MySQL user, which is also configured in the DSN: [mysql.dsn](/spincycle/v2.0/operate/configure#rm.mysql.dsn). The user needs all privileges on the database.
