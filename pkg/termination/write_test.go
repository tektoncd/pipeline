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
package termination_test

import (
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/result"
	termination "github.com/tektoncd/pipeline/pkg/termination"
	"github.com/tektoncd/pipeline/test/diff"
	"knative.dev/pkg/logging"
)

func TestExistingFile(t *testing.T) {
	tmpFile, err := os.CreateTemp(os.TempDir(), "tempFile")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	// Remember to clean up the file afterwards
	defer os.Remove(tmpFile.Name())

	logger, _ := logging.NewLogger("", "test termination")
	defer func() {
		_ = logger.Sync()
	}()
	output := []result.RunResult{{
		Key:   "key1",
		Value: "hello",
	}}

	if err := termination.WriteMessage(tmpFile.Name(), output); err != nil {
		logger.Fatalf("Errot while writing message: %s", err)
	}

	output = []result.RunResult{{
		Key:   "key2",
		Value: "world",
	}}

	if err := termination.WriteMessage(tmpFile.Name(), output); err != nil {
		logger.Fatalf("Errot while writing message: %s", err)
	}

	if fileContents, err := os.ReadFile(tmpFile.Name()); err != nil {
		logger.Fatalf("Unexpected error reading %v: %v", tmpFile.Name(), err)
	} else {
		want := `[{"key":"key1","value":"hello"},{"key":"key2","value":"world"}]`
		if d := cmp.Diff(want, string(fileContents)); d != "" {
			t.Fatalf("Diff %s", diff.PrintWantGot(d))
		}
	}
}

func TestMaxSizeFile(t *testing.T) {
	value := strings.Repeat("a", 4096)
	tmpFile, err := os.CreateTemp(os.TempDir(), "tempFile")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	// Remember to clean up the file afterwards
	defer os.Remove(tmpFile.Name())

	output := []result.RunResult{{
		Key:   "key1",
		Value: value,
	}}

	err = termination.WriteMessage(tmpFile.Name(), output)
	var expectedErr termination.MessageLengthError
	if !errors.As(err, &expectedErr) {
		t.Fatalf("Expected MessageLengthError, received: %v", err)
	}
}
