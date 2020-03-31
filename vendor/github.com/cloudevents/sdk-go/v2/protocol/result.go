package protocol

import (
	"errors"
	"fmt"
)

// Result leverages go's 1.13 error wrapping.
type Result error

// Is reports whether any error in err's chain matches target.
//
// The chain consists of err itself followed by the sequence of errors obtained by
// repeatedly calling Unwrap.
//
// An error is considered to match a target if it is equal to that target or if
// it implements a method Is(error) bool such that Is(target) returns true.
// (text from errors/wrap.go)
var ResultIs = errors.Is

// As finds the first error in err's chain that matches target, and if so, sets
// target to that error value and returns true.
//
// The chain consists of err itself followed by the sequence of errors obtained by
// repeatedly calling Unwrap.
//
// An error matches target if the error's concrete value is assignable to the value
// pointed to by target, or if the error has a method As(interface{}) bool such that
// As(target) returns true. In the latter case, the As method is responsible for
// setting target.
//
// As will panic if target is not a non-nil pointer to either a type that implements
// error, or to any interface type. As returns false if err is nil.
// (text from errors/wrap.go)
var ResultAs = errors.As

func NewResult(messageFmt string, args ...interface{}) Result {
	return fmt.Errorf(messageFmt, args...) // TODO: look at adding ACK/Nak support.
}

func IsACK(target Result) bool {
	// special case, nil target also means ACK.
	if target == nil {
		return true
	}

	return ResultIs(target, ResultACK)
}

func IsNACK(target Result) bool {
	return ResultIs(target, ResultNACK)
}

var (
	ResultACK  = NewReceipt(true, "")
	ResultNACK = NewReceipt(false, "")
)

// NewReceipt returns a fully populated protocol Receipt that should be used as
// a transport.Result. This type holds the base ACK/NACK results.
func NewReceipt(ack bool, messageFmt string, args ...interface{}) Result {
	return &Receipt{
		ACK:    ack,
		Format: messageFmt,
		Args:   args,
	}
}

// Receipt wraps the fields required to understand if a protocol event is acknowledged.
type Receipt struct {
	ACK    bool
	Format string
	Args   []interface{}
}

// make sure Result implements error.
var _ error = (*Receipt)(nil)

// Is returns if the target error is a Result type checking target.
func (e *Receipt) Is(target error) bool {
	if o, ok := target.(*Receipt); ok {
		if e.ACK == o.ACK {
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
func (e *Receipt) Error() string {
	return fmt.Sprintf(e.Format, e.Args...)
}
