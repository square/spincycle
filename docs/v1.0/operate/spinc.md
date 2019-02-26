---
layout: default
title: spinc (CLI)
parent: Operate
nav_order: 3
---

# spinc (CLI)

spinc is the command line interface (CLI) for humans to operate Spin Cycle. spinc uses the [Request Manager client](https://godoc.org/github.com/square/spincycle/request-manager#Client), so anything spinc does can be done through the Request Manager API. spinc just makes it a little easier for humans, rather than querying the API directly.

## addr

spinc needs the Request Manager address specified by `--addr`, or `ADDR` environment variable, or `addr: <URL>` in `/etc/spinc/spinc.yaml` or `~/.spinc.yaml`. Setting `addr` in one of the config YAML files is probably easiest. If many RM are deployed, this should be the URL of the load balancer.

Once `addr` is set, run `spinc` (no arguments) to list available requests:

```sh
$ spinc
spinc help  <request>
spinc start <request>

Requests:
  checksum-mysql-metacluster
  provision-mysql-cluster
  provision-redis-cluster
  start-host
  stop-host
  test
  update-mysql-ibp

'spinc help' for more
```

This queries the RM to obtain the list of requests.

## Commands

The main spinc commands are:

| Command | Purpose | 
| ------- | -------- |
| start | Start new request |
| stop  | Stop request |
| status | Print status of request |
| log | Print job log of request |
| ps | Show all running requests and jobs |

Run `spinc start <request>` to start a request by name. It will prompt you for request arguments (args) in the order listed in the request spec, required then optional args.

Use `spinc status <request ID>` and `spinc log <request ID>` to check the status and results of a request.

`spinc ps` shows all running requests/jobs, analogous to Unix ps.
