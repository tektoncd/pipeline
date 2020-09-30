/*
Copyright 2020 The Knative Authors

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

package logging

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
)

// StructuredError is an error which can hold arbitrary key-value arguments.
//
// TODO(coryrc): The Structured Error is experimental and likely to be removed, but is currently in use in a refactored test.
type StructuredError interface {
	error
	GetValues() []interface{}
	WithValues(...interface{}) StructuredError
	DisableValuePrinting()
	EnableValuePrinting()
}

type structuredError struct {
	msg           string
	keysAndValues []interface{}
	print         bool
}

func keysAndValuesToSpewedMap(args ...interface{}) map[string]string {
	m := make(map[string]string, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, val := args[i], args[i+1]
		if keyStr, ok := key.(string); ok {
			m[keyStr] = spew.Sdump(val)
		}
	}
	return m
}

// Error implements `error` interface
func (e structuredError) Error() string {
	// TODO(coryrc): accept zap.Field entries?
	if e.print {
		// %v for fmt.Sprintf does print keys sorted
		return fmt.Sprintf("Error: %s\nContext:\n%v", e.msg, keysAndValuesToSpewedMap(e.keysAndValues...))
	}
	return e.msg
}

// GetValues gives you the structured key values in a plist
func (e structuredError) GetValues() []interface{} {
	return e.keysAndValues
}

// DisableValuePrinting disables printing out the keys and values from the Error() method
func (e *structuredError) DisableValuePrinting() {
	e.print = false
}

// EnableValuePrinting enables printing out the keys and values from the Error() method
func (e *structuredError) EnableValuePrinting() {
	e.print = true
}

// Create a StructuredError. Gives a little better logging when given to a TLogger.
// This may prove to not be useful if users use the logger's WithValues() better.
func Error(msg string, keysAndValues ...interface{}) *structuredError {
	return &structuredError{msg, keysAndValues, true}
}

// WithValues operates just like TLogger's WithValues but stores them in the error object.
func (e *structuredError) WithValues(keysAndValues ...interface{}) StructuredError {
	newKAV := make([]interface{}, 0, len(keysAndValues)+len(e.keysAndValues))
	newKAV = append(newKAV, e.keysAndValues...)
	newKAV = append(newKAV, keysAndValues...)
	return &structuredError{e.msg, newKAV, e.print}
}
