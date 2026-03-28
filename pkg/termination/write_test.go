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

// TestWriteCompressedMessage_ExceedsLimitUncompressedButFitsCompressed proves
// that results exceeding the 4KB limit in plain JSON succeed with compression.
// This is the core value proposition of the compression feature.
func TestWriteCompressedMessage_ExceedsLimitUncompressedButFitsCompressed(t *testing.T) {
	// Create results that produce >4KB JSON but <4KB compressed.
	// Repetitive data compresses well — typical for results with similar
	// structure (e.g., multiple image digests, URLs, build metadata).
	var results []result.RunResult
	for i := 0; i < 50; i++ {
		results = append(results, result.RunResult{
			Key:        "image-digest-result-" + string(rune('a'+i%26)) + strings.Repeat("-", i%5),
			Value:      "gcr.io/my-project/my-image-name@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678" + strings.Repeat("90", i%3+1),
			ResultType: result.TaskRunResultType,
		})
	}

	// Step 1: Verify this exceeds the 4KB limit without compression
	plainFile, err := os.CreateTemp("", "plain-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(plainFile.Name())
	plainFile.Close()

	plainErr := termination.WriteMessage(plainFile.Name(), results)
	var lengthErr termination.MessageLengthError
	if !errors.As(plainErr, &lengthErr) {
		t.Fatalf("Expected plain JSON to exceed 4KB limit, but got: %v", plainErr)
	}

	// Step 2: Verify the same results fit with compression
	compressedFile, err := os.CreateTemp("", "compressed-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(compressedFile.Name())
	compressedFile.Close()

	compressedErr := termination.WriteCompressedMessage(compressedFile.Name(), results)
	if compressedErr != nil {
		t.Fatalf("Expected compressed message to fit in 4KB, but got: %v", compressedErr)
	}

	// Step 3: Verify all results survive the round-trip
	compressedData, err := os.ReadFile(compressedFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	logger, _ := logging.NewLogger("", "test")
	parsed, err := termination.ParseMessage(logger, string(compressedData))
	if err != nil {
		t.Fatalf("Failed to parse compressed message: %v", err)
	}

	if len(parsed) != len(results) {
		t.Fatalf("Expected %d results after round-trip, got %d", len(results), len(parsed))
	}

	t.Logf("Plain JSON: >4KB (rejected), Compressed: %d bytes (accepted), Results: %d",
		len(compressedData), len(parsed))
}
