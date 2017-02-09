// Copyright 2017, Square, Inc.

// Package router provides advanced routing logic, which can be used on top of standard net/http MUX.
package router

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

const section = "([^/]*)"

// Route - A single route, matched by regex.
type Route struct {
	Name    string            // API endpoint name.
	Pattern *regexp.Regexp    // URL Path to match against.
	Handler func(HTTPContext) // Handler function.
}

// Router is a collection of routes.
type Router struct {
	Routes []Route // list of routes supported by the application.
}

// AddRoute adds an HTTP handler to the router. Any parameter {} is replacted to become
// a slash-component of the URL.
func (router *Router) AddRoute(pattern string, handler func(HTTPContext), name string) {
	processed := strings.Replace(pattern, "{}", section, -1)
	compiled := regexp.MustCompile("\\A" + processed + "/?\\z")
	router.Routes = append(router.Routes, Route{
		Name:    name,
		Pattern: compiled,
		Handler: handler,
	})
}

func (router *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if handler, _ := router.Handler(req); handler != nil {
		handler.ServeHTTP(rw, req)
		return
	}

	// Fallback
	http.NotFound(rw, req)
}

// Handler returns the HTTP handler and associated pattern for the given request.
func (router *Router) Handler(req *http.Request) (h http.Handler, pattern string) {
	for _, route := range router.Routes {
		match := route.Pattern.FindStringSubmatch(req.URL.Path)
		if len(match) != 0 {
			return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				ctx := HTTPContext{
					Response:  rw,
					Request:   req,
					Arguments: match,
					router:    router,
				}
				ctx.Request.ParseForm()
				route.Handler(ctx)
			}), route.Name
		}
	}

	return nil, ""
}

// helper function to marshal results to JSON
func marshal(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
