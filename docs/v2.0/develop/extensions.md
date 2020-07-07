---
layout: default
title: Extensions
parent: Develop
nav_order: 4
---

# Extensions

Extensions are user-provided plugins, hooks, and factories. Extensions allow you to extend and modify how core parts of Spin Cycle work without changing the core code. As an open source project, Spin Cycle cannot include all possible solutions out of the box. [Auth](/spincycle/v2.0/operate/auth), for example, is user-specific. The auth plugin lets you tailor Spin Cycle auth for your security.

The `app` packages  define and document all the extensions:

* [request-manager/app](https://godoc.org/github.com/square/spincycle/request-manager/app)
* [job-runner/app](https://godoc.org/github.com/square/spincycle/job-runner/app)
* [spinc/app](https://godoc.org/github.com/square/spincycle/spinc/app)

## Start Sequence

Using any extension requires a custom build of Spin Cycle, which is described in the next section. But first, it is helpful to understand how Spin Cycle is started because after defining your extensions, your code needs to start Spin Cycle. The default start sequences are [request-manager/bin/main.go](https://github.com/square/spincycle/blob/master/request-manager/bin/main.go) and [job-runner/bin/main.go](https://github.com/square/spincycle/blob/master/job-runner/bin/main.go). The start sequence  is five steps which must be performed in order.

_1. App context_

The first step is creating an app context: `app.Defaults()`. This returns a default app context with default, built-in functions for all extensions. For example, the default auth plugin allows all callers. You should always begin with the defaults. Since main.go does not modify anything, the app context is not assigned to a variable. However, to define extensions, assign the app context to a variable: `appCtx := app.Defaults()`.

_2. Define extensions_

Extensions are defined in the app context, like:

```go
type authPlugin struct{}

appCtx.Plugins.Auth = authPlugin{}
```

That defines an `authPlugin{}` object as the auth plugin, presuming it implements [auth.Plugin](https://godoc.org/github.com/square/spincycle/request-manager/auth#Plugin).

_3. Create server_

Create a new server object with the app context: `s := server.NewServer(appCtx)`. This will be either a `request-manager/server` or `job-runner/server`.

_4. Boot server_

Boot the server: `err := s.Boot()`. Booting makes the server (RM or JR) ready to run. This is when most validation happens. Error are fatal; do not run the server on error.

_5. Run server_

The final step is running the server: `s.Run(true)`. This blocks until the server is stopped. After calling `Run()`, the API is listening on the configured address.

## Building

Since extensions require defining custom values in the app context (step 2), your code must import open-source Spin Cycle. Then you build your code, which builds Spin Cycle indirectly. Furthermore, if you extend and custom build one part of Spin Cycle, you should custom build the other parts. For example, if you define an auth plugin for the Request Manager, you should also custom build the Job Runner and spinc to ensure all parts originate from the same code base.

If your repo is `mycorp.local/spincycle`, it would contain:

```
job-runner/
  main.go
jobs/
  <jobs repo>
request-manager/
  main.go
spinc/
  main.go
vendor/
  github.com/
    square/
      spincycle/
        ...
```

Open-source Spin Cycle is a vendor package: `vendor/github.com/square/spincycle`. The main.go files contain your extensions, and you build the binaries from these instead of the open-source files.

Like a [normal build](/spincycle/v2.0/operate/deploy#building), your `jobs/` repo is needed, but the symlink is different:

```sh
$ rm -f vendor/github.com/square/spincycle/jobs
$ ln -s $PWD/jobs vendor/github.com/square/spincycle/jobs
```

The `jobs/` dir in the open-source vendor copy needs to be a symlink that points to the `jobs/` dir in your repo.

When you update the Spin Cycle vendor dep (`vendor/github.com/square/spincycle`), be sure to re-link the jobs directory to your jobs repo.
