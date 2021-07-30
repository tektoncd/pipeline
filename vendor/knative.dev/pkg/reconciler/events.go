/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reconciler

import (
	"errors"
	"fmt"
)

// Event leverages go's 1.13 error wrapping.
type Event error

// EventIs reports whether any error in err's chain matches target.
//
// The chain consists of err itself followed by the sequence of errors obtained by
// repeatedly calling Unwrap.
//
// An error is considered to match a target if it is equal to that target or if
// it implements a method Is(error) bool such that Is(target) returns true.
// (text from errors/wrap.go)
var EventIs = errors.Is

// EventAs finds the first error in err's chain that matches target, and if so, sets
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
var EventAs = errors.As

// NewEvent returns an Event fully populated.
func NewEvent(eventtype, reason, messageFmt string, args ...interface{}) Event {
	return &ReconcilerEvent{
		EventType: eventtype,
		Reason:    reason,
		Format:    messageFmt,
		Args:      args,
	}
}

// ReconcilerEvent wraps the fields required for recorders to create a
// kubernetes recorder Event.
type ReconcilerEvent struct { //nolint:revive // for backcompat.
	EventType string
	Reason    string
	Format    string
	Args      []interface{}
}

// make sure ReconcilerEvent implements error.
var _ error = (*ReconcilerEvent)(nil)

// Is returns if the target error is a ReconcilerEvent type checking that
// EventType and Reason match.
func (e *ReconcilerEvent) Is(target error) bool {
	var t *ReconcilerEvent
	if errors.As(target, &t) {
		if t != nil && t.EventType == e.EventType && t.Reason == e.Reason {
			return true
		}
		return false
	}
	// Allow for wrapped errors.
	err := fmt.Errorf(e.Format, e.Args...)
	return errors.Is(err, target)
}

// As allows ReconcilerEvents to be treated as regular error types.
func (e *ReconcilerEvent) As(target interface{}) bool {
	err := fmt.Errorf(e.Format, e.Args...)
	return errors.As(err, target)
}

// Error returns the string that is formed by using the format string with the
// provided args.
func (e *ReconcilerEvent) Error() string {
	return fmt.Errorf(e.Format, e.Args...).Error()
}
