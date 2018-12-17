# Spin Cycle Sequence File Syntax


## v1.1

File Spec

```yaml
---
syntax_version: 1.1
sequences:
  name:
    request: true|false
    defaults:
      jobTimeout:   5s
      jobRetry:     0
      jobRetryWait: 500ms
      seqTimeout:   1m
      seqRetry:     0
      seqRetryWait: 500ms
    args:
      required:
        - name: foo
          desc: "Required foo arg"
      optional:
        - name: bar
          desc: "Optional bar arg"
          default: baz
    nodes:
      <node spec>
```

New Fields

| Field | Value |
| ----- | ----- |
| name | Globally unique sequence name |
| request | If `true`, sequence is a caller request and API returns in list of possible requests.  If `false`, sequence cannot be requested by caller, sequence can only be used internally. |
| defaults | Sequence-specific defaults that override API defaults. Sub-sequence do _not_ inherent these defaults. |
| args | Required and optional sequence argumetns |

Node Spec

```yaml
name:
  job:  <job type>
  seq:  <sequence name>
  each: node:nodes
  args: [actual:alias,...]
  sets: []
  deps: [<node name>,...]
  retry: 1
  retryWait: 500
```

## v1.0

File Spec

```yaml
---
sequences:
  name:
    args:
      required:
        - name: foo
          desc: "Required foo arg"
      optional:
        - name: bar
          desc: "Optional bar arg"
          default: baz
    nodes:
      <node spec>
```

Node Spec

```yaml
name1:
  category: job
  type:     <job type>
  args:
    - expected: x
      given: y
  sets: [<node name>,...]
  deps: [<node name>,...]
name2:
  category: sequence
  type:     <sequence name>
  each: hosts:host
  args:
  sets: []
  deps: []
```
