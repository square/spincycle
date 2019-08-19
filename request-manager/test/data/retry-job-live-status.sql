-- used for TestStatusJobRetried

INSERT INTO requests (request_id, type, user, created_at, started_at, total_jobs, finished_jobs, state, jr_url)
  VALUES ("aaabbbcccdddeeefff00", "req-name", "finch", "2019-03-04 00:00:00", "2019-03-04 00:00:00", 3, 0, 2, "http://localhost"); -- proto.STATE_RUNNING=2

INSERT INTO request_archives (request_id, create_request, args, job_chain)
  VALUES ("aaabbbcccdddeeefff00", '{"arg1":"arg1val"}', '', '{"requestId":"aaabbbcccdddeeefff00","jobs":{"0001":{"id":"0001","type":"job1Type","args":null,"retry":1,"retryWait":"3s"},"0002":{"id":"0002","type":"job2Type","args":null},"0003":{"id":"0003","type":"job3Type","args":null}}}');

-- In test, mock JR is still running this job on 2nd try, but 1st try failed so we have a JLE like:
INSERT INTO job_log (request_id, job_id, name, `type`, try, state, started_at, finished_at, error, `exit`)
  VALUES ("aaabbbcccdddeeefff00", "0001", "job1Name", "job1Type", 1, 4, "1551711684469893000", "1551711692603116000", "error msg", 1); -- proto.STATE_FAILED=4
