/*
Copyright 2019 The Tekton Authors

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

package termination

import (
	"encoding/json"
	"os"

	"github.com/tektoncd/pipeline/pkg/result"
)

const (
	// MaxContainerTerminationMessageLength is the upper bound any one container may write to
	// its termination message path. Contents above this length will cause a failure.
	MaxContainerTerminationMessageLength = 1024 * 4
)

// WriteMessage writes the results to the termination message path.
func WriteMessage(path string, pro []result.RunResult) error {
	// if the file at path exists, concatenate the new values otherwise create it
	// file at path already exists
	fileContents, err := os.ReadFile(path)
	if err == nil {
		var existingEntries []result.RunResult
		if err := json.Unmarshal(fileContents, &existingEntries); err == nil {
			// append new entries to existing entries
			pro = append(existingEntries, pro...)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	jsonOutput, err := json.Marshal(pro)
	if err != nil {
		return err
	}

	if len(jsonOutput) > MaxContainerTerminationMessageLength {
		return errTooLong
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.Write(jsonOutput); err != nil {
		return err
	}
	return f.Sync()
}

// MessageLengthError indicate the length of termination message of container is beyond 4096 which is the max length read by kubenates
type MessageLengthError string

const (
	errTooLong MessageLengthError = "Termination message is above max allowed size 4096, caused by large task result."
)

func (e MessageLengthError) Error() string {
	return string(e)
}
