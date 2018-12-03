// Copyright 2017-2018, Square, Inc.

// Package api provides controllers for each api endpoint. Controllers are
// "dumb wiring"; there is little to no application logic in this package.
// Controllers call and coordinate other packages to satisfy the api endpoint.
// Authentication and authorization happen in controllers.
package api

import (
	"context"
	"net/http"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/app"
	"github.com/square/spincycle/request-manager/auth"
	"github.com/square/spincycle/request-manager/db"
	"github.com/square/spincycle/request-manager/joblog"
	"github.com/square/spincycle/request-manager/request"
	"github.com/square/spincycle/request-manager/status"
)

const (
	API_ROOT = "/api/v1/"
)

// API provides controllers for endpoints it registers with a router.
// It satisfies the http.HandlerFunc interface.
type API struct {
	appCtx       app.Context
	rm           request.Manager
	rr           request.Resumer
	jls          joblog.Store
	shutdownChan chan struct{}
	// --
	echo *echo.Echo
}

// NewAPI creates a new API struct. It initializes an echo web server within the
// struct, and registers all of the API's routes with it.
// @todo: create a struct of managers and pass that in here instead?
func NewAPI(appCtx app.Context) *API {
	api := &API{
		appCtx:       appCtx,
		rm:           appCtx.RM,
		jls:          appCtx.JLS,
		rr:           appCtx.RR,
		shutdownChan: appCtx.ShutdownChan,
		// --
		echo: echo.New(),
	}

	// //////////////////////////////////////////////////////////////////////
	// Routes
	// //////////////////////////////////////////////////////////////////////

	// Request
	api.echo.POST(API_ROOT+"requests", api.createRequestHandler)                   // create
	api.echo.GET(API_ROOT+"requests/:reqId", api.getRequestHandler)                // get
	api.echo.PUT(API_ROOT+"requests/:reqId/start", api.startRequestHandler)        // start
	api.echo.PUT(API_ROOT+"requests/:reqId/finish", api.finishRequestHandler)      // finish
	api.echo.PUT(API_ROOT+"requests/:reqId/stop", api.stopRequestHandler)          // stop
	api.echo.PUT(API_ROOT+"requests/:reqId/suspend", api.suspendRequestHandler)    // suspend
	api.echo.GET(API_ROOT+"requests/:reqId/status", api.statusRequestHandler)      // status
	api.echo.GET(API_ROOT+"requests/:reqId/job-chain", api.jobChainRequestHandler) // job chain

	// Job Log
	api.echo.POST(API_ROOT+"requests/:reqId/log", api.createJLHandler)    // create
	api.echo.GET(API_ROOT+"requests/:reqId/log", api.getFullJLHandler)    // per request
	api.echo.GET(API_ROOT+"requests/:reqId/log/:jobId", api.getJLHandler) // per job

	// Meta
	api.echo.GET(API_ROOT+"request-list", api.requestListHandler)     // request list
	api.echo.GET(API_ROOT+"status/running", api.statusRunningHandler) // running requests

	// //////////////////////////////////////////////////////////////////////
	// Middleware and hooks
	// //////////////////////////////////////////////////////////////////////
	api.echo.Use(middleware.Recover())
	api.echo.Use(middleware.Logger())

	// Auth plugin: authenticate caller. This is called before every route.
	// @todo: ignore OPTION requests?
	api.echo.Use((func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			caller, err := appCtx.Auth.Authenticate(c.Request())
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}
			c.Set("caller", caller)
			c.Set("username", caller.Name)
			return next(c) // authenticed
		}
	}))

	// SetUsername hook, overrides ^
	if appCtx.Hooks.SetUsername != nil {
		api.echo.Use((func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				username, err := appCtx.Hooks.SetUsername(c.Request())
				if err != nil {
					return err
				}
				c.Set("username", username)
				return next(c)
			}
		}))
	}

	return api
}

// Run makes the API listen on the configured address.
func (api *API) Run() error {
	// Shut down the API when the Request Manager shuts down.
	doneChan := make(chan struct{})
	doneShuttingDown := make(chan struct{})
	go func() {
		select {
		case <-api.shutdownChan:
			api.shutdown()
			close(doneShuttingDown)
		case <-doneChan:
			return
		}
	}()

	var err error
	if api.appCtx.Config.Server.TLS.CertFile != "" && api.appCtx.Config.Server.TLS.KeyFile != "" {
		err = api.echo.StartTLS(api.appCtx.Config.Server.ListenAddress, api.appCtx.Config.Server.TLS.CertFile, api.appCtx.Config.Server.TLS.KeyFile)
	} else {
		err = api.echo.Start(api.appCtx.Config.Server.ListenAddress)
	}

	// If API is being shut down, wait to make sure it's done.
	select {
	case <-api.shutdownChan:
		<-doneShuttingDown
	default:
		close(doneChan) // stop the shutdown goroutine
	}
	return err
}

