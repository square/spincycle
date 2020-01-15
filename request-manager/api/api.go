// Copyright 2017-2019, Square, Inc.

// Package api provides controllers for each api endpoint. Controllers are
// "dumb wiring"; there is little to no application logic in this package.
// Controllers call and coordinate other packages to satisfy the api endpoint.
// Authentication and authorization happen in controllers.
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	serr "github.com/square/spincycle/errors"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/app"
	"github.com/square/spincycle/request-manager/auth"
	"github.com/square/spincycle/request-manager/joblog"
	"github.com/square/spincycle/request-manager/request"
	"github.com/square/spincycle/request-manager/status"
	v "github.com/square/spincycle/version"
)

const (
	API_ROOT = "/api/v1/"
)

var (
	// Error when Request Manager is shutting down and not starting new requests
	ErrShuttingDown = errors.New("Request Manager is shutting down - no new requests are being started")
)

// API provides controllers for endpoints it registers with a router.
// It satisfies the http.HandlerFunc interface.
type API struct {
	appCtx       app.Context
	rm           request.Manager
	sm           status.Manager
	rr           request.Resumer
	jls          joblog.Store
	shutdownChan chan struct{}
	// --
	echo *echo.Echo
}

// NewAPI creates a new API struct. It initializes an echo web server within the
// struct, and registers all of the API's routes with it.
func NewAPI(appCtx app.Context) *API {
	api := &API{
		appCtx:       appCtx,
		rm:           appCtx.RM,
		sm:           appCtx.Status,
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
	api.echo.GET(API_ROOT+"requests/:reqId", api.getRequestHandler)                // get -> proto.Request
	api.echo.PUT(API_ROOT+"requests/:reqId/start", api.startRequestHandler)        // start
	api.echo.PUT(API_ROOT+"requests/:reqId/finish", api.finishRequestHandler)      // finish
	api.echo.PUT(API_ROOT+"requests/:reqId/stop", api.stopRequestHandler)          // stop
	api.echo.PUT(API_ROOT+"requests/:reqId/suspend", api.suspendRequestHandler)    // suspend
	api.echo.PUT(API_ROOT+"requests/:reqId/progress", api.requestProgressHandler)  // progress
	api.echo.GET(API_ROOT+"requests/:reqId/job-chain", api.jobChainRequestHandler) // job chain

	// Job Log
	api.echo.POST(API_ROOT+"requests/:reqId/log", api.createJLHandler)    // create
	api.echo.GET(API_ROOT+"requests/:reqId/log", api.getFullJLHandler)    // per request
	api.echo.GET(API_ROOT+"requests/:reqId/log/:jobId", api.getJLHandler) // per job

	// Meta
	api.echo.GET(API_ROOT+"request-list", api.requestListHandler)     // request list
	api.echo.GET(API_ROOT+"status/running", api.statusRunningHandler) // running requests/jobs -> proto.RunningStatus
	api.echo.GET("/version", api.versionHandler)                      // return version.VERSION

	// //////////////////////////////////////////////////////////////////////
	// Middleware and hooks
	// //////////////////////////////////////////////////////////////////////
	api.echo.Use(middleware.Recover())
	api.echo.Use(middleware.Logger())

	// Auth plugin: authenticate caller. This is called before every route.
	// @todo: ignore OPTION requests?
	api.echo.Use((func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Spincycle-Version", v.Version())
			caller, err := appCtx.Auth.Authenticate(c.Request())
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}
			c.Set("caller", caller)
			c.Set("username", caller.Name)
			return next(c) // authenticated
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

func (api *API) Router() *echo.Echo {
	return api.echo
}

// Run makes the API listen on the configured address.
func (api *API) Run() error {
	var err error
	if api.appCtx.Config.Server.TLS.CertFile != "" && api.appCtx.Config.Server.TLS.KeyFile != "" {
		err = api.echo.StartTLS(api.appCtx.Config.Server.Addr, api.appCtx.Config.Server.TLS.CertFile, api.appCtx.Config.Server.TLS.KeyFile)
	} else {
		err = api.echo.Start(api.appCtx.Config.Server.Addr)
	}
	return err
}

// Stop stops the API when it's running. When Stop is called, Run returns
// immediately. Make sure to wait for Stop to return.
func (api *API) Stop() error {
	var err error
	if api.appCtx.Config.Server.TLS.CertFile != "" && api.appCtx.Config.Server.TLS.KeyFile != "" {
		err = api.echo.TLSServer.Shutdown(context.TODO())
	} else {
		err = api.echo.Server.Shutdown(context.TODO())
	}
	return err
}

// ServeHTTP makes the API implement the http.HandlerFunc interface.
func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.echo.ServeHTTP(w, r)
}

// POST <API_ROOT>/requests
// Create a new request and start it.
func (api *API) createRequestHandler(c echo.Context) error {
	// If Request Manager is shutting down, don't start running any new requests.
	select {
	case <-api.shutdownChan:
		return handleError(ErrShuttingDown, c)
	default:
	}

	// ----------------------------------------------------------------------
	// Make and validate request

	// Convert the payload into a proto.CreateRequest.
	var reqParams proto.CreateRequest
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
		return handleError(err, c)
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
		return handleError(err, c)
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
	req, err := api.rm.GetWithJC(reqId)
	if err != nil {
		return handleError(err, c)
	}

	// Return the request.
	return c.JSON(http.StatusOK, req)
}

// PUT <API_ROOT>/requests/{reqId}/start
// Start a request by sending it to the Job Runner.
func (api *API) startRequestHandler(c echo.Context) error {
	// If Request Manager is shutting down, don't start running any new requests.
	select {
	case <-api.shutdownChan:
		return handleError(ErrShuttingDown, c)
	default:
	}

	reqId := c.Param("reqId")

	if err := api.rm.Start(reqId); err != nil {
		return handleError(err, c)
	}

	return nil
}

// PUT <API_ROOT>/requests/{reqId}/finish
// Mark a request as being finished. The Job Runner hits this endpoint when
// it finishes running a job chain.
func (api *API) finishRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	// Convert the payload into a proto.FinishRequest.
	var finishParams proto.FinishRequest
	if err := c.Bind(&finishParams); err != nil {
		return err
	}

	if err := api.rm.Finish(reqId, finishParams); err != nil {
		return handleError(err, c)
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
		return handleError(err, c)
	}
	if err := api.appCtx.Auth.Authorize(c.Get("caller").(auth.Caller), proto.REQUEST_OP_STOP, req); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	if err := api.rm.Stop(reqId); err != nil {
		return handleError(err, c)
	}

	return nil
}

// PUT <API_ROOT>/requests/{reqId}/suspend
// Suspend a request and save its suspended job chain. The Job Runner hits this
// endpoint when suspending a job chain on shutdown.
func (api *API) suspendRequestHandler(c echo.Context) error {
	// Convert the payload into a proto.SuspendedJobChain
	var sjc proto.SuspendedJobChain
	if err := c.Bind(&sjc); err != nil {
		return err
	}

	if err := api.rr.Suspend(sjc); err != nil {
		return handleError(err, c)
	}

	return nil
}

func (api *API) requestProgressHandler(c echo.Context) error {
	reqId := c.Param("reqId")
	var prg proto.RequestProgress
	if err := c.Bind(&prg); err != nil {
		return err
	}
	// Validate
	if prg.RequestId == "" {
		errMsg := "invalid proto.StatusProgress: RequestId is empty, must be set"
		return handleError(serr.ValidationError{Message: errMsg}, c)
	}
	if prg.RequestId != reqId {
		errMsg := fmt.Sprintf("invalid proto.StatusProgress: RequestId=%s does not match request ID in URL: %s", prg.RequestId, reqId)
		return handleError(serr.ValidationError{Message: errMsg}, c)
	}
	// Update
	if err := api.sm.UpdateProgress(prg); err != nil {
		return handleError(err, c)
	}
	return c.JSON(http.StatusOK, nil)
}

// GET <API_ROOT>/requests/{reqId}/job-chain
// Get the job chain for a request.
func (api *API) jobChainRequestHandler(c echo.Context) error {
	reqId := c.Param("reqId")

	// Get the request's job chain from the rm.
	jc, err := api.rm.JobChain(reqId)
	if err != nil {
		return handleError(err, c)
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
		return handleError(err, c)
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
		return handleError(err, c)
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
		return handleError(err, c)
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
	f := proto.StatusFilter{
		RequestId: c.QueryParam("requestId"),
		OrderBy:   c.QueryParam("orderBy"),
	}
	running, err := api.sm.Running(f)
	if err != nil {
		return handleError(err, c)
	}
	return c.JSON(http.StatusOK, running)
}

func (api *API) versionHandler(c echo.Context) error {
	return c.String(http.StatusOK, v.Version())
}

// ------------------------------------------------------------------------- //

func handleError(err error, c echo.Context) error {
	ret := proto.Error{
		Message:    err.Error(),
		HTTPStatus: http.StatusInternalServerError,
	}

	switch err.(type) {
	case serr.RequestNotFound, serr.JobNotFound:
		ret.HTTPStatus = http.StatusNotFound
	case serr.ErrInvalidCreateRequest:
		ret.HTTPStatus = http.StatusBadRequest
	case serr.ValidationError:
		ret.HTTPStatus = http.StatusBadRequest
	}

	switch err {
	case ErrShuttingDown:
		ret.HTTPStatus = http.StatusServiceUnavailable
	}

	return c.JSON(ret.HTTPStatus, ret)
}
