package errors

type WarningError struct {
	msg string
}

func NewWarning(message string) *WarningError {
	return &WarningError{msg: message}
}

func (e *WarningError) Error() string {
	return e.msg
}

func Warning() bool {
	return true
}
