/*
  This data is used by tests in the request-manager/request package.
*/

-- a pending request + job chain
INSERT INTO requests (request_id, type, user, created_at, state) VALUES ("0874a524aa1edn3ysp00", 'some-type', 'john', '2017-09-13 00:00:00', 1);
INSERT INTO raw_requests (request_id, request, job_chain) VALUES ("0874a524aa1edn3ysp00", '{"some":"param"}', '{"requestId":"0874a524aa1edn3ysp00","jobs":{"1q2w":{"id":"1q2w","type":"dummy","bytes":null,"state":1,"args":null,"data":null,"retry":0,"retryWait":0}},"adjacencyList":null,"state":1}');

-- a running request + job chain + job logs
INSERT INTO requests (request_id, type, created_at, state, finished_jobs) VALUES ("454ae2f98a05cv16sdwt", 'do-something', '2017-09-13 01:00:00', 2, 4);
INSERT INTO raw_requests (request_id, request, job_chain) VALUES ("454ae2f98a05cv16sdwt", '{"some":"param"}', '{"requestId":"454ae2f98a05cv16sdwt","jobs":{"590s":{"id":"590s","type":"fake","bytes":null,"state":3,"args":null,"data":null,"retry":0,"retryWait":0},"9sa1":{"id":"9sa1","type":"fake","bytes":null,"state":4,"args":null,"data":null,"retry":0,"retryWait":0},"di12":{"id":"di12","type":"fake","bytes":null,"state":3,"args":null,"data":null,"retry":0,"retryWait":0},"g012":{"id":"g012","type":"fake","bytes":null,"state":1,"args":null,"data":null,"retry":0,"retryWait":0},"ldfi":{"id":"ldfi","type":"fake","bytes":null,"state":2,"args":null,"data":null,"retry":0,"retryWait":0},"pzi8":{"id":"pzi8","type":"fake","bytes":null,"state":1,"args":null,"data":null,"retry":0,"retryWait":0}},"adjacencyList":{"590s":["g012"],"9sa1":["pzi8"],"di12":["ldfi","590s","9sa1"],"g012":["pzi8"],"ldfi":["pzi8"]},"state":2}');
INSERT INTO job_log (request_id, job_id, try, type, state) VALUES ("454ae2f98a05cv16sdwt", "di12", 0, "fake", 3),
                                                                  ("454ae2f98a05cv16sdwt", "590s", 0, "fake", 4), -- this job failed on its first try
                                                                  ("454ae2f98a05cv16sdwt", "590s", 1, "fake", 3), -- succeeded on its second
                                                                  ("454ae2f98a05cv16sdwt", "g012", 0, "fake", 1),
                                                                  ("454ae2f98a05cv16sdwt", "9sa1", 0, "fake", 4), -- failed on its only try
                                                                  ("454ae2f98a05cv16sdwt", "pzi8", 0, "fake", 1);

-- a completed request
INSERT INTO requests (request_id, type, created_at, state) VALUES ("93ec156e204ety45sgf0", 'something-else', '2017-09-13 02:00:00', 3);
