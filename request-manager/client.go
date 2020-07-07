// Copyright 2017-2019, Square, Inc.

// Package rm provides an HTTP client for interacting with the Request Manager (RM) API.
package rm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/square/spincycle/v2/proto"
)

// A Client is an HTTP client used for interacting with the RM API.
type Client interface {
	// CreateRequest takes parameters for a new request, creates the request,
	// and returns the request's id.
	CreateRequest(string, map[string]interface{}) (string, error)

	// GetRequest takes a request id and returns the corresponding request.
	GetRequest(string) (proto.Request, error)

	// FindRequests takes a request filter and returns a list of requests
	// matching the filter conditions, in descending order by create time
	// (i.e. most recent first).
	FindRequests(proto.RequestFilter) ([]proto.Request, error)

	// StartRequest takes a request id and starts the corresponding request
	// (by sending it to the job runner).
	StartRequest(string) error

	// FinishRequest marks a request as finished.
	FinishRequest(proto.FinishRequest) error

	// StopRequest takes a request id and stops the corresponding request.
	// If the request is not running, it returns an error.
	StopRequest(string) error

	// SuspendRequest takes a request id and a SuspendedJobChain and suspends the
	// corresponding request. It marks the request's state as suspended and saves
	// the SuspendedJobChain.
	SuspendRequest(string, proto.SuspendedJobChain) error

	// GetJobChain gets the job chain for a given request id.
	GetJobChain(string) (proto.JobChain, error)

	// GetJL gets the job log of the given request ID.
	GetJL(string) ([]proto.JobLog, error)

	// CreateJL creates a JL for a given request id.
	CreateJL(string, proto.JobLog) error

	// RequestList returns a list of possible requests.
	RequestList() ([]proto.RequestSpec, error)

	// Running returns a list of running jobs, sorted by runtime.
	Running(proto.StatusFilter) (proto.RunningStatus, error)

	// UpdateProgress updates request progress from Job Runner.
	UpdateProgress(proto.RequestProgress) error
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
	if err := c.makeRequest("POST", url, reqParams, &req); err != nil {
		return "", err
	}

	return req.Id, nil
}

func (c *client) GetRequest(requestId string) (proto.Request, error) {
	// GET /api/v1/requests/${requestId}
	url := c.baseUrl + "/api/v1/requests/" + requestId

	var req proto.Request
	err := c.makeRequest("GET", url, nil, &req)
	return req, err
}

func (c *client) FindRequests(filter proto.RequestFilter) ([]proto.Request, error) {
	// GET /api/v1/requests
	url := c.baseUrl + "/api/v1/requests/?" + filter.String()

	var requests []proto.Request
	err := c.makeRequest("GET", url, nil, &requests)
	return requests, err
}

func (c *client) StartRequest(requestId string) error {
	// PUT /api/v1/requests/${requestId}/start
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/start"

	return c.makeRequest("PUT", url, nil, nil)
}

func (c *client) FinishRequest(fr proto.FinishRequest) error {
	// PUT /api/v1/requests/${requestId}/finish
	url := c.baseUrl + "/api/v1/requests/" + fr.RequestId + "/finish"
	if fr.RequestId == "" {
		return fmt.Errorf("RequestId is empty")
	}
	if _, ok := proto.StateName[fr.State]; !ok {
		return fmt.Errorf("invalid State value: %d", fr.State)
	}
	if fr.FinishedAt.IsZero() {
		return fmt.Errorf("FinishedAt is zero")
	}
	return c.makeRequest("PUT", url, fr, nil)
}

func (c *client) StopRequest(requestId string) error {
	// PUT /api/v1/requests/${requestId}/stop
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/stop"

	return c.makeRequest("PUT", url, nil, nil)
}

func (c *client) SuspendRequest(requestId string, sjc proto.SuspendedJobChain) error {
	// PUT /api/v1/requests/${requestId}/suspend
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/suspend"

	return c.makeRequest("PUT", url, sjc, nil)
}

func (c *client) GetJobChain(requestId string) (proto.JobChain, error) {
	// GET /api/v1/requests/${requestId}/job-chain
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/job-chain"

	var jc proto.JobChain
	err := c.makeRequest("GET", url, nil, &jc)
	return jc, err
}

func (c *client) GetJL(requestId string) ([]proto.JobLog, error) {
	// GET /api/v1/requests/${requestId}/log
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/log"

	var jl []proto.JobLog
	err := c.makeRequest("GET", url, nil, &jl)
	return jl, err
}

func (c *client) CreateJL(requestId string, jl proto.JobLog) error {
	// POST /api/v1/requests/${requestId}/log
	url := c.baseUrl + "/api/v1/requests/" + requestId + "/log"

	return c.makeRequest("POST", url, jl, nil)
}

func (c *client) RequestList() ([]proto.RequestSpec, error) {
	// GET /api/v1/requests
	url := c.baseUrl + "/api/v1/request-list"
	var req []proto.RequestSpec
	err := c.makeRequest("GET", url, nil, &req)
	return req, err
}

func (c *client) Running(f proto.StatusFilter) (proto.RunningStatus, error) {
	// GET /api/v1/requests
	url := c.baseUrl + "/api/v1/status/running" + f.String()
	var req proto.RunningStatus
	err := c.makeRequest("GET", url, nil, &req)
	return req, err
}

func (c *client) UpdateProgress(prg proto.RequestProgress) error {
	// GET /api/v1/requests/${requestId}/status
	url := c.baseUrl + "/api/v1/requests/" + prg.RequestId + "/progress"
	return c.makeRequest("PUT", url, prg, nil)
}

// ------------------------------------------------------------------------- //

// makeRequest is a helper function for making HTTP requests. The httpVerb, url,
// and expectedStatusCode arguments are self explanatory. If the payloadStruct
// argument is provided (if it's not nil), the struct will be marshalled into
// JSON and sent as the payload of the request. If the respStruct argument is
// provided (if it's not nil), the response body of the request will be
// unmarshalled into the struct pointed to by it.
func (c *client) makeRequest(httpVerb, url string, payloadStruct interface{}, respStruct interface{}) error {
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

	// Success if status 200 or 201. Else it should be a proto.Error message with
	// a helpful error message. The err returned here will most likely be reported
	// verbatim by the client (e.g. spinc), so it's important to make it clear.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if len(body) == 0 {
			// If there's no response body, then the API probably crashed and
			// the status code is probably 500
			return fmt.Errorf("no response from API, check logs (HTTP status %d)", resp.StatusCode)
		}
		var perr proto.Error
		err := json.Unmarshal(body, &perr)
		if err == nil && perr.Message != "" {
			if resp.StatusCode == http.StatusNotFound {
				// 404s aren't API errors, so just report the "not found" error message as-is
				return perr
			} else {
				// This can be anything from 500 errors on db error, or 401 errors
				// if caller sends bad data
				return fmt.Errorf("API error: %s (HTTP status %d)", perr, resp.StatusCode)
			}
		} else {
			// If proto.Error.Message is empty, the API probably crashed and maybe
			// the framework (Echo) sent something else. Dump whatever content body
			// we have; it probably has some info about the error.
			return fmt.Errorf("API error: %s (HTTP status %d)", string(body), resp.StatusCode)
		}
	}

	// Unmarshal the body into the struct pointed to by the respStruct argument.
	if respStruct != nil {
		if err = json.Unmarshal(body, respStruct); err != nil {
			return err
		}
	}

	return nil
}
