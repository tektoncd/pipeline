/*
Copyright 2020 The Tekton Authors

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
	"fmt"
)

// OK is returned for successful subcommand executions.
type OK struct {
	message string
}

func (err OK) Error() string {
	return err.message
}

// SubcommandSuccessful is an alias for the OK type.
//
// Deprecated: replace usage with OK type.
type SubcommandSuccessful = OK

var (
	// Compile-time check that OK is an error type.
	_ error = OK{}
	// Compile-time check that objects of type OK are cast to deprecated
	// SubcommandSuccessful type.
	_ SubcommandSuccessful = OK{}
)

// SubcommandError is returned for failed subcommand executions.
type SubcommandError struct {
	subcommand string
	message    string
}

func (err SubcommandError) Error() string {
	return fmt.Sprintf("%s error: %s", err.subcommand, err.message)
}

// Process takes the set of arguments passed to entrypoint and executes any
// subcommand that the args call for. An error is returned to the caller to
// indicate that a subcommand was matched and to pass back its success/fail
// state. The returned error will be nil if no subcommand was matched to the
// passed args, OK if args matched and the subcommand
// succeeded, or any other error if the args matched but the subcommand failed.
func Process(args []string) error {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case InitCommand:
		// If invoked in "init mode" (`entrypoint init <src> <dst> [<step-name>]`),
		// it will copy the src path to the dst path (like CopyCommand), and initialize
		// the /tekton/steps folder (like StepInitCommand)
		if len(args) >= 3 {
			src, dst := args[1], args[2]
			steps := args[3:]
			if err := entrypointInit(src, dst, steps); err != nil {
				return SubcommandError{subcommand: InitCommand, message: err.Error()}
			}
			return OK{message: "Entrypoint initialization"}
		}
	case CopyCommand:
		// If invoked in "cp mode" (`entrypoint cp <src> <dst>`), simply copy
		// the src path to the dst path. This is used to place the entrypoint
		// binary in the tools directory, without requiring the cp command to
		// exist in the base image.
		if len(args) == 3 {
			src, dst := args[1], args[2]
			if err := cp(src, dst); err != nil {
				return SubcommandError{subcommand: CopyCommand, message: err.Error()}
			}
			return OK{message: fmt.Sprintf("Copied %s to %s", src, dst)}
		}
	case DecodeScriptCommand:
		// If invoked in "decode-script" mode (`entrypoint decode-script <src>`),
		// read the script at <src> and overwrite it with its decoded content.
		if len(args) == 2 {
			src := args[1]
			if err := decodeScript(src); err != nil {
				return SubcommandError{subcommand: DecodeScriptCommand, message: err.Error()}
			}
			return OK{message: fmt.Sprintf("Decoded script %s", src)}
		}
	case StepInitCommand:
		if err := stepInit(args[1:]); err != nil {
			return SubcommandError{subcommand: StepInitCommand, message: err.Error()}
		}
		return OK{message: "Setup /step directories"}
	default:
	}
	return nil
}
