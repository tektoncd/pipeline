/*
Copyright 2023 The Tekton Authors

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

package result

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-multierror"
)

const (
	// TaskRunResultType default task run result value
	TaskRunResultType ResultType = 1
	// reserved: 2
	// was RunResultType

	// InternalTektonResultType default internal tekton result value
	InternalTektonResultType = 3
	// UnknownResultType default unknown result type value
	UnknownResultType = 10
)

// RunResult is used to write key/value pairs to TaskRun pod termination messages.
// The key/value pairs may come from the entrypoint binary, or represent a TaskRunResult.
// If they represent a TaskRunResult, the key is the name of the result and the value is the
// JSON-serialized value of the result.
type RunResult struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	// ResourceName may be used in tests, but it is not populated in termination messages.
	// It is preserved here for backwards compatibility and will not be ported to v1.
	ResourceName string     `json:"resourceName,omitempty"`
	ResultType   ResultType `json:"type,omitempty"`
}

// ResultType used to find out whether a RunResult is from a task result or not
// Note that ResultsType is another type which is used to define the data type
// (e.g. string, array, etc) we used for Results
//
//nolint:revive // revive complains about stutter of `result.ResultType`.
type ResultType int

// UnmarshalJSON unmarshals either an int or a string into a ResultType. String
// ResultTypes were removed because they made JSON messages bigger, which in
// turn limited the amount of space in termination messages for task results. String
// support is maintained for backwards compatibility - the Pipelines controller could
// be stopped midway through TaskRun execution, updated with support for int in place
// of string, and then fail the running TaskRun because it doesn't know how to interpret
// the string value that the TaskRun's entrypoint will emit when it completes.
func (r *ResultType) UnmarshalJSON(data []byte) error {
	var asInt int
	var intErr error

	if err := json.Unmarshal(data, &asInt); err != nil {
		intErr = err
	} else {
		*r = ResultType(asInt)
		return nil
	}

	var asString string

	if err := json.Unmarshal(data, &asString); err != nil {
		return fmt.Errorf("unsupported value type, neither int nor string: %w", multierror.Append(intErr, err).ErrorOrNil())
	}

	switch asString {
	case "TaskRunResult":
		*r = TaskRunResultType
	case "InternalTektonResult":
		*r = InternalTektonResultType
	default:
		*r = UnknownResultType
	}

	return nil
}
