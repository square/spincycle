// Copyright 2017-2019, Square, Inc.

// Package rm provides an HTTP client for interacting with the Request Manager (RM) API.
package rm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/square/spincycle/proto"
)

// A Client is an HTTP client used for interacting with the RM API.
type Client interface {
	// CreateRequest takes parameters for a new request, creates the request,
	// and returns the request's id.
	CreateRequest(string, map[string]interface{}) (string, error)

	// GetRequest takes a request id and returns the corresponding request.
	GetRequest(string) (proto.Request, error)

	// StartRequest takes a request id and starts the corresponding request
	// (by sending it to the job runner).
	StartRequest(string) error

	// FinishRequest takes a request id, a state, and a finish time, and marks the
	// corresponding request as being finished with the provided state, at the
	// provided time.
	FinishRequest(string, byte, time.Time) error

	// StopRequest takes a request id and stops the corresponding request.
	// If the request is not running, it returns an error.
	StopRequest(string) error

	// SuspendRequest takes a request id and a SuspendedJobChain and suspends the
	// corresponding request. It marks the request's state as suspended and saves
	// the SuspendedJobChain.
	SuspendRequest(string, proto.SuspendedJobChain) error

	// RequestStatus gets the status of a request that corresponds to a
	// given request id.
	RequestStatus(string) (proto.RequestStatus, error)

	// GetJobChain gets the job chain for a given request id.
	GetJobChain(string) (proto.JobChain, error)

	// GetJL gets the job log of the given request ID.
	GetJL(string) ([]proto.JobLog, error)

	// CreateJL creates a JL for a given request id.
	CreateJL(string, proto.JobLog) error

	// RequestList returns a list of possible requests.
	RequestList() ([]proto.RequestSpec, error)

	// SysStatRunning returns a list of running jobs, sorted by runtime.
	SysStatRunning() (proto.RunningStatus, error)
}

type client struct {
	*http.Client
	baseUrl string
}

// NewClient takes an http.Client and base API URL and creates a Client.
func NewClient(c *http.Client, baseUrl string) Client {
	return &client{
		Client:  c,
		baseUrl: baseUrl,
	}
}

func (c *client) CreateRequest(reqType string, args map[string]interface{}) (string, error) {
	// POST /api/v1/requests
	url := c.baseUrl + "/api/v1/requests"

	// Create the payload struct.
	reqParams := &proto.CreateRequest{
		Type: reqType,
		Args: args,
	}

	var req proto.Request
	if err := c.makeRequest("POST", url, reqParams, http.StatusCreated, &req); err != nil {
		return "", err
	}

	return req.Id, nil
}

func (c *client) GetRequest(requestId string) (proto.Request, error) {
	// GET /api/v1/requests/${requestId}
	url := c.baseUrl + "/api/v1/requests/" + requestId

	var req proto.Request
	err := c.makeRequest("GET", url, nil, http.StatusOK, &req)
	return req, err
}

func (c *client) StartRequest(requestId string) error {
	// PUT /api/v1/requests/${requestId}/start
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/start"

	return c.makeRequest("PUT", url, nil, http.StatusOK, nil)
}

func (c *client) FinishRequest(requestId string, state byte, finishedAt time.Time) error {
	// PUT /api/v1/requests/${requestId}/finish
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/finish"

	// Create the payload struct.
	finishParams := proto.FinishRequest{
		State:      state,
		FinishedAt: finishedAt,
	}

	return c.makeRequest("PUT", url, finishParams, http.StatusOK, nil)
}

func (c *client) StopRequest(requestId string) error {
	// PUT /api/v1/requests/${requestId}/stop
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/stop"

	return c.makeRequest("PUT", url, nil, http.StatusOK, nil)
}

func (c *client) SuspendRequest(requestId string, sjc proto.SuspendedJobChain) error {
	// PUT /api/v1/requests/${requestId}/suspend
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/suspend"

	return c.makeRequest("PUT", url, sjc, http.StatusOK, nil)
}

func (c *client) RequestStatus(requestId string) (proto.RequestStatus, error) {
	// GET /api/v1/requests/${requestId}/status
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/status"

	var status proto.RequestStatus
	err := c.makeRequest("GET", url, nil, http.StatusOK, &status)
	return status, err
}

func (c *client) GetJobChain(requestId string) (proto.JobChain, error) {
	// GET /api/v1/requests/${requestId}/job-chain
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/job-chain"

	var jc proto.JobChain
	err := c.makeRequest("GET", url, nil, http.StatusOK, &jc)
	return jc, err
}

func (c *client) GetJL(requestId string) ([]proto.JobLog, error) {
	// GET /api/v1/requests/${requestId}/log
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/log"

	var jl []proto.JobLog
	err := c.makeRequest("GET", url, nil, http.StatusOK, &jl)
	return jl, err
}

func (c *client) CreateJL(requestId string, jl proto.JobLog) error {
	// POST /api/v1/requests/${requestId}/log
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/log"

	return c.makeRequest("POST", url, jl, http.StatusCreated, nil)
}

func (c *client) RequestList() ([]proto.RequestSpec, error) {
	// GET /api/v1/requests
	url := c.baseUrl + "/api/v1/request-list"
	var req []proto.RequestSpec
	err := c.makeRequest("GET", url, nil, http.StatusOK, &req)
	return req, err
}

func (c *client) SysStatRunning() (proto.RunningStatus, error) {
	// GET /api/v1/requests
	url := c.baseUrl + "/api/v1/status/running"
	var req proto.RunningStatus
	err := c.makeRequest("GET", url, nil, http.StatusOK, &req)
	return req, err
}

// ------------------------------------------------------------------------- //

// makeRequest is a helper function for making HTTP requests. The httpVerb, url,
// and expectedStatusCode arguments are self explanatory. If the payloadStruct
// argument is provided (if it's not nil), the struct will be marshalled into
// JSON and sent as the payload of the request. If the respStruct argument is
// provided (if it's not nil), the response body of the request will be
// unmarshalled into the struct pointed to by it.
func (c *client) makeRequest(httpVerb, url string, payloadStruct interface{}, expectedStatusCode int, respStruct interface{}) error {
	// Marshal payload.
	var payload []byte
	var err error
	if payloadStruct != nil {
		payload, err = json.Marshal(payloadStruct)
		if err != nil {
			return err
		}
	}

	// Create the request.
	req, err := http.NewRequest(httpVerb, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	// Send the request.
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read the response body.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Check the status code.
	if resp.StatusCode != expectedStatusCode {
		return fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	// Unmarshal the body into the struct pointed to by the respStruct argument.
	if respStruct != nil {
		if err = json.Unmarshal(body, respStruct); err != nil {
			return err
		}
	}

	return nil
}
