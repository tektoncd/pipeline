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
	"io/ioutil"
	"os"

	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

const (
	// MaxContainerTerminationMessageLength is the upper bound any one container may write to
	// its termination message path. Contents above this length will cause a failure.
	MaxContainerTerminationMessageLength = 1024 * 4
)

// WriteMessage writes the results to the termination message path.
func WriteMessage(path string, pro []v1beta1.PipelineResourceResult) error {
	// if the file at path exists, concatenate the new values otherwise create it
	// file at path already exists
	fileContents, err := ioutil.ReadFile(path)
	if err == nil {
		var existingEntries []v1beta1.PipelineResourceResult
		if err := json.Unmarshal([]byte(fileContents), &existingEntries); err == nil {
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
		return aboveMax
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
	aboveMax MessageLengthError = "Termination message is above max allowed size 4096, caused by large task result."
)

func (e MessageLengthError) Error() string {
	return string(e)
}
