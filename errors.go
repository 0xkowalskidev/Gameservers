package main

import (
	"fmt"
	"net/http"

	apperrors "0xkowalskidev/gameservers/errors"
	"github.com/rs/zerolog/log"
)

// Re-export types for backward compatibility
type AppError = apperrors.AppError
type HTTPError = apperrors.HTTPError

// Re-export common errors
var (
	ErrNotFound         = apperrors.ErrNotFound
	ErrBadRequest       = apperrors.ErrBadRequest
	ErrInternalServer   = apperrors.ErrInternalServer
	ErrMethodNotAllowed = apperrors.ErrMethodNotAllowed
)

// Re-export constructors
var (
	BadRequest      = apperrors.BadRequest
	NotFound        = apperrors.NotFound
	InternalError   = apperrors.InternalError
	NewError        = apperrors.NewError
	WrapError       = apperrors.WrapError
	DatabaseError   = apperrors.DatabaseError
	DockerError     = apperrors.DockerError
	RequireMethod   = apperrors.RequireMethod
	ParseForm       = apperrors.ParseForm
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


// LogAndRespond logs an info message and writes a response
func LogAndRespond(w http.ResponseWriter, status int, message string, args ...interface{}) {
	msg := fmt.Sprintf(message, args...)
	log.Info().Int("status", status).Msg(msg)
	w.WriteHeader(status)
	fmt.Fprint(w, msg)
}
