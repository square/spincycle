---
layout: default
title: "Release Notes"
nav_order: 6
permalink: /release-notes
---

# Release Notes

## v2.0

### v2.0.0 (released 2020-09-23)

* RM now [supports subdirectories](/spincycle/v2.0/develop/requests) for [specs.dir](/spincycle/v2.0/operate/configure#rm.specs.dir).

* RM runs a specs linter on startup. The linter is also available as a CLI. [More here](/spincycle/v2.0/develop/requests).

* `MakeGrapher` factory in RM removed and `LoadSpecs` hook function signature in RM changed.

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
