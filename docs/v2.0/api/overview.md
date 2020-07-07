---
layout: default
title: Overview
parent: API
nav_order: 1
---

# Overview
{: .no_toc }

* TOC
{:toc}

---

The Request Manager (RM) is the user-facing API of Spin Cycle. It exposes a standard JSON REST API to clients.

## Auth
By default, the Spin Cycle API does not require any form of authentication or authorization. If you would like to require these, please review the [Auth](/spincycle/v2.0/operate/auth.html) section for more details.

## Examples
* Create a new request
```
$> curl \
     -H "Content-Type: application/json" \
     -X POST -d '{"type":"test","args":{"sleepTime":"1000"}}' \
     ${rm-url}/api/v1/requests
```
* Get a request
```
$> curl \
     -H "Content-Type: application/json" \
     ${rm-url}/api/v1/requests/abcd1234
```
