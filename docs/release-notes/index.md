---
layout: default
title: "Release Notes"
nav_order: 6
permalink: /release-notes
---

# Release Notes

## v2.0

### v2.1.0 (released 2023-05-12)

* RM find endpoint now supports filtering by request arg/value pairs
* Spinc cli find now supports filtering by request arg/value pairs

### v2.0.8 (released 2022-03-14)

* Fix exit code in SQL schema to be a signed bigint to match the int64 from Go code.

### v2.0.7 (released 2022-02-17)

* Fixed panic on `--help` and `--verison` introduced in v2.0.6

### v2.0.6 (released 2022-02-16)

* JR now handles jobs that end in an `UNKNOWN` state without a panic.

* `spinc` reports the user-specified options when printing the commands to re-run the request or get the status. Only includes options that were explicitly set by the user on the command line.

### v2.0.5 (released 2020-11-19)

* Minor adjustments to the linter.

* Improved performance when creating graphs from requests.

### v2.0.4 (released 2020-09-23)

* RM now [supports subdirectories](/spincycle/v2.0/develop/requests) for [specs.dir](/spincycle/v2.0/operate/configure#rm.specs.dir).

* RM runs a specs linter on startup. The linter is also available as a CLI. [More here](/spincycle/v2.0/develop/requests).

* `MakeGrapher` factory in RM removed and `LoadSpecs` hook function signature in RM changed.

### v2.0.0 (released 2020-07-07)

* Request specs use backwards-incompatible sets-arg-as syntax. Script to upgrade [here](https://github.com/square/spincycle/tree/master/util/reformat-yaml/).

  New syntax allows specs to rename arguments set by a node.

  Old syntax:
  ```
  node-name:
    category: {job | sequence | conditional}
    type: node-type
    ...
    sets: [new-arg]
  ```
  New syntax:
  ```
  node-name:
    category: {job | sequence | conditional}
    type: node-type
    ...
    sets:
      - arg: new-arg
        as: renamed-arg # optional if new-arg == renamed-arg
  ```

## v1.0

### v1.0.0 (released 2019-04-09)

* First GA, production-ready release.
