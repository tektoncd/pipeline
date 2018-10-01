/*
Copyright 2017 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apis

import (
	"fmt"
	"sort"
	"strings"
)

// CurrentField is a constant to supply as a fieldPath for when there is
// a problem with the current field itself.
const CurrentField = ""

// FieldError is used to propagate the context of errors pertaining to
// specific fields in a manner suitable for use in a recursive walk, so
// that errors contain the appropriate field context.
// FieldError methods are non-mutating.
// +k8s:deepcopy-gen=false
type FieldError struct {
	Message string
	Paths   []string
	// Details contains an optional longer payload.
	// +optional
	Details string
	errors  []FieldError
}

// FieldError implements error
var _ error = (*FieldError)(nil)

// ViaField is used to propagate a validation error along a field access.
// For example, if a type recursively validates its "spec" via:
//   if err := foo.Spec.Validate(); err != nil {
//     // Augment any field paths with the context that they were accessed
//     // via "spec".
//     return err.ViaField("spec")
//   }
func (fe *FieldError) ViaField(prefix ...string) *FieldError {
	if fe == nil {
		return nil
	}
	newErr := &FieldError{}
	for _, e := range fe.getNormalizedErrors() {
		// Prepend the Prefix to existing errors.
		newPaths := make([]string, 0, len(e.Paths))
		for _, oldPath := range e.Paths {
			newPaths = append(newPaths, flatten(append(prefix, oldPath)))
		}
		sort.Slice(newPaths, func(i, j int) bool { return newPaths[i] < newPaths[j] })
		e.Paths = newPaths

		// Append the mutated error to the errors list.
		newErr = newErr.Also(&e)
	}
	return newErr
}

// ViaIndex is used to attach an index to the next ViaField provided.
// For example, if a type recursively validates a parameter that has a collection:
//  for i, c := range spec.Collection {
//    if err := doValidation(c); err != nil {
//      return err.ViaIndex(i).ViaField("collection")
//    }
//  }
func (fe *FieldError) ViaIndex(index int) *FieldError {
	return fe.ViaField(asIndex(index))
}

// ViaFieldIndex is the short way to chain: err.ViaIndex(bar).ViaField(foo)
func (fe *FieldError) ViaFieldIndex(field string, index int) *FieldError {
	return fe.ViaIndex(index).ViaField(field)
}

// ViaKey is used to attach a key to the next ViaField provided.
// For example, if a type recursively validates a parameter that has a collection:
//  for k, v := range spec.Bag. {
//    if err := doValidation(v); err != nil {
//      return err.ViaKey(k).ViaField("bag")
//    }
//  }
func (fe *FieldError) ViaKey(key string) *FieldError {
	return fe.ViaField(asKey(key))
}

// ViaFieldKey is the short way to chain: err.ViaKey(bar).ViaField(foo)
func (fe *FieldError) ViaFieldKey(field string, key string) *FieldError {
	return fe.ViaKey(key).ViaField(field)
}

// Also collects errors, returns a new collection of existing errors and new errors.
func (fe *FieldError) Also(errs ...*FieldError) *FieldError {
	newErr := &FieldError{}
	// collect the current objects errors, if it has any
	if fe != nil {
		newErr.errors = fe.getNormalizedErrors()
	}
	// and then collect the passed in errors
	for _, e := range errs {
		newErr.errors = append(newErr.errors, e.getNormalizedErrors()...)
	}
	if len(newErr.errors) == 0 {
		return nil
	}
	return newErr
}

func (fe *FieldError) getNormalizedErrors() []FieldError {
	// in case we call getNormalizedErrors on a nil object, return just an empty
	// list. This can happen when .Error() is called on a nil object.
	if fe == nil {
		return []FieldError(nil)
	}
	var errors []FieldError
	// if this FieldError is a leaf,
	if fe.Message != "" {
		err := FieldError{
			Message: fe.Message,
			Paths:   fe.Paths,
			Details: fe.Details,
		}
		errors = append(errors, err)
	}
	// and then collect all other errors recursively.
	for _, e := range fe.errors {
		errors = append(errors, e.getNormalizedErrors()...)
	}
	sort.Slice(errors, func(i, j int) bool { return errors[i].Message < errors[j].Message })
	return errors
}

func asIndex(index int) string {
	return fmt.Sprintf("[%d]", index)
}

func asKey(key string) string {
	return fmt.Sprintf("[%s]", key)
}

// flatten takes in a array of path components and looks for chances to flatten
// objects that have index prefixes, examples:
//   err([0]).ViaField(bar).ViaField(foo) -> foo.bar.[0] converts to foo.bar[0]
//   err(bar).ViaIndex(0).ViaField(foo) -> foo.[0].bar converts to foo[0].bar
//   err(bar).ViaField(foo).ViaIndex(0) -> [0].foo.bar converts to [0].foo.bar
//   err(bar).ViaIndex(0).ViaIndex[1].ViaField(foo) -> foo.[1].[0].bar converts to foo[1][0].bar
func flatten(path []string) string {
	var newPath []string
	for _, part := range path {
		for _, p := range strings.Split(part, ".") {
			if p == CurrentField {
				continue
			} else if len(newPath) > 0 && isIndex(p) {
				newPath[len(newPath)-1] = fmt.Sprintf("%s%s", newPath[len(newPath)-1], p)
			} else {
				newPath = append(newPath, p)
			}
		}
	}
	return strings.Join(newPath, ".")
}

func isIndex(part string) bool {
	return strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]")
}

// Error implements error
func (fe *FieldError) Error() string {
	var errs []string
	for _, e := range fe.getNormalizedErrors() {
		if e.Details == "" {
			errs = append(errs, fmt.Sprintf("%v: %v", e.Message, strings.Join(e.Paths, ", ")))
		} else {
			errs = append(errs, fmt.Sprintf("%v: %v\n%v", e.Message, strings.Join(e.Paths, ", "), e.Details))
		}
	}
	return strings.Join(errs, "\n")
}

// ErrMissingField is a variadic helper method for constructing a FieldError for
// a set of missing fields.
func ErrMissingField(fieldPaths ...string) *FieldError {
	return &FieldError{
		Message: "missing field(s)",
		Paths:   fieldPaths,
	}
}

// ErrDisallowedFields is a variadic helper method for constructing a FieldError
// for a set of disallowed fields.
func ErrDisallowedFields(fieldPaths ...string) *FieldError {
	return &FieldError{
		Message: "must not set the field(s)",
		Paths:   fieldPaths,
	}
}

// ErrInvalidValue constructs a FieldError for a field that has received an
// invalid string value.
func ErrInvalidValue(value, fieldPath string) *FieldError {
	return &FieldError{
		Message: fmt.Sprintf("invalid value %q", value),
		Paths:   []string{fieldPath},
	}
}

// ErrMissingOneOf is a variadic helper method for constructing a FieldError for
// not having at least one field in a mutually exclusive field group.
func ErrMissingOneOf(fieldPaths ...string) *FieldError {
	return &FieldError{
		Message: "expected exactly one, got neither",
		Paths:   fieldPaths,
	}
}

// ErrMultipleOneOf is a variadic helper method for constructing a FieldError
// for having more than one field set in a mutually exclusive field group.
func ErrMultipleOneOf(fieldPaths ...string) *FieldError {
	return &FieldError{
		Message: "expected exactly one, got both",
		Paths:   fieldPaths,
	}
}

// ErrInvalidKeyName is a variadic helper method for constructing a FieldError
// that specifies a key name that is invalid.
func ErrInvalidKeyName(value, fieldPath string, details ...string) *FieldError {
	return &FieldError{
		Message: fmt.Sprintf("invalid key name %q", value),
		Paths:   []string{fieldPath},
		Details: strings.Join(details, ", "),
	}
}
