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
	tmpFile, err := os.CreateTemp(t.TempDir(), "tempFile")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}

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
	tmpFile, err := os.CreateTemp(t.TempDir(), "tempFile")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}

	// Use enough results so compression actually saves space (small payloads
	// fall back to plain JSON when compressed output is larger).
	var output []result.RunResult
	for i := range 20 {
		output = append(output, result.RunResult{
			Key:        "image-digest-" + string(rune('a'+i)),
			Value:      "gcr.io/project/image@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			ResultType: result.TaskRunResultType,
		})
	}

	if err := termination.WriteCompressedMessage(tmpFile.Name(), output); err != nil {
		t.Fatalf("WriteCompressedMessage: %v", err)
	}

	fileContents, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.HasPrefix(string(fileContents), "tknz:") {
		t.Fatalf("Expected compressed message to start with 'tknz:' prefix, got: %.40s...", string(fileContents))
	}
}

func TestWriteCompressedMessage_RoundTrip(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "tempFile")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}

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

	plainFile, err := os.CreateTemp(t.TempDir(), "plainFile")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}

	compressedFile, err := os.CreateTemp(t.TempDir(), "compressedFile")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}

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
	tmpFile, err := os.CreateTemp(t.TempDir(), "tempFile")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}

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
	tmpFile, err := os.CreateTemp(t.TempDir(), "tempFile")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}

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
	for i := range 50 {
		results = append(results, result.RunResult{
			Key:        "image-digest-result-" + string(rune('a'+i%26)) + strings.Repeat("-", i%5),
			Value:      "gcr.io/my-project/my-image-name@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678" + strings.Repeat("90", i%3+1),
			ResultType: result.TaskRunResultType,
		})
	}

	// Step 1: Verify this exceeds the 4KB limit without compression
	plainFile, err := os.CreateTemp(t.TempDir(), "plain-*")
	if err != nil {
		t.Fatal(err)
	}
	plainFile.Close()

	plainErr := termination.WriteMessage(plainFile.Name(), results)
	var lengthErr termination.MessageLengthError
	if !errors.As(plainErr, &lengthErr) {
		t.Fatalf("Expected plain JSON to exceed 4KB limit, but got: %v", plainErr)
	}

	// Step 2: Verify the same results fit with compression
	compressedFile, err := os.CreateTemp(t.TempDir(), "compressed-*")
	if err != nil {
		t.Fatal(err)
	}
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

// TestWriteCompressedMessage_CapacityBenchmark measures exactly how many
// results fit within the 4KB limit with and without compression.
// This documents the concrete capacity improvement from the feature.
func TestWriteCompressedMessage_CapacityBenchmark(t *testing.T) {
	// Each result simulates a realistic image digest reference (~100 bytes value).
	makeResult := func(i int) result.RunResult {
		return result.RunResult{
			Key:        "image-digest-" + strings.Repeat("x", 5) + "-" + string(rune('a'+i%26)),
			Value:      "gcr.io/my-project/my-image@sha256:" + strings.Repeat("abcdef1234567890", 4),
			ResultType: result.TaskRunResultType,
		}
	}

	// Find max results without compression
	tmpDir := t.TempDir()
	maxPlain := 0
	for n := 1; n <= 200; n++ {
		var results []result.RunResult
		for i := range n {
			results = append(results, makeResult(i))
		}
		tmpFile, _ := os.CreateTemp(tmpDir, "plain-*")
		err := termination.WriteMessage(tmpFile.Name(), results)
		os.Remove(tmpFile.Name())
		if err != nil {
			break
		}
		maxPlain = n
	}

	// Find max results with compression
	maxCompressed := 0
	for n := 1; n <= 200; n++ {
		var results []result.RunResult
		for i := range n {
			results = append(results, makeResult(i))
		}
		tmpFile, _ := os.CreateTemp(tmpDir, "compressed-*")
		err := termination.WriteCompressedMessage(tmpFile.Name(), results)
		os.Remove(tmpFile.Name())
		if err != nil {
			break
		}
		maxCompressed = n
	}

	improvement := float64(maxCompressed-maxPlain) / float64(maxPlain) * 100

	t.Logf("=== Compression Capacity Benchmark ===")
	t.Logf("Result size: ~100 byte image digest reference")
	t.Logf("Without compression: %d results fit in 4KB", maxPlain)
	t.Logf("With compression:    %d results fit in 4KB", maxCompressed)
	t.Logf("Improvement:         +%.0f%% more results", improvement)
	t.Logf("======================================")

	// Verify compression provides meaningful improvement
	if maxCompressed <= maxPlain {
		t.Fatalf("Compression should fit more results than plain JSON (%d <= %d)", maxCompressed, maxPlain)
	}
}
