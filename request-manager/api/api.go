// Copyright 2017-2018, Square, Inc.

// Package api provides controllers for each api endpoint. Controllers are
// "dumb wiring"; there is little to no application logic in this package.
// Controllers call and coordinate other packages to satisfy the api endpoint.
package api

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/app"
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
	appCtx app.Context
	rm     request.Manager
	jls    joblog.Store
	stat   status.Manager
	// --
	echo *echo.Echo
}

// NewAPI cretes a new API struct. It initializes an echo web server within the
// struct, and registers all of the API's routes with it.
// @todo: create a struct of managers and pass that in here instead?
func NewAPI(appCtx app.Context, rm request.Manager, jls joblog.Store, stat status.Manager) *API {
	api := &API{
		appCtx: appCtx,
		rm:     rm,
		jls:    jls,
		stat:   stat,
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

	// Auth hook
	api.echo.Use((func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if appCtx.Hooks.Auth == nil {
				return next(c) // no auth
			}
			ok, err := appCtx.Hooks.Auth(c.Request())
			if err != nil {
				return err
			}
			if !ok {
				//c.Response().Header().Set(echo.HeaderWWWAuthenticate, basic+" realm="+realm)
				return echo.ErrUnauthorized // 401
			}
			return next(c) // auth OK
		}
	}))

	// SetUsername hook
	api.echo.Use((func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if appCtx.Hooks.SetUsername == nil {
				return next(c) // no auth
			}
			username, err := appCtx.Hooks.SetUsername(c.Request())
			if err != nil {
				return err
			}
			c.Set("username", username)
			return next(c)
		}
	}))

	return api
}

func (api *API) Run() error {
	if api.appCtx.Config.Server.TLS.CertFile != "" && api.appCtx.Config.Server.TLS.KeyFile != "" {
		return api.echo.StartTLS(api.appCtx.Config.Server.ListenAddress, api.appCtx.Config.Server.TLS.CertFile, api.appCtx.Config.Server.TLS.KeyFile)
	} else {
		return api.echo.Start(api.appCtx.Config.Server.ListenAddress)
	}
}

func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.echo.ServeHTTP(w, r)
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
	req, err := api.rm.Create(reqParams)
	if err != nil {
		return handleError(err)
	}

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

	if err := api.rm.Stop(reqId); err != nil {
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
// Get a list of all requets.
func (api *API) requestListHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, api.rm.Specs())
}

// GET <API_ROOT>/status/running
// Report all requests that are running
func (api *API) statusRunningHandler(c echo.Context) error {
	running, err := api.stat.Running(status.NoFilter)
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
}
