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
Request Manager address: https://mycorp.local:32308

Requests:
  checksum-mysql-metacluster
  provision-mysql-cluster
  provision-redis-cluster
  start-host
  stop-host
  test
  update-mysql-ibp

spinc help  <request>
spinc start <request>
```

This queries the Request Manager to obtain the list of requests.

## Commands

The spinc commands are:

| Command | Purpose | 
| ------- | -------- |
| help [command] | Print general help and command-specific help |
| info \<ID\>      | Print complete request information |
| log \<ID\>       | Print job log (hint: pipe output to less) |
| ps \[ID\]        | Show running requests and jobs. Request ID is optional. |
| running        | Exit 0 if request is running or pending, else exit 1 |
| start \<ID\>     | Start new request |
| status \<ID\>    | Print request status and basic information |
| stop \<ID\>      | Stop request |

Run `spinc start <request>` to start a request by name. It will prompt you for request arguments (args) in the order listed in the request spec, required then optional args.

Use `spinc status <request ID>` and `spinc log <request ID>` to check the status and results of a request.

`spinc ps` shows all running requests/jobs, analogous to Unix ps.
