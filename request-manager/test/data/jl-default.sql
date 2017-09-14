/*
  This data is used by tests in the request-manager/jl package.
*/

-- a running request + job logs
INSERT INTO requests (request_id, type, created_at, state, finished_jobs) VALUES ("fa0d862f16ca4f14a0613e2c26562de6", 'do-something', '2017-09-13 01:00:00', 2, 7);
INSERT INTO job_log (request_id, job_id, try, type, state) VALUES ("fa0d862f16ca4f14a0613e2c26562de6", "k238", 0, "test", 1),
                                                                  ("fa0d862f16ca4f14a0613e2c26562de6", "fndu", 0, "test", 2),
                                                                  ("fa0d862f16ca4f14a0613e2c26562de6", "g89d", 0, "test", 4),
                                                                  ("fa0d862f16ca4f14a0613e2c26562de6", "g89d", 1, "test", 3);
