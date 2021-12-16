package subcommands

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const helloWorldBase64 = "aGVsbG8gd29ybGQK"

// TestProcessSuccessfulSubcommands checks that input args matching the format
// expected by subcommands results in successfully running those subcommands.
func TestProcessSuccessfulSubcommands(t *testing.T) {
	tmp, err := ioutil.TempDir("", "cp-test-*")
	if err != nil {
		t.Fatalf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmp)
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

	returnValue := Process([]string{CopyCommand, src, dst})
	if _, ok := returnValue.(SubcommandSuccessful); !ok {
		t.Errorf("unexpected return value from cp command: %v", returnValue)
	}

	returnValue = Process([]string{DecodeScriptCommand, src})
	if _, ok := returnValue.(SubcommandSuccessful); !ok {
		t.Errorf("unexpected return value from decode-script command: %v", returnValue)
	}

	t.Run(StepInitCommand, func(t *testing.T) {
		tektonRoot = tmp

		returnValue := Process([]string{StepInitCommand})
		if _, ok := returnValue.(SubcommandSuccessful); !ok {
			t.Errorf("unexpected return value from step-init command: %v", returnValue)
		}

		returnValue = Process([]string{StepInitCommand, "foo", "bar"})
		if _, ok := returnValue.(SubcommandSuccessful); !ok {
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

	if err := Process([]string{CopyCommand}); err != nil {
		t.Errorf("unexpected error processing decode-script command with 0 additional args: %v", err)
	}

	if err := Process([]string{CopyCommand, "foo.txt", "bar.txt", "baz.txt"}); err != nil {
		t.Errorf("unexpected error processing cp command with invalid number of args: %v", err)
	}

	if err := Process([]string{DecodeScriptCommand}); err != nil {
		t.Errorf("unexpected error processing decode-script command with 0 additional args: %v", err)
	}

	if err := Process([]string{DecodeScriptCommand, "foo.txt", "bar.txt"}); err != nil {
		t.Errorf("unexpected error processing decode-script command with invalid number of args: %v", err)
	}
}
