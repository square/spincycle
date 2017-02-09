# External Job Factory

Modify `factory.go` in this directory to link your jobs into Spin Cycle.
If your jobs repo is located at `github.com/user/jobs`, then change only line


```go
import myJobs "github.com/square/spincycle/job/internal"
```

to

```go
import myJobs "github.com/user/jobs"
```

and rebuild Spin Cycle.
