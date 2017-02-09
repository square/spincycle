// Copyright 2017, Square, Inc.

package router

import (
	"net/http"
)

// ErrorResponse is a structured error returned in HTTP endpoints returning JSON.
type ErrorResponse struct {
	Message string `json:"message"` // human-readable message.
	Type    string `json:"type"`    // enum-like object.
}

// Common error code enums
const (
	ErrNotFound     = "not_found"
	ErrMissingParam = "bad_request.missing_parameter"
	ErrInvalidParam = "bad_request.invalid_parameter"
	ErrBadRequest   = "bad_request"
	ErrInternal     = "internal_server_error"
)

var errorCodes = map[string]int{
	ErrNotFound:     http.StatusNotFound,
	ErrMissingParam: http.StatusBadRequest,
	ErrInvalidParam: http.StatusBadRequest,
	ErrBadRequest:   http.StatusBadRequest,
	ErrInternal:     http.StatusInternalServerError,
}
