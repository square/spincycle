## Job Runner API
TODO: Fill this out

### Running the Code
1. Update the import path of your jobs in `spincycle/job/external/factory`
2. In the root `spincycle` directory, run `go get ./...`
3. Run the JR web server: `go run spincycle/job-runner/main.go`
4. Send commands to the web server:
```bash
# POST a new chain
curl -H "Content-Type: application/json" -X POST -d '<CHAIN_PAYLOAD>' localhost:9999/api/v1/job-chains

# PUT a chain that is running to start it
curl -X PUT localhost:9999/api/v1/job-chains/<REQUEST_ID_OF_THE_CHAIN>/start

# PUT a chain that is running to stop it
curl -X PUT localhost:9999/api/v1/job-chains/<REQUEST_ID_OF_THE_CHAIN>/stop

# GET the status of a running chain
curl localhost:9999/api/v1/job-chains/<REQUEST_ID_OF_THE_CHAIN>/status
```

### TODOs
* When a traverser finishes, it should POST back to the Request Manager API the final status of the chain.
* Make basic things configurable (ex: port for http server).
* Simplify http routing stuff.
