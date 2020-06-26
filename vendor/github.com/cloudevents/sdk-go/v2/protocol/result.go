package protocol

import (
	"errors"
	"fmt"
)

// Result leverages go's 1.13 error wrapping.
type Result error

// ResultIs reports whether any error in err's chain matches target.
//
// The chain consists of err itself followed by the sequence of errors obtained by
// repeatedly calling Unwrap.
//
// An error is considered to match a target if it is equal to that target or if
// it implements a method Is(error) bool such that Is(target) returns true.
// (text from errors/wrap.go)
var ResultIs = errors.Is

// ResultAs finds the first error in err's chain that matches target, and if so, sets
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

// IsACK true means the recipient acknowledged the event.
func IsACK(target Result) bool {
	// special case, nil target also means ACK.
	if target == nil {
		return true
	}

	return ResultIs(target, ResultACK)
}

// IsNACK true means the recipient did not acknowledge the event.
func IsNACK(target Result) bool {
	return ResultIs(target, ResultNACK)
}

// IsUndelivered true means the target result is not an ACK/NACK, but some other
// error unrelated to delivery not from the intended recipient. Likely target
// is an error that represents some part of the protocol is misconfigured or
// the event that was attempting to be sent was invalid.
func IsUndelivered(target Result) bool {
	return !ResultIs(target, ResultACK) && !ResultIs(target, ResultNACK)
}

var (
	ResultACK  = NewReceipt(true, "")
	ResultNACK = NewReceipt(false, "")
)

// NewReceipt returns a fully populated protocol Receipt that should be used as
// a transport.Result. This type holds the base ACK/NACK results.
func NewReceipt(ack bool, messageFmt string, args ...interface{}) Result {
	return &Receipt{
		Err: fmt.Errorf(messageFmt, args...),
		ACK: ack,
	}
}

// Receipt wraps the fields required to understand if a protocol event is acknowledged.
type Receipt struct {
	Err error
	ACK bool
}

// make sure Result implements error.
var _ error = (*Receipt)(nil)

// Is returns if the target error is a Result type checking target.
func (e *Receipt) Is(target error) bool {
	if o, ok := target.(*Receipt); ok {
		return e.ACK == o.ACK
	}
	// Allow for wrapped errors.
	return errors.Is(e.Err, target)
}

// Error returns the string that is formed by using the format string with the
// provided args.
func (e *Receipt) Error() string {
	return e.Err.Error()
}

// Unwrap returns the wrapped error if exist or nil
func (e *Receipt) Unwrap() error {
	return errors.Unwrap(e.Err)
}
