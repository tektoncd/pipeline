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
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const helloWorldBase64 = "aGVsbG8gd29ybGQK"

// TestProcessSuccessfulSubcommands checks that input args matching the format
// expected by subcommands results in successfully running those subcommands.
func TestProcessSuccessfulSubcommands(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "foo.txt")
	dst := filepath.Join(tmp, "bar.txt")

	srcFile, err := os.OpenFile(src, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("error opening temp file for writing: %v", err)
	}
	defer srcFile.Close()
	if _, err := srcFile.Write([]byte(helloWorldBase64)); err != nil {
		t.Fatalf("error writing source file: %v", err)
	}

	var ok OK

	for _, tc := range []struct {
		command string
		args    []string
	}{
		{
			command: CopyCommand,
			args:    []string{src, dst},
		},
		{
			command: DecodeScriptCommand,
			args:    []string{src},
		},
	} {
		t.Run(tc.command, func(t *testing.T) {
			returnValue := Process(append([]string{tc.command}, tc.args...))
			if !errors.As(returnValue, &ok) {
				t.Errorf("unexpected return value from command: %v", returnValue)
			}
		})
	}

	t.Run(StepInitCommand, func(t *testing.T) {
		tektonRoot = tmp

		returnValue := Process([]string{StepInitCommand})
		if !errors.As(returnValue, &ok) {
			t.Errorf("unexpected return value from step-init command: %v", returnValue)
		}

		returnValue = Process([]string{StepInitCommand, "foo", "bar"})
		if !errors.As(returnValue, &ok) {
			t.Errorf("unexpected return value from step-init command w/ params: %v", returnValue)
		}
	})
}

// TestProcessIgnoresNonSubcommands checks that any input to Process which
// does not exactly match the requirements of a configured subcommand
// correctly passes back a nil result.
func TestProcessIgnoresNonSubcommands(t *testing.T) {
	if err := Process([]string{"not", "a", "matching", "subcommand"}); err != nil {
		t.Errorf("unexpected error processing unmatched subcommand: %v", err)
	}

	if err := Process([]string{}); err != nil {
		t.Errorf("unexpected error processing 0 args: %v", err)
	}

	t.Run(CopyCommand, func(t *testing.T) {
		if err := Process([]string{CopyCommand}); err != nil {
			t.Errorf("unexpected error processing command with 0 additional args: %v", err)
		}

		if err := Process([]string{CopyCommand, "foo.txt", "bar.txt", "baz.txt"}); err != nil {
			t.Errorf("unexpected error processing command with invalid number of args: %v", err)
		}
	})

	t.Run(DecodeScriptCommand, func(t *testing.T) {
		if err := Process([]string{DecodeScriptCommand}); err != nil {
			t.Errorf("unexpected error processing command with 0 additional args: %v", err)
		}

		if err := Process([]string{DecodeScriptCommand, "foo.txt", "bar.txt"}); err != nil {
			t.Errorf("unexpected error processing command with invalid number of args: %v", err)
		}
	})
}
