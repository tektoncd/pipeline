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

package subcommands

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// StepInitCommand is the name of the /tekton/steps initialization command.
const StepInitCommand = "step-init"

var (
	// root is the location of the Tekton root directory.
	// Included as a global variable to allow overriding for tests.
	tektonRoot = "/tekton"
)

// stepInit sets up the /tekton/steps directory for the pod.
// This expects the list of steps (in order matching the Task spec).
func stepInit(steps []string) error {
	// Setup step directory symlinks - step data is written to a /tekton/run/<step>/status
	// folder corresponding to each step - this is only mounted RW for the matching user step
	// (and RO for all other steps).
	// /tekton/steps provides a convenience symlink so that Tekton utilities to reference steps
	// by name or index.
	// NOTE: /tekton/steps may be removed in the future. Prefer using /tekton/run directly if
	// possible.

	// Create directory if it doesn't already exist
	stepDir := filepath.Join(tektonRoot, "steps")
	if err := os.MkdirAll(stepDir, os.ModePerm); err != nil {
		log.Fatalf("Error creating steps directory %q: %v", stepDir, err)
	}

	for i, s := range steps {
		run := filepath.Join(tektonRoot, "run", strconv.Itoa(i), "status")
		if err := os.Symlink(run, filepath.Join(stepDir, s)); err != nil {
			return err
		}
		if err := os.Symlink(run, filepath.Join(stepDir, strconv.Itoa(i))); err != nil {
			return err
		}
	}
	return nil
}
