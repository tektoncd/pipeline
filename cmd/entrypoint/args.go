/*
Copyright 2022 The Tekton Authors

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

package main

func extractArgs(initialArgs []string) ([]string, []string) {
	commandArgs := []string{}
	args := initialArgs
	if len(initialArgs) == 0 {
		return args, commandArgs
	}
	// Detect if `--` is present, if it is, parse only the one before.
	terminatorIndex := -1
	for i, a := range initialArgs {
		if a == "--" {
			terminatorIndex = i
			break
		}
	}
	if terminatorIndex > 0 {
		commandArgs = initialArgs[terminatorIndex+1:]
		args = initialArgs[:terminatorIndex]
	}
	return args, commandArgs
}
