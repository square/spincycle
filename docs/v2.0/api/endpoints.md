---
layout: default
title: Endpoints
parent: API
nav_order: 2
---

# Endpoints
{: .no_toc }

* TOC
{:toc}

---

## Requests
All things related to Spin Cycle [requests](/spincycle/v2.0/learn-more/basic-concepts#request).

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
  "type": "test",
  "args": {
    "sleepTime": "1000"
  }
}
```

#### Sample Response
{: .no_toc }

```json
{
  "id": "bafebl1ddiob71ka5bag",
  "type": "test",
  "state": 1
  "user": "kristen",
  "args": {
    "sleepTime": "1000"
  },
  "createdAt": "2019-03-15T16:49:59Z",
  "startedAt": "2019-03-15T16:49:59Z",
  "finishedAt": "2019-03-15T16:55:42Z",
  "totalJobs": 2,
  "finishedJobs": 0
}
```

#### Response Status Codes
{: .no_toc }

<strong>201</strong>: Successful operation.
{: .good-response .fs-3 .text-green-200 }

<strong>400</strong>: Invalid request. Either the request type does not exist, or the args are invalid.
{: .bad-response .fs-3 .text-red-200 }

<strong>401</strong>: Unauthorized operation.
{: .bad-response .fs-3 .text-red-200 }

<strong>503</strong>: The Request Manager (RM) API server is in the process of shutting down.
{: .bad-response .fs-3 .text-red-200 }

</div>

### Get a request
<div class="code-example" markdown="1">
GET
{: .label .label-green .mt-3 }
`/api/v1/requests/${requestId}`
{: .d-inline }

#### Sample Response
{: .no_toc }

```json
{
  "id": "bihqongkp0sg00cq9vo0",
  "type": "test",
  "state": 3,
  "user": "kristen",
  "args": [
    {
      "Pos": 0,
      "Name": "sleepTime",
      "Desc": "How long to sleep (milliseconds) during the request. Useful to verify how RM and JR respond before request has finished.",
      "Type": "optional",
      "Given": true,
      "Default": "1000",
      "Value": "1000"
    }
  ],
  "createdAt": "2019-04-02T18:39:26Z",
  "startedAt": "2019-04-02T18:39:26Z",
  "finishedAt": "2019-04-02T18:39:27Z",
  "JobChain": {
    "requestId": "bihqongkp0sg00cq9vo0",
    "jobs": {
      "3RNT": {
        "id": "3RNT",
        "name": "wait",
        "type": "sleep",
        "bytes": "eyJkdXJhdGlvbiI6MTAwMDAwMDAwMH0=",
        "state": 1,
        "args": {
          "duration": "1000"
        },
        "retry": 0,
        "sequenceId": "FDbP",
        "sequenceRetry": 0
      },
      "eSTn": {
        "id": "eSTn",
        "name": "wait",
        "type": "sleep",
        "bytes": "eyJkdXJhdGlvbiI6MTAwMDAwMDAwMH0=",
        "state": 1,
        "args": {
          "duration": "1000"
        },
        "retry": 0,
        "sequenceId": "cDNR",
        "sequenceRetry": 0
      }
    },
    "adjacencyList": {
      "3RNT": [
        "eSTn"
      ]
    }
  },
  "totalJobs": 2,
  "finishedJobs": 2
}
```

#### Response Status Codes
{: .no_toc }

<strong>200</strong>: Successful operation.
{: .good-response .fs-3 .text-green-200 }

<strong>401</strong>: Unauthorized operation.
{: .bad-response .fs-3 .text-red-200 }

<strong>404</strong>: Request not found.
{: .bad-response .fs-3 .text-red-200 }

</div>

### Stop a request
<div class="code-example" markdown="1">
PUT
{: .label .label-yellow .mt-3 }
`/api/v1/requests/${requestId}/stop`
{: .d-inline }

#### Response Status Codes
{: .no_toc }

<strong>200</strong>: Successful operation.
{: .good-response .fs-3 .text-green-200 }

<strong>401</strong>: Unauthorized operation.
{: .bad-response .fs-3 .text-red-200 }

<strong>404</strong>: Request not found.
{: .bad-response .fs-3 .text-red-200 }

</div>

### Get all job logs for a request
<div class="code-example" markdown="1">
GET
{: .label .label-green .mt-3 }
`/api/v1/requests/${requestId}/log`
{: .d-inline }

#### Sample Response
{: .no_toc }

```json
[
  {
    "requestId": "bihqongkp0sg00cq9vo0",
    "jobId": "3RNT",
    "try": 1,
    "name": "wait",
    "type": "sleep",
    "startedAt": 1554230366094196500,
    "finishedAt": 1554230367094791700,
    "state": 3,
    "exit": 0,
    "error": "",
    "stdout": "",
    "stderr": ""
  },
  {
    "requestId": "bihqongkp0sg00cq9vo0",
    "jobId": "eSTn",
    "try": 1,
    "name": "wait",
    "type": "sleep",
    "startedAt": 1554230366095376600,
    "finishedAt": 1554230367096359700,
    "state": 3,
    "exit": 0,
    "error": "",
    "stdout": "",
    "stderr": ""
  }
]
```

#### Response Status Codes
{: .no_toc }

<strong>200</strong>: Successful operation.
{: .good-response .fs-3 .text-green-200 }

<strong>401</strong>: Unauthorized operation.
{: .bad-response .fs-3 .text-red-200 }

<strong>404</strong>: Request not found.
{: .bad-response .fs-3 .text-red-200 }

</div>

### Get logs for a specific job in a request
<div class="code-example" markdown="1">
GET
{: .label .label-green .mt-3 }
`/api/v1/requests/${requestId}/log/${jobId}`
{: .d-inline }

#### Sample Response
{: .no_toc }

```json
{
  "requestId": "bihqongkp0sg00cq9vo0",
  "jobId": "3RNT",
  "try": 1,
  "name": "wait",
  "type": "sleep",
  "startedAt": 1554230366094196500,
  "finishedAt": 1554230367094791700,
  "state": 3,
  "exit": 0,
  "error": "",
  "stdout": "",
  "stderr": ""
}
```

#### Response Status Codes
{: .no_toc }

<strong>200</strong>: Successful operation.
{: .good-response .fs-3 .text-green-200 }

<strong>401</strong>: Unauthorized operation.
{: .bad-response .fs-3 .text-red-200 }

<strong>404</strong>: Request or job not found.
{: .bad-response .fs-3 .text-red-200 }

</div>

### Get status of all running jobs and requests
<div class="code-example" markdown="1">
GET
{: .label .label-green .mt-3 }
`/api/v1/status/running`
{: .d-inline }

#### Sample Response
{: .no_toc }

```json
{
  "jobs": [
    {
      "requestId": "bihr0sgkp0sg00cq9vog",
      "jobId": "96i7",
      "type": "sleep",
      "name": "wait",
      "startedAt": 1554231410126312200,
      "state": 2,
      "status": "sleeping",
      "try": 1
    },
    {
      "requestId": "bihr0tgkp0sg00cq9vp0",
      "jobId": "4avk",
      "type": "sleep",
      "name": "wait",
      "startedAt": 1554231414572741000,
      "state": 2,
      "status": "sleeping",
      "try": 1
    }
  ],
  "requests": {
    "bihr0sgkp0sg00cq9vog": {
      "id": "bihr0sgkp0sg00cq9vog",
      "type": "test",
      "state": 2,
      "user": "",
      "createdAt": "2019-04-02T18:56:50Z",
      "startedAt": "2019-04-02T18:56:50Z",
      "finishedAt": null,
      "totalJobs": 2,
      "finishedJobs": 0
    },
    "bihr0tgkp0sg00cq9vp0": {
      "id": "bihr0tgkp0sg00cq9vp0",
      "type": "test",
      "state": 2,
      "user": "",
      "createdAt": "2019-04-02T18:56:55Z",
      "startedAt": "2019-04-02T18:56:55Z",
      "finishedAt": null,
      "totalJobs": 2,
      "finishedJobs": 0
    }
  }
}
```

#### Response Status Codes
{: .no_toc }

<strong>200</strong>: Successful operation.
{: .good-response .fs-3 .text-green-200 }

<strong>401</strong>: Unauthorized operation.
{: .bad-response .fs-3 .text-red-200 }

</div>

### Find requests that match certain conditions
<div class="code-example" markdown="1">
GET
{: .label .label-green .mt-3 }
`/api/v1/requests`
{: .d-inline }

Requests are returned in descending order by create time (i.e. most recently created first).

#### Optional Query Parameters
{: .no_toc }

| Parameter    | Description                      | Notes  |
|:-------------|:---------------------------------|:-------|
| type         | The type of request              |        |
| user         | The user who created the request |        |
| state        | The state of the request         | See [proto.go](https://godoc.org/github.com/square/spincycle/proto#pkg-variables) — the string name of the state, not the byte. Specify this parameter multiple times to search for multiple states. |
| since        | Return only requests which were running after this time  | Format: 2006-01-02T15:04:05.999999Z07:00 |
| until        | Return only requests which were running before this time | Format: 2006-01-02T15:04:05.999999Z07:00 |
| limit        | Maximum number of requests to return |    |
| offset       | Skip this number of requests     | Use with limit for pagination of results. |

#### Sample Response
{: .no_toc }

```json
[
  {
    "id": "bihr0sgkp0sg00cq9vog",
    "type": "test",
    "state": 2,
    "user": "Bob",
    "createdAt": "2019-04-02T18:56:50Z",
    "startedAt": "2019-04-02T18:56:50Z",
    "finishedAt": null,
    "totalJobs": 2,
    "finishedJobs": 0
  },
  {
    "id": "bihr0tgkp0sg00cq9vp0",
    "type": "test",
    "state": 3,
    "user": "Alice",
    "createdAt":  "2019-04-02T18:56:55Z",
    "startedAt":  "2019-04-02T18:56:55Z",
    "finishedAt": "2019-04-02T18:57:55Z",
    "totalJobs": 2,
    "finishedJobs": 2
  }
]
```

#### Response Status Codes
{: .no_toc }

<strong>200</strong>: Successful operation.
{: .good-response .fs-3 .text-green-200 }

<strong>400</strong>: Invalid parameters.
{: .bad-response .fs-3 .text-red-200 }

<strong>401</strong>: Unauthorized operation.
{: .bad-response .fs-3 .text-red-200 }

</div>

### Get list of all available requests
<div class="code-example" markdown="1">
GET
{: .label .label-green .mt-3 }
`/api/v1/request-list`
{: .d-inline }

#### Sample Response
{: .no_toc }

```json
[
  {
    "Name": "test",
    "Args": [
      {
        "Pos": 0,
        "Name": "sleepTime",
        "Desc": "How long to sleep (milliseconds) during the request. Useful to verify how RM and JR respond before request has finished.",
        "Type": "optional",
        "Given": false,
        "Default": "1000",
        "Value": null
      }
    ]
  }
]
```

#### Response Status Codes
{: .no_toc }

<strong>200</strong>: Successful operation.
{: .good-response .fs-3 .text-green-200 }

<strong>401</strong>: Unauthorized operation.
{: .bad-response .fs-3 .text-red-200 }

</div>
