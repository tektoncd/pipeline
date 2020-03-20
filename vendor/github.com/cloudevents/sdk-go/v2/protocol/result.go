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
	return fmt.Errorf(messageFmt, args...) // TODO: look at adding Ack/Nak support.
}
