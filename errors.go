package main

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

// HTTPError represents an HTTP error with status code and message
type HTTPError struct {
	Status  int
	Message string
	Err     error
}

func (e HTTPError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Common HTTP errors
var (
	ErrNotFound         = HTTPError{Status: http.StatusNotFound, Message: "Resource not found"}
	ErrBadRequest       = HTTPError{Status: http.StatusBadRequest, Message: "Bad request"}
	ErrInternalServer   = HTTPError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	ErrMethodNotAllowed = HTTPError{Status: http.StatusMethodNotAllowed, Message: "Method not allowed"}
)

// HandleError handles errors in HTTP handlers with consistent logging and responses
func HandleError(w http.ResponseWriter, err error, context string) {
	if err == nil {
		return
	}

	httpErr, ok := err.(HTTPError)
	if !ok {
		httpErr = HTTPError{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Err:     err,
		}
	}

	log.Error().
		Err(httpErr.Err).
		Str("context", context).
		Int("status", httpErr.Status).
		Msg(httpErr.Message)

	http.Error(w, httpErr.Message, httpErr.Status)
}

// WrapError wraps an error with HTTP status and message
func WrapError(err error, status int, message string) error {
	if err == nil {
		return nil
	}
	return HTTPError{
		Status:  status,
		Message: message,
		Err:     err,
	}
}

// BadRequest creates a bad request error
func BadRequest(format string, args ...interface{}) error {
	return HTTPError{
		Status:  http.StatusBadRequest,
		Message: fmt.Sprintf(format, args...),
	}
}

// NotFound creates a not found error
func NotFound(resource string) error {
	return HTTPError{
		Status:  http.StatusNotFound,
		Message: fmt.Sprintf("%s not found", resource),
	}
}

// InternalError wraps an internal error
func InternalError(err error, message string) error {
	return HTTPError{
		Status:  http.StatusInternalServerError,
		Message: message,
		Err:     err,
	}
}

// RequireMethod validates HTTP method and returns error if not matched
func RequireMethod(r *http.Request, method string) error {
	if r.Method != method {
		return HTTPError{
			Status:  http.StatusMethodNotAllowed,
			Message: fmt.Sprintf("Method %s not allowed", r.Method),
		}
	}
	return nil
}

// ParseForm parses form data and handles errors
func ParseForm(r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return WrapError(err, http.StatusBadRequest, "Failed to parse form data")
	}
	return nil
}
