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

// InitCommand is the name of main initialization command
const InitCommand = "init"

// init copies the entrypoint to the right place and sets up /tekton/steps directory for the pod.
// This expects  the list of steps (in order matching the Task spec).
func entrypointInit(src, dst string, steps []string) error {
	if err := cp(src, dst); err != nil {
		return err
	}
	return stepInit(steps)
}
