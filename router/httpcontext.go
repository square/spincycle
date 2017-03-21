// Copyright 2017, Square, Inc.

package router

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
)

// HTTPContext is an object that is passed around during the handling of the request.
type HTTPContext struct {
	Response  http.ResponseWriter // HTTP Response object.
	Request   *http.Request       // HTTP Request object.
	Arguments []string            // Arguments matched by the wildcard portions ({}) in the URL pattern.
	router    *Router
}

// WriteJSON writes a response as the JSON object.
func (ctx HTTPContext) WriteJSON(v interface{}) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(ctx.Response, "Error while encoding JSON.", http.StatusInternalServerError)
		return err
	}
	_, err = ctx.Response.Write(out)
	if err != nil {
		http.Error(ctx.Response, "Error whie writing the error", http.StatusInternalServerError)
		return err
	}
	return nil
}

// APIError writes a custom error message in the JSON format.
func (ctx HTTPContext) APIError(errorType, message string, messageArgs ...interface{}) {
	ctx.Response.Header().Set("Content-Type", "application/json")

	errorCode, ok := errorCodes[errorType]
	if !ok {
		log.Errorf("Unknown error type: %v", errorType)
		errorType = ErrInternal
		errorCode = http.StatusInternalServerError
	}
	formatted := fmt.Sprintf(message, messageArgs...)
	ctx.Response.WriteHeader(errorCode)
	ctx.WriteJSON(ErrorResponse{formatted, errorType})
}

// UnsupportedAPIMethod writes an error message regarding unsupported HTTP method.
func (ctx HTTPContext) UnsupportedAPIMethod() {
	ctx.APIError(ErrBadRequest, "Unsupported method %s.", ctx.Request.Method)
}
