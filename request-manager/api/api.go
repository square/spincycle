// Copyright 2017, Square, Inc.

// Package api provides controllers for each api endpoint. Controllers are
// "dumb wiring"; there is little to no application logic in this package.
// Controllers call and coordinate other packages to satisfy the api endpoint.
package api

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/request"
)

const (
	API_ROOT = "/api/v1/"
)

// API provides controllers for endpoints it registers with a router.
// It satisfies the http.HandlerFunc interface.
type API struct {
	// An echo web server.
	echo *echo.Echo

	// Interface for creating and managing requests.
	rm request.Manager
}

// NewAPI cretes a new API struct. It initializes an echo web server within the
// struct, and registers all of the API's routes with it.
func NewAPI(requestManager request.Manager) *API {
	api := &API{
		echo: echo.New(),
		rm:   requestManager,
	}

	// //////////////////////////////////////////////////////////////////////
	// Routes
	// //////////////////////////////////////////////////////////////////////
	// Create a request.
	api.echo.POST(API_ROOT+"requests", api.createRequestHandler)
	// Get a request.
	api.echo.GET(API_ROOT+"requests/:reqId", api.getRequestHandler)
	// Start a request.
	api.echo.PUT(API_ROOT+"requests/:reqId/start", api.startRequestHandler)
	// Finish a request. This is how the JR tells the RM a request has finished running.
	api.echo.PUT(API_ROOT+"requests/:reqId/finish", api.finishRequestHandler)
	// Stop a request.
	api.echo.PUT(API_ROOT+"requests/:reqId/stop", api.stopRequestHandler)
	// Get the status of a request.
	api.echo.GET(API_ROOT+"requests/:reqId/status", api.statusRequestHandler)
	// Get a request's job chain.
	api.echo.GET(API_ROOT+"requests/:reqId/job-chain", api.jobChainRequestHandler)
	// Get a JL.
	api.echo.GET(API_ROOT+"requests/:reqId/log/:jobId", api.getJLHandler)
	// Create a JL.
	api.echo.POST(API_ROOT+"requests/:reqId/log", api.createJLHandler)

	return api
}

// ServeHTTP allows the API to statisfy the http.HandlerFunc interface.
func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	es := standard.WithConfig(engine.Config{})
	es.SetHandler(api.echo)
	es.ServeHTTP(w, r)
}

// Use adds middleware to the echo web server in the API. See
// https://echo.labstack.com/middleware for more details.
func (api *API) Use(middleware ...echo.MiddlewareFunc) {
	api.echo.Use(middleware...)
}

// ============================== CONTROLLERS ============================== //

// POST <API_ROOT>/requests
// Create a new request, but don't start it.
func (api *API) createRequestHandler(c echo.Context) error {
	// Convert the payload into a proto.CreateRequestParams.
	var reqParams proto.CreateRequestParams
	if err := c.Bind(&reqParams); err != nil {
		return err
	}

	// Get the username of the requestor from the context. By default, the
	// username is set in middleware in the main.go file, and it is always
	// "admin". You can change this middleware as you choose.
	reqParams.User = "?" // in case we can't get a username from the context
	if val := c.Get("username"); val != nil {
		if username, ok := val.(string); ok {
			reqParams.User = username
		}
	}

	// Create the request.
	req, err := api.rm.CreateRequest(reqParams)
	if err != nil {
		return handleError(err)
	}

	// Set the location of the request in the response header.
	locationUrl, _ := url.Parse(API_ROOT + "requests/" + req.Id)
	c.Response().Header().Set("Location", locationUrl.EscapedPath())

	// Return the request.
	req.JobChain = nil // don't include the job chain in the return
	return c.JSON(http.StatusCreated, req)
}

// GET <API_ROOT>/requests/{reqId}
// Get a request (just the high-level info; no job chain).
func (api *API) getRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	// Get the request from the rm.
	req, err := api.rm.GetRequest(reqId)
	if err != nil {
		return handleError(err)
	}

	// Return the request.
	return c.JSON(http.StatusOK, req)
}

// PUT <API_ROOT>/requests/{reqId}/start
// Start a request by sending it to the Job Runner.
func (api *API) startRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	if err := api.rm.StartRequest(reqId); err != nil {
		return handleError(err)
	}

	return nil
}

// PUT <API_ROOT>/requests/{reqId}/finish
// Mark a request as being finished. The Job Runner hits this endpoint when
// it finishes running a job chain.
func (api *API) finishRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	// Convert the payload into a proto.FinishRequestParams.
	var finishParams proto.FinishRequestParams
	if err := c.Bind(&finishParams); err != nil {
		return err
	}

	if err := api.rm.FinishRequest(reqId, finishParams); err != nil {
		return handleError(err)
	}

	return nil
}

// PUT <API_ROOT>/requests/{reqId}/stop
// Stop a request by telling the Job Runner to stop running it. Return an error
// if the request is not running.
func (api *API) stopRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	if err := api.rm.StopRequest(reqId); err != nil {
		return handleError(err)
	}

	return nil
}

// GET <API_ROOT>/requests/{reqId}/status
// Get the high-level status of a request, as well as the live status of any
// jobs that are running.
func (api *API) statusRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	reqStatus, err := api.rm.RequestStatus(reqId)
	if err != nil {
		return handleError(err)
	}

	// Return the RequestStatus struct.
	return c.JSON(http.StatusOK, reqStatus)
}

// GET <API_ROOT>/requests/{reqId}/job-chain
// Get the job chain for a request.
func (api *API) jobChainRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	// Get the request's job chain from the rm.
	jc, err := api.rm.GetJobChain(reqId)
	if err != nil {
		return handleError(err)
	}

	// Return the job chain.
	return c.JSON(http.StatusOK, jc)
}

// GET <API_ROOT>/requests/{reqId}/log/{jobId}
// Get a JL.
func (api *API) getJLHandler(c echo.Context) error {
	reqId := c.Param("reqId")
	jobId := c.Param("jobId")

	// Get the JL from the rm.
	jl, err := api.rm.GetJL(reqId, jobId)
	if err != nil {
		return handleError(err)
	}

	// Return the JL.
	return c.JSON(http.StatusOK, jl)
}

// POST <API_ROOT>/requests/{reqId}/log
// Create a JL.
func (api *API) createJLHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	// Get the JL from the payload.
	var jl proto.JobLog
	if err := c.Bind(&jl); err != nil {
		return err
	}

	// Create a JL in the rm.
	jl, err := api.rm.CreateJL(reqId, jl)
	if err != nil {
		return handleError(err)
	}

	// Return the JL.
	return c.JSON(http.StatusCreated, jl)
}

// ------------------------------------------------------------------------- //

func handleError(err error) *echo.HTTPError {
	switch err.(type) {
	case request.ErrNotFound:
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case request.ErrInvalidState:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		switch err {
		case request.ErrInvalidParams, request.ErrNotUpdated, request.ErrMultipleUpdated:
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	return nil
}
