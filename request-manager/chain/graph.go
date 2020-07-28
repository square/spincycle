// Copyright 2020, Square, Inc.

package chain

type JobNode struct {
	// payload
	Job               job.Job                // runnable job that this graph node represents
	Name              string                 // name of node
	Args              map[string]interface{} // the args the node was created with
	Retry             uint                   // the number of times to retry a node
	RetryWait         string                 // the time to sleep between retries
	SequenceId        string                 // ID for first node in sequence
	SequenceRetry     uint                   // Number of times to retry a sequence. Only set for first node in sequence.
	SequenceRetryWait string                 // the time to sleep between sequence retries

	// fields required by graph.Node interface
	Next map[string]*Node
	Prev map[string]*Node
}

func (g JobNode) GetId() string {
	return g.Job.Id().Id
}

func (g JobNode) GetNext() map[string]*Node {
	return g.Next
}

func (g JobNode) GetPrev() map[string]*Node {
	return g.Prev
}

func (g JobNode) GetName() string {
	return g.Name
}

func (g JobNode) String() string {
	var string s
	s += fmt.Sprintf("Sequence ID: %s\\n ", vertex.SequenceId)
	s += fmt.Sprintf("Sequence Retry: %v\\n ", vertex.SequenceRetry)
	for k, v := range vertex.Args {
		s += fmt.Printf(" %s : %s \\n ", k, v)
	}
	return s
}

type JobGraph struct {
	graph.Graph // graph of JobNodes
}
