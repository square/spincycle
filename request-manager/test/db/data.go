// Copyright 2017, Square, Inc.

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
	SavedRequests map[string]proto.Request  // requestId => request
	SavedJLs      map[string][]proto.JobLog // requestId => []jl
	SavedJCs      map[string]proto.JobChain // requestId => jc
)

func init() {
	SavedRequests = make(map[string]proto.Request)
	SavedJLs = make(map[string][]proto.JobLog)
	SavedJCs = make(map[string]proto.JobChain)

	// //////////////////////////////////////////////////////////////////////////
	// Pending request
	// //////////////////////////////////////////////////////////////////////////
	reqId := "0874a524aa1e4561b95218a43c5c54ea"
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
	reqId = "454ae2f98a0549bcb693fa656d6f8eb5"
	curTime, _ = time.Parse(time.RFC3339, "2017-09-13T01:00:00Z")

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
		FinishedJobs: 4,
	}

	// //////////////////////////////////////////////////////////////////////////
	// Completed request
	// //////////////////////////////////////////////////////////////////////////
	reqId = "93ec156e204e4450b031259249b6992d"
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
	reqId = "fa0d862f16ca4f14a0613e2c26562de6"
	SavedJLs[reqId] = []proto.JobLog{
		proto.JobLog{
			RequestId: reqId,
			JobId:     "k238",
			Try:       0,
			Type:      "test",
			State:     proto.STATE_PENDING,
		},
		proto.JobLog{
			RequestId: reqId,
			JobId:     "fndu",
			Try:       0,
			Type:      "test",
			State:     proto.STATE_RUNNING,
		},
		proto.JobLog{
			RequestId: reqId,
			JobId:     "g89d",
			Try:       0,
			Type:      "test",
			State:     proto.STATE_FAIL,
		},
		proto.JobLog{
			RequestId: reqId,
			JobId:     "g89d",
			Try:       1,
			Type:      "test",
			State:     proto.STATE_COMPLETE,
		},
	}

	// //////////////////////////////////////////////////////////////////////////
	// Job chain
	// //////////////////////////////////////////////////////////////////////////
	reqId = "8bff5def4f3f4e429bec07723e905265"
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
