package main

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

// AppError represents a unified error type with context, status, and error chaining
type AppError struct {
	Op      string // Operation context (e.g., "create_gameserver", "list_files")
	Status  int    // HTTP status code
	Message string // User-friendly message
	Err     error  // Wrapped error
}

func (e AppError) Error() string {
	if e.Err != nil {
		if e.Op != "" {
			return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
		}
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	if e.Op != "" {
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	}
	return e.Message
}

// HTTPError is an alias for backward compatibility
type HTTPError = AppError

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

	appErr, ok := err.(AppError)
	if !ok {
		appErr = AppError{
			Op:      context,
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Err:     err,
		}
	}

	// Use context if Op is not set
	if appErr.Op == "" {
		appErr.Op = context
	}

	log.Error().
		Err(appErr.Err).
		Str("context", appErr.Op).
		Int("status", appErr.Status).
		Msg(appErr.Message)

	http.Error(w, appErr.Message, appErr.Status)
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
	return AppError{
		Status:  http.StatusBadRequest,
		Message: fmt.Sprintf(format, args...),
	}
}

// NotFound creates a not found error
func NotFound(resource string) error {
	return AppError{
		Status:  http.StatusNotFound,
		Message: fmt.Sprintf("%s not found", resource),
	}
}

// InternalError wraps an internal error
func InternalError(err error, message string) error {
	return AppError{
		Status:  http.StatusInternalServerError,
		Message: message,
		Err:     err,
	}
}

// NewError creates a new AppError with operation context
func NewError(op string, status int, message string, err error) error {
	return AppError{
		Op:      op,
		Status:  status,
		Message: message,
		Err:     err,
	}
}

// DatabaseError creates a database operation error
func DatabaseError(op, message string, err error) error {
	return AppError{
		Op:      op,
		Status:  http.StatusInternalServerError,
		Message: message,
		Err:     err,
	}
}

// DockerError creates a docker operation error  
func DockerError(op, message string, err error) error {
	return AppError{
		Op:      "docker_" + op,
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

// LogAndRespond logs an info message and writes a response
func LogAndRespond(w http.ResponseWriter, status int, message string, args ...interface{}) {
	msg := fmt.Sprintf(message, args...)
	log.Info().Int("status", status).Msg(msg)
	w.WriteHeader(status)
	fmt.Fprint(w, msg)
}
