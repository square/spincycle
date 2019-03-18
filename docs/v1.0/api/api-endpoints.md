---
layout: default
title: API Endpoints
parent: API
nav_order: 1
---

# API Endpoints
{: .no_toc }

* TOC
{:toc}

---

## Requests
All things related to Spin Cycle [requests](/spincycle/v1.0/learn-more/basic-concepts#request).

### Create and start a new request
<div class="code-example" markdown="1">
POST
{: .label .label-blue .mt-3 }
`/api/v1/requests`
{: .d-inline }

#### Request Parameters
{: .no_toc }

| Parameter    | Type                   | Description                   |
|:-------------|:-----------------------|:------------------------------|
| type         | string                 | The type of request to create |
| args         | object                 | The arguments for the request |

#### Sample Request Body
{: .no_toc }

```json
{
  "type": "provision-database",
  "args": {
    "dbname": "app-cluster-1",
    "zone": 3,
    "size": "large"
  }
}
```

#### Sample Response
{: .no_toc }

```json
{
  "id": "bafebl1ddiob71ka5bag",
  "type": "provision-database",
  "state": 1
  "user": "kristen",
  "args": {
    "dbname": "app-cluster-1",
    "zone": 3,
    "size": "large"
  },
  "createdAt": "2019-03-15T16:49:59Z",
  "startedAt": "2019-03-15T16:49:59Z",
  "finishedAt": "2019-03-15T16:55:42Z",
  "totalJobs": 30,
  "finishedJobs": 0
}
```

#### Response Status Codes
{: .no_toc }

<strong>201</strong>: Successful operation.
{: .good-response .fs-3 .text-green-200 }

<strong>400</strong>: Invalid request. Either the request type does not exist, or the args are invalid.
{: .bad-response .fs-3 .text-red-200 }

<strong>503</strong>: The Request Manager (RM) API server is in the process of shutting down.
{: .bad-response .fs-3 .text-red-200 }

</div>
