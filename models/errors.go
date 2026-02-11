package models

// OperationError represents an error that occurred during a database or docker operation
type OperationError struct {
	Op  string
	Msg string
	Err error
}

func (e *OperationError) Error() string {
	if e.Err != nil {
		return e.Op + ": " + e.Msg + ": " + e.Err.Error()
	}
	return e.Op + ": " + e.Msg
}

// DatabaseError is deprecated, use OperationError instead
type DatabaseError = OperationError
