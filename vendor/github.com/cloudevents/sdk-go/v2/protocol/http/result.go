package http

import (
	"errors"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/protocol"
)

// NewResult returns a fully populated http Result that should be used as
// a transport.Result.
func NewResult(statusCode int, messageFmt string, args ...interface{}) protocol.Result {
	return &Result{
		StatusCode: statusCode,
		Format:     messageFmt,
		Args:       args,
	}
}

// Result wraps the fields required to make adjustments for http Responses.
type Result struct {
	StatusCode int
	Format     string
	Args       []interface{}
}

// make sure Result implements error.
var _ error = (*Result)(nil)

// Is returns if the target error is a Result type checking target.
func (e *Result) Is(target error) bool {
	if o, ok := target.(*Result); ok {
		if e.StatusCode == o.StatusCode {
			return true
		}
		return false
	}
	// Allow for wrapped errors.
	err := fmt.Errorf(e.Format, e.Args...)
	return errors.Is(err, target)
}

// Error returns the string that is formed by using the format string with the
// provided args.
func (e *Result) Error() string {
	return fmt.Sprintf("%d: %s", e.StatusCode, fmt.Sprintf(e.Format, e.Args...))
}
