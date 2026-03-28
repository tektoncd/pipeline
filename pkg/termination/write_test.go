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
		logger.Fatalf("Error while writing message: %s", err)
	}

	output = []result.RunResult{{
		Key:   "key2",
		Value: "world",
	}}

	if err := termination.WriteMessage(tmpFile.Name(), output); err != nil {
		logger.Fatalf("Error while writing message: %s", err)
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

func TestWriteCompressedMessage(t *testing.T) {
	tmpFile, err := os.CreateTemp(os.TempDir(), "tempFile")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(tmpFile.Name())

	output := []result.RunResult{{
		Key:        "digest",
		Value:      "sha256:abc123",
		ResultType: result.TaskRunResultType,
	}, {
		Key:        "url",
		Value:      "https://example.com/image",
		ResultType: result.TaskRunResultType,
	}}

	if err := termination.WriteCompressedMessage(tmpFile.Name(), output); err != nil {
		t.Fatalf("WriteCompressedMessage: %v", err)
	}

	fileContents, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.HasPrefix(string(fileContents), "tknz:") {
		t.Fatalf("Expected compressed message to start with 'tknz:' prefix, got: %.20s...", string(fileContents))
	}
}

func TestWriteCompressedMessage_RoundTrip(t *testing.T) {
	tmpFile, err := os.CreateTemp(os.TempDir(), "tempFile")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(tmpFile.Name())

	logger, _ := logging.NewLogger("", "test termination")
	defer func() {
		_ = logger.Sync()
	}()

	output := []result.RunResult{{
		Key:        "digest",
		Value:      "sha256:abc123def456",
		ResultType: result.TaskRunResultType,
	}, {
		Key:        "url",
		Value:      "https://registry.example.com/myimage:latest",
		ResultType: result.TaskRunResultType,
	}}

	if err := termination.WriteCompressedMessage(tmpFile.Name(), output); err != nil {
		t.Fatalf("WriteCompressedMessage: %v", err)
	}

	fileContents, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	got, err := termination.ParseMessage(logger, string(fileContents))
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	if d := cmp.Diff(output, got); d != "" {
		t.Fatalf("Round-trip mismatch %s", diff.PrintWantGot(d))
	}
}

func TestWriteCompressedMessage_SmallerThanJSON(t *testing.T) {
	// Use large repetitive values that compress well.
	largeValue := strings.Repeat("result-value-data-", 50)
	results := []result.RunResult{{
		Key:        "key1",
		Value:      largeValue,
		ResultType: result.TaskRunResultType,
	}, {
		Key:        "key2",
		Value:      largeValue,
		ResultType: result.TaskRunResultType,
	}}

	plainFile, err := os.CreateTemp(os.TempDir(), "plainFile")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(plainFile.Name())

	compressedFile, err := os.CreateTemp(os.TempDir(), "compressedFile")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(compressedFile.Name())

	if err := termination.WriteMessage(plainFile.Name(), results); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	if err := termination.WriteCompressedMessage(compressedFile.Name(), results); err != nil {
		t.Fatalf("WriteCompressedMessage: %v", err)
	}

	plainContents, err := os.ReadFile(plainFile.Name())
	if err != nil {
		t.Fatalf("ReadFile (plain): %v", err)
	}
	compressedContents, err := os.ReadFile(compressedFile.Name())
	if err != nil {
		t.Fatalf("ReadFile (compressed): %v", err)
	}

	if len(compressedContents) >= len(plainContents) {
		t.Fatalf("Expected compressed (%d bytes) to be smaller than plain JSON (%d bytes)",
			len(compressedContents), len(plainContents))
	}
}

func TestWriteCompressedMessage_AppendToExisting(t *testing.T) {
	tmpFile, err := os.CreateTemp(os.TempDir(), "tempFile")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(tmpFile.Name())

	logger, _ := logging.NewLogger("", "test termination")
	defer func() {
		_ = logger.Sync()
	}()

	first := []result.RunResult{{
		Key:        "digest",
		Value:      "sha256:first",
		ResultType: result.TaskRunResultType,
	}}

	second := []result.RunResult{{
		Key:        "url",
		Value:      "https://example.com/image",
		ResultType: result.TaskRunResultType,
	}}

	if err := termination.WriteCompressedMessage(tmpFile.Name(), first); err != nil {
		t.Fatalf("WriteCompressedMessage (first): %v", err)
	}
	if err := termination.WriteCompressedMessage(tmpFile.Name(), second); err != nil {
		t.Fatalf("WriteCompressedMessage (second): %v", err)
	}

	fileContents, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	got, err := termination.ParseMessage(logger, string(fileContents))
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	want := []result.RunResult{{
		Key:        "digest",
		Value:      "sha256:first",
		ResultType: result.TaskRunResultType,
	}, {
		Key:        "url",
		Value:      "https://example.com/image",
		ResultType: result.TaskRunResultType,
	}}

	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("Append mismatch %s", diff.PrintWantGot(d))
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
