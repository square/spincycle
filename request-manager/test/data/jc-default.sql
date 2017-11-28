/*
  This data is used by tests in the request-manager/jc package.
*/

-- a failed job chain
INSERT INTO raw_requests (request_id, request, job_chain) VALUES ("8bff5def4f3f", '{"some":"param"}', '{"requestId":"8bff5def4f3f","jobs":{"vr34":{"id":"vr34","type":"noop","bytes":null,"state":4,"args":null,"data":null,"retry":0,"retryWait":0}},"adjacencyList":null,"state":4}');
