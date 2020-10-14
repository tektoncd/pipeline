/*
Copyright 2019 The Knative Authors

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

// error.go helps with error handling

package helpers

import (
	"errors"
	"strings"
)

// CombineErrors combines slice of errors and return a single error
func CombineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0)
	for _, err := range errs {
		if err != nil {
			msgs = append(msgs, err.Error())
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	return errors.New(strings.Join(msgs, "\n"))
}
