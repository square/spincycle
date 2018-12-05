// Copyright 2017-2018, Square, Inc.

package db

import (
	"time"

	"github.com/square/spincycle/proto"
)

var (
	// These maps contain struct representations of dummy data stored
	// in the database for testing (sourced from the sql files in the
	// request-manager/test/data directory). Callers should never
	// modify them.
	//
	// IMPORTANT: if you change the structs below, you will also need
	// to update the corresponding sql that inserts them into the
	// database (in the request-manager/test/data directory).
	SavedRequests map[string]proto.Request           // requestId => request
	SavedJLs      map[string][]proto.JobLog          // requestId => []jl
	SavedJCs      map[string]proto.JobChain          // requestId => jc
	SavedSJCs     map[string]proto.SuspendedJobChain // requestId => sjc
)

func init() {
	SavedRequests = make(map[string]proto.Request)
	SavedJLs = make(map[string][]proto.JobLog)
	SavedJCs = make(map[string]proto.JobChain)
	SavedSJCs = make(map[string]proto.SuspendedJobChain)

	// //////////////////////////////////////////////////////////////////////////
	// Pending request
	// //////////////////////////////////////////////////////////////////////////
	reqId := "0874a524aa1edn3ysp00"
	curTime, _ := time.Parse(time.RFC3339, "2017-09-13T00:00:00Z")
	SavedRequests[reqId] = proto.Request{
		Id:        reqId,
		Type:      "some-type",
		User:      "john",
		CreatedAt: curTime,
		State:     proto.STATE_PENDING,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"1q2w": proto.Job{
					Id:    "1q2w",
					Type:  "dummy",
					State: proto.STATE_PENDING,
				},
			},
			State: proto.STATE_PENDING,
		},
	}

	// //////////////////////////////////////////////////////////////////////////
	// Running request
	// //////////////////////////////////////////////////////////////////////////
	reqId = "454ae2f98a05cv16sdwt"
	curTime, _ = time.Parse(time.RFC3339, "2017-09-13T01:00:00Z")
	jrHost := "fake_jr_host"

	// Graph for this chain looks like this:
	//       -------> ldfi ------->
	//     /                        \
	//   di12 --> 590s --> g012 --> pzi8
	//     \                        /
	//       -------> 9sa1 ------->
	//
	SavedRequests[reqId] = proto.Request{
		Id:        reqId,
		Type:      "do-something",
		CreatedAt: curTime,
		State:     proto.STATE_RUNNING,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"di12": proto.Job{ // job1
					Id:    "di12",
					Type:  "fake",
					State: proto.STATE_COMPLETE,
				},
				"ldfi": proto.Job{ // job2
					Id:    "ldfi",
					Type:  "fake",
					State: proto.STATE_RUNNING,
				},
				"590s": proto.Job{ // job3
					Id:    "590s",
					Type:  "fake",
					State: proto.STATE_COMPLETE,
				},
				"g012": proto.Job{ // job4
					Id:    "g012",
					Type:  "fake",
					State: proto.STATE_PENDING,
				},
				"9sa1": proto.Job{ // job5
					Id:    "9sa1",
					Type:  "fake",
					State: proto.STATE_FAIL,
				},
				"pzi8": proto.Job{ // job6
					Id:    "pzi8",
					Type:  "fake",
					State: proto.STATE_PENDING,
				},
			},
			AdjacencyList: map[string][]string{
				"di12": []string{"ldfi", "590s", "9sa1"},
				"ldfi": []string{"pzi8"},
				"590s": []string{"g012"},
				"g012": []string{"pzi8"},
				"9sa1": []string{"pzi8"},
			},
			State: proto.STATE_RUNNING,
		},
		FinishedJobs:  4,
		JobRunnerHost: jrHost,
	}

	// //////////////////////////////////////////////////////////////////////////
	// Suspended request with SJC
	// //////////////////////////////////////////////////////////////////////////
	reqId = "suspended___________"
	curTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:00:00Z")
	startTime, _ := time.Parse(time.RFC3339, "2017-09-13T03:01:00Z")
	SavedRequests[reqId] = proto.Request{
		Id:        reqId,
		Type:      "do-another-thing",
		CreatedAt: curTime,
		StartedAt: &startTime,
		State:     proto.STATE_SUSPENDED,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_PENDING,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_PENDING, // jc in request is never updated
		},
	}
	SavedSJCs[reqId] = proto.SuspendedJobChain{
		RequestId:         reqId,
		TotalJobTries:     map[string]uint{"hw48": 5},
		LatestRunJobTries: map[string]uint{"hw48": 2},
		SequenceTries:     map[string]uint{"hw48": 1},
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_STOPPED,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_SUSPENDED,
		},
	}

	// //////////////////////////////////////////////////////////////////////////
	// Running request with an old SJC (unclaimed by an RM + suspended long ago)
	// //////////////////////////////////////////////////////////////////////////
	reqId = "running_with_old_sjc"
	curTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:00:00Z")
	startTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:01:00Z")
	SavedRequests[reqId] = proto.Request{
		Id:        reqId,
		Type:      "do-another-thing",
		CreatedAt: curTime,
		StartedAt: &startTime,
		State:     proto.STATE_RUNNING, // running
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_PENDING,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_PENDING, // jc in request is never updated
		},
	}
	SavedSJCs[reqId] = proto.SuspendedJobChain{
		RequestId:         reqId,
		TotalJobTries:     map[string]uint{"hw48": 5},
		LatestRunJobTries: map[string]uint{"hw48": 2},
		SequenceTries:     map[string]uint{"hw48": 1},
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_STOPPED,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_SUSPENDED,
		},
	}

	// //////////////////////////////////////////////////////////////////////////
	// Suspended Request + Abandoned SJC (claimed by an RM + not updated recently)
	// //////////////////////////////////////////////////////////////////////////
	reqId = "abandoned_sjc_______"
	curTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:00:00Z")
	startTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:01:00Z")
	SavedRequests[reqId] = proto.Request{
		Id:        reqId,
		Type:      "do-another-thing",
		CreatedAt: curTime,
		StartedAt: &startTime,
		State:     proto.STATE_SUSPENDED,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_PENDING,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_PENDING, // jc in request is never updated
		},
	}
	SavedSJCs[reqId] = proto.SuspendedJobChain{
		RequestId:         reqId,
		TotalJobTries:     map[string]uint{"hw48": 5},
		LatestRunJobTries: map[string]uint{"hw48": 2},
		SequenceTries:     map[string]uint{"hw48": 1},
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_STOPPED,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_SUSPENDED,
		},
	}

	// //////////////////////////////////////////////////////////////////////////
	// Running Request + Abandoned SJC (claimed by an RM + not updated recently)
	// //////////////////////////////////////////////////////////////////////////
	reqId = "running_abandoned___"
	curTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:00:00Z")
	startTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:01:00Z")
	SavedRequests[reqId] = proto.Request{
		Id:        reqId,
		Type:      "do-another-thing",
		CreatedAt: curTime,
		StartedAt: &startTime,
		State:     proto.STATE_SUSPENDED,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_PENDING,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_PENDING, // jc in request is never updated
		},
	}
	SavedSJCs[reqId] = proto.SuspendedJobChain{
		RequestId:         reqId,
		TotalJobTries:     map[string]uint{"hw48": 5},
		LatestRunJobTries: map[string]uint{"hw48": 2},
		SequenceTries:     map[string]uint{"hw48": 1},
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_STOPPED,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_SUSPENDED,
		},
	}

	// //////////////////////////////////////////////////////////////////////////
	// Suspended Request + Old SJC (unclaimed by an RM + suspended long ago)
	// //////////////////////////////////////////////////////////////////////////
	reqId = "old_sjc_____________"
	curTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:00:00Z")
	startTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:01:00Z")
	SavedRequests[reqId] = proto.Request{
		Id:        reqId,
		Type:      "do-another-thing",
		CreatedAt: curTime,
		StartedAt: &startTime,
		State:     proto.STATE_SUSPENDED,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_PENDING,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_PENDING, // jc in request is never updated
		},
	}
	SavedSJCs[reqId] = proto.SuspendedJobChain{
		RequestId:         reqId,
		TotalJobTries:     map[string]uint{"hw48": 5},
		LatestRunJobTries: map[string]uint{"hw48": 2},
		SequenceTries:     map[string]uint{"hw48": 1},
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_STOPPED,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_SUSPENDED,
		},
	}

	// //////////////////////////////////////////////////////////////////////////
	// Suspended Request + Old SJC (CLAIMED by an RM + suspended long ago)
	// //////////////////////////////////////////////////////////////////////////
	reqId = "abandoned_old_sjc___"
	curTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:00:00Z")
	startTime, _ = time.Parse(time.RFC3339, "2017-09-13T03:01:00Z")
	SavedRequests[reqId] = proto.Request{
		Id:        reqId,
		Type:      "do-another-thing",
		CreatedAt: curTime,
		StartedAt: &startTime,
		State:     proto.STATE_SUSPENDED,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_PENDING,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_PENDING, // jc in request is never updated
		},
	}
	SavedSJCs[reqId] = proto.SuspendedJobChain{
		RequestId:         reqId,
		TotalJobTries:     map[string]uint{"hw48": 5},
		LatestRunJobTries: map[string]uint{"hw48": 2},
		SequenceTries:     map[string]uint{"hw48": 1},
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_STOPPED,
					Retry:         5,
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_SUSPENDED,
		},
	}

	// //////////////////////////////////////////////////////////////////////////
	// Completed request
	// //////////////////////////////////////////////////////////////////////////
	reqId = "93ec156e204ety45sgf0"
	curTime, _ = time.Parse(time.RFC3339, "2017-09-13T02:00:00Z")
	SavedRequests[reqId] = proto.Request{
		Id:        reqId,
		Type:      "something-else",
		CreatedAt: curTime,
		State:     proto.STATE_COMPLETE,
	}

	// //////////////////////////////////////////////////////////////////////////
	// Assortment of jls
	// //////////////////////////////////////////////////////////////////////////
	reqId = "fa0d862f16casg200lkf"
	SavedJLs[reqId] = []proto.JobLog{
		proto.JobLog{
			RequestId: reqId,
			JobId:     "k238",
			Name:      "j1",
			Try:       0,
			Type:      "test",
			State:     proto.STATE_PENDING,
		},
		proto.JobLog{
			RequestId: reqId,
			JobId:     "fndu",
			Name:      "j2",
			Try:       0,
			Type:      "test",
			State:     proto.STATE_RUNNING,
		},
		proto.JobLog{
			RequestId: reqId,
			JobId:     "g89d",
			Name:      "j4",
			Try:       0,
			Type:      "test",
			State:     proto.STATE_FAIL,
		},
		proto.JobLog{
			RequestId: reqId,
			JobId:     "g89d",
			Name:      "j3",
			Try:       1,
			Type:      "test",
			State:     proto.STATE_COMPLETE,
		},
	}

	// //////////////////////////////////////////////////////////////////////////
	// Job chain
	// //////////////////////////////////////////////////////////////////////////
	reqId = "8bff5def4f3fvh78skjy"
	SavedJCs[reqId] = proto.JobChain{
		RequestId: reqId,
		Jobs: map[string]proto.Job{
			"vr34": proto.Job{
				Id:    "vr34",
				Type:  "noop",
				State: proto.STATE_FAIL,
			},
		},
		State: proto.STATE_FAIL,
	}
}
