// Package client provides an HTTP client for interacting with the Job Runner (JR) API.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/square/spincycle/proto"
)

// A JRClient is an HTTP client used for interacting with the JR API.
type JRClient interface {
	// NewJobChain takes a job chain and sends it to the JR.
	NewJobChain(proto.JobChain) error
	// StartRequest starts the job chain that corresponds to a given request Id.
	StartRequest(uint) error
	// StopRequest stops the job chain that corresponds to a given request Id.
	StopRequest(uint) error
	// RequestStatus gets the status of the job chain that corresponds to a given request Id.
	RequestStatus(uint) (*proto.JobChainStatus, error)
}

type jrClient struct {
	*http.Client
	baseUrl string
}

// NewJRClient takes an http.Client and base API URL and creates a new jrClient.
func NewJRClient(c *http.Client, baseUrl string) *jrClient {
	return &jrClient{
		Client:  c,
		baseUrl: baseUrl,
	}
}

func (c *jrClient) NewJobChain(jobChain proto.JobChain) error {
	// POST /api/v1/job-chains
	httpMethod := "POST"
	url := c.baseUrl + "/api/v1/job-chains"
	successStatusResp := 200

	// Create the payload.
	payloadBytes, err := json.Marshal(jobChain)
	if err != nil {
		return err
	}

	// Create the request.
	req, err := http.NewRequest(httpMethod, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	// Send the request.
	resp, body, err := c.do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != successStatusResp {
		return fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	return nil
}

func (c *jrClient) StartRequest(requestId uint) error {
	// PUT /api/v1/job-chains/${requestId}/start
	httpMethod := "PUT"
	url := c.baseUrl + "/api/v1/job-chains/" + strconv.FormatUint(uint64(requestId), 10) + "/start"
	successStatusResp := 200

	// Create the request.
	req, err := http.NewRequest(httpMethod, url, nil)
	if err != nil {
		return err
	}

	// Send the request.
	resp, body, err := c.do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != successStatusResp {
		return fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	return nil
}

func (c *jrClient) StopRequest(requestId uint) error {
	// PUT /api/v1/job-chains/${requestId}/stop
	httpMethod := "PUT"
	url := c.baseUrl + "/api/v1/job-chains/" + strconv.FormatUint(uint64(requestId), 10) + "/stop"
	successStatusResp := 200

	// Create the request.
	req, err := http.NewRequest(httpMethod, url, nil)
	if err != nil {
		return err
	}

	// Send the request.
	resp, body, err := c.do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != successStatusResp {
		return fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}
	return nil
}

func (c *jrClient) RequestStatus(requestId uint) (*proto.JobChainStatus, error) {
	// GET /api/v1/job-chains/${requestId}/status
	httpMethod := "GET"
	url := c.baseUrl + "/api/v1/job-chains/" + strconv.FormatUint(uint64(requestId), 10) + "/status"
	successStatusResp := 200

	// Create the request.
	req, err := http.NewRequest(httpMethod, url, nil)
	if err != nil {
		return nil, err
	}

	// Send the request.
	resp, body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != successStatusResp {
		return nil, fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	// Unmarshal the response.
	status := &proto.JobChainStatus{}
	err = json.Unmarshal(body, status)
	if err != nil {
		return nil, err
	}

	return status, nil
}

// ------------------------------------------------------------------------- //

func (c *jrClient) do(req *http.Request) (*http.Response, []byte, error) {
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("http.Client.Do: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("ioutil.ReadAll: %s", err)
	}
	return resp, body, nil
}
