---
layout: default
title: "Release Notes"
nav_order: 6
permalink: /release-notes
---

# Release Notes

## v1.0

### v1.0.0 (released 2019-04-09)

* First GA, production-ready release.

## v2.0

### v2.0.0 (released tbd)

* Request specs use backwards-incompatible sets-arg-as syntax. Script to upgrade [here](https://github.com/square/spincycle/util/reformat-yaml/).

  Old syntax:
  ```
  node-name:
    category: {job | sequence | conditional}
    type: node-type
    sets: [new-arg]
  ```
  New syntax:
  ```
  node-name:
    category: {job | sequence | conditional}
    type: node-type
    sets:
      - arg: new-arg
        as: new-arg
  ```
