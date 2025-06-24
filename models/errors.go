package models

type DatabaseError struct {
	Op  string
	Msg string
	Err error
}

func (e *DatabaseError) Error() string {
	if e.Err != nil {
		return e.Op + ": " + e.Msg + ": " + e.Err.Error()
	}
	return e.Op + ": " + e.Msg
}
