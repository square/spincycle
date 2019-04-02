---
layout: default
title: Dev Env
parent: Develop
nav_order: 1
---

# Development Environment

Run `docker-compose up` in the repo root directory to run Spin Cycle locally using the jobs and specs in `dev/`. Then compile `spinc`:

```
~/Development/go/src/github.com/square/spincycle/spinc/bin$ go build -o spinc

~/Development/go/src/github.com/square/spincycle/spinc/bin$ ./spinc
Request Manager address: http://127.0.0.1:32308

Requests:
  test

spinc help  <request>
spinc start <request>
```

The first line indicates that `spinc` is querying Spin Cycle locally.

Ideally, production could be simulated locally, allowing you to develop and test real Spin Cycle requests on your laptop. But in reality, this is rarely possible. The development environment is a laboratory for experimenting with and learning about Spin Cycle, and "lab work" usually needs to be adapted for production use.

## Rebuild

Docker containers, once built, are static. If you change files in `dev/`, you must `docker-compose build` to rebuild the containers, which copies `dev/`.
