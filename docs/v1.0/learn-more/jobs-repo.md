---
layout: default
title: Jobs Repo
parent: Learn More
nav_order: 2
---

# Jobs Repo

The jobs repo contains your jobs and a factory for making them. When compling Spin Cycle, the open-source code is "linked" to your jobs repo. (See [Building](/spincycle/v1.0/operate/deploy.html#building).) There are four requirements for your jobs repo:

1. Package name `jobs`
1. Defines package variable `var Factory job.Factory`
1. `Factory` implements [job.Factory](https://godoc.org/github.com/square/spincycle/job#Factory)
1. Every job implements [job.Job](https://godoc.org/github.com/square/spincycle/job#Job)


Open-source Spin Cycle only uses the `Factory` package variable. In request specs, jobs are given any type name you like. Spin Cycle makes jobs by type, by calling `Factory.Make`. Spin Cycle has no specific knowledge about any job. To Spin Cycle, every job is equal.

You can organize the jobs repo any way you want. For example:

```
# mycorp.local/spincycle/jobs:
factory.go
mysql/
  start.go
  stop.go
redis/
  start.go
  stop.go
```

The repo root is `mycorp.local/spincycle` with a `jobs/` package (requirement 1). The factory is implemented in `jobs` (requirement 2). Jobs in the `jobs` package are organized by database type: `mysql/` and `redis/`. Each has two jobs: `start` and `stop`. All jobs could be in the `jobs` package, too, or in one `db/` package, but we chose to organize them by database type.

`factory.go` looks like:

```go
package jobs

import (
    "fmt"

    "github.com/square/spincycle/job"

    "mycorp.local/spincycle/jobs/mysql"
    "mycorp.local/spincycle/jobs/redis"
)

var Factory job.Factory

func init() {
    Factory = &factory{}
}

type factory struct{}

func (f factory) Make(id Id) (Job, error) {
    switch id.Type {
    case "mysql/start":
        return mysql.NewStart(), nil
    case "mysql/stop":
        return mysql.NewStop(), nil
    case "redis/start":
        return redis.NewStart(), nil
    case "redis/stop":
        return redis.NewStop(), nil
    }
    return nil, fmt.Errorf("unknown job type: %s", id.Type)
}
```

A real factory is much more complicated, but the basic idea is the same. Private type `factory` implements [job.Factory](https://godoc.org/github.com/square/spincycle/job#Factory): `func (f factory) Make(id Id) (Job, error)` (requirement 3).

The `case` statements implicitly defined the job types. Specifying "mysql/start" in a request spec will match the first `case`, etc. You can name jobs types however you like. "mysql/start" could be called "mysql-start", "start_MySQL", etc. as long as it matches usage in request specs. We chose to name them matching the package organization.