// ServeHTTP makes the API implement the http.HandlerFunc interface.
func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.echo.ServeHTTP(w, r)
}

func (api *API) shutdown() {
	// Shut down server.
	if api.appCtx.Config.Server.TLS.CertFile != "" && api.appCtx.Config.Server.TLS.KeyFile != "" {
		err := api.echo.TLSServer.Shutdown(context.TODO())
		if err != nil {
			log.Warnf("error when shutting down API: %s", err)
		}
	} else {
		err := api.echo.Server.Shutdown(context.TODO())
		if err != nil {
			log.Warnf("error when shutting down API: %s", err)
		}
	}
}

// POST <API_ROOT>/requests
// Create a new request and start it.
func (api *API) createRequestHandler(c echo.Context) error {
	// ----------------------------------------------------------------------
	// Make and validate request

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

	req, err := api.rm.Create(reqParams)
	if err != nil {
		return handleError(err)
	}

	// ----------------------------------------------------------------------
	// Authorize

	if err := api.appCtx.Auth.Authorize(c.Get("caller").(auth.Caller), proto.REQUEST_OP_START, req); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	// ----------------------------------------------------------------------
	// Run (non-blocking)

	// TODO(felixp): if creating the request succeeded but starting it failed,
	// mark the request as Failed. There's currently no way for a
	// user to Start a request that's already been created, so otherwise
	// the request will be Pending forever.
	if err := api.rm.Start(req.Id); err != nil {
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
	req, err := api.rm.Get(reqId)
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

	if err := api.rm.Start(reqId); err != nil {
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

	if err := api.rm.Finish(reqId, finishParams); err != nil {
		return handleError(err)
	}

	return nil
}

// PUT <API_ROOT>/requests/{reqId}/stop
// Stop a request by telling the Job Runner to stop running it. Return an error
// if the request is not running.
func (api *API) stopRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	// Authorize caller to stop request
	req, err := api.rm.Get(reqId)
	if err != nil {
		return handleError(err)
	}
	if err := api.appCtx.Auth.Authorize(c.Get("caller").(auth.Caller), proto.REQUEST_OP_STOP, req); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	if err := api.rm.Stop(reqId); err != nil {
		return handleError(err)
	}

	return nil
}

// PUT <API_ROOT>/requests/{reqId}/suspend
// Suspend a request and save its suspended job chain. The Job Runner hits this
// endpoint when suspending a job chain on shutdown.
func (api *API) suspendRequestHandler(c echo.Context) error {
	// Convert the payload into a proto.FinishRequestParams.
	var sjc proto.SuspendedJobChain
	if err := c.Bind(&sjc); err != nil {
		return err
	}

	if err := api.rr.Suspend(sjc); err != nil {
		return handleError(err)
	}

	return nil
}

// GET <API_ROOT>/requests/{reqId}/status
// Get the high-level status of a request, as well as the live status of any
// jobs that are running.
func (api *API) statusRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	reqStatus, err := api.rm.Status(reqId)
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
	jc, err := api.rm.JobChain(reqId)
	if err != nil {
		return handleError(err)
	}

	// Return the job chain.
	return c.JSON(http.StatusOK, jc)
}

// GET <API_ROOT>/requests/{reqId}/log
// Get full job log.
func (api *API) getFullJLHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	// Get the JL from the rm.
	jl, err := api.jls.GetFull(reqId)
	if err != nil {
		return handleError(err)
	}

	// Return the JL.
	return c.JSON(http.StatusOK, jl)
}

// GET <API_ROOT>/requests/{reqId}/log/{jobId}
// Get a JL.
func (api *API) getJLHandler(c echo.Context) error {
	reqId := c.Param("reqId")
	jobId := c.Param("jobId")

	// Get the JL from the rm.
	jl, err := api.jls.Get(reqId, jobId)
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
	jl, err := api.jls.Create(reqId, jl)
	if err != nil {
		return handleError(err)
	}

	// Increment the finished jobs counter if the job completed successfully.
	if jl.State == proto.STATE_COMPLETE {
		if err := api.rm.IncrementFinishedJobs(reqId); err != nil {
			return handleError(err)
		}
	}

	// Return the JL.
	return c.JSON(http.StatusCreated, jl)
}

// GET <API_ROOT>/request-list
// Get a list of all requests.
func (api *API) requestListHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, api.rm.Specs())
}

// GET <API_ROOT>/status/running
// Report all requests that are running.
func (api *API) statusRunningHandler(c echo.Context) error {
	running, err := api.appCtx.Status.Running(status.NoFilter)
	if err != nil {
		return handleError(err)
	}

	// Return the RequestStatus struct.
	return c.JSON(http.StatusOK, running)
}

// ------------------------------------------------------------------------- //

func handleError(err error) *echo.HTTPError {
	switch err.(type) {
	case db.ErrNotFound:
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case request.ErrInvalidState:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		switch err {
		case request.ErrInvalidParams, db.ErrNotUpdated, db.ErrMultipleUpdated:
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	return nil
}
