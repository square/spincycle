/*
  This data is used by tests in the request-manager/jl package.
*/

-- a running request + job logs
INSERT INTO requests (request_id, type, created_at, state, finished_jobs) VALUES ("fa0d862f16casg200lkf", 'do-something', '2017-09-13 01:00:00', 2, 7);
INSERT INTO job_log (request_id, job_id, name, try, type, state) VALUES ("fa0d862f16casg200lkf", "k238", "j1", 0, "test", 1),
                                                                        ("fa0d862f16casg200lkf", "fndu", "j2", 0, "test", 2),
                                                                        ("fa0d862f16casg200lkf", "g89d", "j4", 0, "test", 4),
                                                                        ("fa0d862f16casg200lkf", "g89d", "j3", 1, "test", 3);
