package services

import "fmt"

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	Status  int
	Message string
	Cause   error
}

func (e *HTTPError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// BadRequest creates a 400 error
func BadRequest(format string, args ...interface{}) error {
	return &HTTPError{
		Status:  400,
		Message: fmt.Sprintf(format, args...),
	}
}

// NotFound creates a 404 error
func NotFound(resource string) error {
	return &HTTPError{
		Status:  404,
		Message: fmt.Sprintf("%s not found", resource),
	}
}

// InternalError creates a 500 error
func InternalError(cause error, message string) error {
	return &HTTPError{
		Status:  500,
		Message: message,
		Cause:   cause,
	}
}