/*
  This data is used by tests in the request-manager/jc package.
*/

-- an invalid job chain
INSERT INTO raw_requests (request_id, request, job_chain) VALUES ("cd724fd12092", '{"some":"param"}', '{"requestId":"8bff5def4f3fvh78skjy","jobs":{"this is not valid json"}},"adjacencyList":null,"state":4}');
