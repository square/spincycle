---
layout: default
title: Deploy
parent: Operate
nav_order: 1
---

# Deploy

Spin Cycle has three deployables: Request Manager API (RM), Job Runner API (JR), and spinc (CLI). The APIs can be deployed on the same machine, but it is recommended to deploy them separately. Network connectivity between APIs is required: RM connects to JR, and JR connects to RM. 

Both APIs are horizontally scalable: you can deploy N-many and distribute access with load balancing. The RM is completely stateless, but the JR is stateful with respect to the requests its currently running. Each RM instance must be able to connect directly and indirectly to each JR instance. JR instances report [server.addr](/spincycle/v1.0/operate/configure.html#jr.server.addr) as their address, and RM connect directly to these addresses to get request status. When starting new requests, RM connect to any JR using [jrclient.url](/spincycle/v1.0/operate/configure.html#rm.jrclient.url). See [Networking](/spincycle/v1.0/learn-more/networking.html).

The CLI, spinc, is deployed wherever convenient for users and can reach the RM, if network security is a concern.

## System Requirements

* Go 1.10 or newer
* MySQL 5.6 or newer
* Network allowed between APIs
* Network allowed from spinc to RM

## Building

To build open-source Spin Cycle, you must symlink it to your [jobs repo](/spincycle/v1.0/learn-more/jobs-repo.html). Presuming your jobs repo is located at `$GOPATH/src/mycorp.local/spincycle/jobs/`:

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

## Extending

