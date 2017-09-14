/*
  This data is used by tests in the request-manager/jc package.
*/

-- an invalid job chain
INSERT INTO raw_requests (request_id, request, job_chain) VALUES ("cd724fd1209247eea1c41d8d41b22830", '{"some":"param"}', '{"requestId":"8bff5def4f3f4e429bec07723e905265","jobs":{"this is not valid json"}},"adjacencyList":null,"state":4}');
