# reformat-yaml

This directory contains the go code to reformat a single Spin Cycle v1.0 YAML request spec in compliance with Spin Cycle v2.0.

## Building and running

To build the executable, run `go build reformat.go` (in this directory). The executable will be named `reformat`.

```
usage: reformat [input file path] [output file path]
```

`run.sh` is an example bash script that finds all request specs (files that end with extension `.yaml`) in the Spin Cycle repo and reformats them. Note that `reformat` cannot write to the same file it is reformatting, i.e. the input and output file paths cannot be the same. Thus, `run.sh` writes the reformatted file to a temporary file, then copies that file to the appropriate location.

## Why is reformatting necessary?

See the [release notes](https://github.com/square/spincycle/blob/master/docs/release-notes/index.md).


* In cases where a job does not set any args, i.e. `sets: []`, the reformatter does nothing; this line passes through the reformatter as is.

* This line:
  ```
  sets: [arg1, arg2, arg3] # comment
  ```
  becomes:
  ```
  sets: # comment
    - arg: arg1
      as: arg1
    - arg: arg2
      as: arg2
    - arg: arg3
      as: arg3
   ```

* These lines:
  ```
  sets: # comment
    - arg1
    - arg2 #comment2
    - arg3
  ```
  become:
  ```
  sets: # comment
    - arg: arg1
      as: arg1
    - arg: arg2 #comment2
      as: arg2
    - arg: arg3
      as: arg3
   ```

As noted in the above examples, the reformatter preserves comments.

The reformatter infers the indent size from the `sets:` line.

## Possible errors while reformatting

* The reformatter expects any line containing a `sets:` definition to be have four (equal) indents, and will throw an error if this is not the case.
  * Four spaces before the `sets:` statement is valid.
  * Eight spaces before the `sets:` statement is valid.
  * Ten spaces before the `sets:` statement will throw an error.
  * Three spaces and a tab before the `sets:` statement will throw an error.

* If using `sets: [...]` notation, a closing bracket must be provided. Otherwise, the reformatter throws an error. (This is illegal YAML anyway.)

## What the reformatter isn't

Note that the reformatter does _not_ check syntax for any other part of the YAML file; any line not part of a `sets` block will pass through as is.

