/*
Copyright 2026 The Tekton Authors

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

package entrypoint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1/types"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/pkg/termination"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/logging"
)

func TestEntrypointerSequentialTaskResults(t *testing.T) {
	for _, compress := range []bool{false, true} {
		t.Run(fmt.Sprintf("compression=%t", compress), func(t *testing.T) {
			resultsDir := t.TempDir()
			const count = 12
			names := make([]string, count)
			for i := range names {
				names[i] = fmt.Sprintf("result-%d", i)
			}
			allResults := make(map[string]string)
			previousStepDir := ""
			for i, name := range names {
				stepDir := t.TempDir()
				value := strings.Repeat(fmt.Sprintf("%02d", i), 225)
				e := Entrypointer{
					Waiter: &fakeWaiter{}, PostWriter: &fakePostWriter{},
					Runner:  &fakeResultsWriter{resultsToWrite: map[string]string{filepath.Join(resultsDir, name): value}},
					Results: names, ResultsDirectory: resultsDir,
					StepMetadataDir:            stepDir,
					PreviousStepMetadataDir:    previousStepDir,
					TerminationPath:            filepath.Join(stepDir, "termination"),
					ResultExtractionMethod:     ResultExtractionMethodTerminationMessage,
					CompressTerminationMessage: compress,
				}
				if err := e.Go(); err != nil {
					t.Fatalf("step %d: %v", i, err)
				}
				previousStepDir = stepDir
				got := taskResultsInMessage(t, e.TerminationPath)
				if d := cmp.Diff(map[string]string{name: value}, got); d != "" {
					t.Errorf("step %d results (-want +got): %s", i, d)
				}
				for k, v := range got {
					allResults[k] = v
				}
			}
			if len(allResults) != count {
				t.Errorf("got %d results, want %d", len(allResults), count)
			}
		})
	}
}

func taskResultsInMessage(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	logger, _ := logging.NewLogger("", "results-test")
	entries, err := termination.ParseMessage(logger, string(data))
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string)
	for _, r := range entries {
		if r.ResultType == result.TaskRunResultType {
			got[r.Key] = r.Value
		}
	}
	return got
}

func TestEntrypointerSharedTaskResults(t *testing.T) {
	resultsDir := t.TempDir()
	resultPath := filepath.Join(resultsDir, "output")
	// A pre-populated result must still be reported by the first step.
	if err := os.WriteFile(resultPath, []byte("first"), 0o666); err != nil {
		t.Fatal(err)
	}
	previousDir := ""
	stamp := time.Unix(100, 0)
	for _, tc := range []struct {
		name         string
		value        *string
		skip, remove bool
		want         map[string]string
	}{
		{name: "pre-populated", want: map[string]string{"output": "first"}},
		{name: "read previous result", want: map[string]string{}},
		{name: "same size and timestamp overwrite", value: ptr("other"), want: map[string]string{"output": "other"}},
		{name: "identical rewrite", value: ptr("other"), want: map[string]string{}},
		{name: "skipped step", skip: true, want: map[string]string{}},
		{name: "after skipped step", want: map[string]string{}},
		{name: "remove result", remove: true, want: map[string]string{}},
		{name: "recreate with last reported value", value: ptr("other"), want: map[string]string{}},
		{name: "restore earlier value", value: ptr("first"), want: map[string]string{"output": "first"}},
		{name: "empty result", value: ptr(""), want: map[string]string{"output": ""}},
	} {
		stepDir := t.TempDir()
		t.Run(tc.name, func(t *testing.T) {
			ran := false
			e := Entrypointer{
				Waiter: &fakeWaiter{}, PostWriter: &fakePostWriter{},
				Runner: resultRunnerFunc(func(context.Context, ...string) error {
					ran = true
					if tc.remove {
						return os.Remove(resultPath)
					}
					if tc.value != nil {
						if err := os.WriteFile(resultPath, []byte(*tc.value), 0o666); err != nil {
							return err
						}
					}
					// Reading another step's result remains supported. The directory is shared.
					if _, err := os.ReadFile(resultPath); err != nil {
						return err
					}
					return os.Chtimes(resultPath, stamp, stamp)
				}),
				Results: []string{"output", "never-written"}, ResultsDirectory: resultsDir,
				StepMetadataDir: stepDir, PreviousStepMetadataDir: previousDir,
				TerminationPath:        filepath.Join(stepDir, "termination"),
				ResultExtractionMethod: ResultExtractionMethodTerminationMessage,
			}
			if tc.skip {
				e.StepWhenExpressions = v1.StepWhenExpressions{{Input: "no", Operator: selection.In, Values: []string{"yes"}}}
			}
			if err := e.Go(); err != nil {
				t.Fatal(err)
			}
			if ran == tc.skip {
				t.Errorf("runner called=%t, skip=%t", ran, tc.skip)
			}
			if d := cmp.Diff(tc.want, taskResultsInMessage(t, e.TerminationPath)); d != "" {
				t.Errorf("results (-want +got): %s", d)
			}
			previousDir = stepDir
		})
	}
}

func TestEntrypointerPublishesResultsBeforeNextStep(t *testing.T) {
	resultsDir, stepDir := t.TempDir(), t.TempDir()
	resultPath := filepath.Join(resultsDir, "output")
	terminationPath := filepath.Join(stepDir, "termination")
	postFile := filepath.Join(stepDir, "out")
	released := false
	e := Entrypointer{
		Waiter:   &fakeWaiter{},
		Runner:   &fakeResultsWriter{resultsToWrite: map[string]string{resultPath: "first"}},
		PostFile: postFile,
		PostWriter: resultPostWriterFunc(func(file, _ string) {
			if file != postFile {
				return
			}
			released = true
			if d := cmp.Diff(map[string]string{"output": "first"}, taskResultsInMessage(t, terminationPath)); d != "" {
				t.Errorf("results before release (-want +got): %s", d)
			}
			hashes, err := readTaskResultHashes(stepDir)
			if err != nil {
				t.Fatal(err)
			}
			if hashes["output"] == "" {
				t.Error("released next step before saving result fingerprints")
			}
			// Simulate the next step immediately overwriting the shared result.
			if err := os.WriteFile(resultPath, []byte("second"), 0o666); err != nil {
				t.Fatal(err)
			}
		}),
		Results: []string{"output"}, ResultsDirectory: resultsDir,
		StepMetadataDir: stepDir, TerminationPath: terminationPath,
		ResultExtractionMethod: ResultExtractionMethodTerminationMessage,
	}
	if err := e.Go(); err != nil {
		t.Fatal(err)
	}
	if !released {
		t.Fatal("next step was not released")
	}
	if d := cmp.Diff(map[string]string{"output": "first"}, taskResultsInMessage(t, terminationPath)); d != "" {
		t.Errorf("results after release (-want +got): %s", d)
	}
}

func TestEntrypointerResultCollectionFailureStopsNextStep(t *testing.T) {
	for _, onError := range []string{FailOnError, ContinueOnError} {
		t.Run(onError, func(t *testing.T) {
			resultsDir, stepDir := t.TempDir(), t.TempDir()
			pw := &fakePostWriter{}
			e := Entrypointer{
				Waiter: &fakeWaiter{}, PostWriter: pw, PostFile: filepath.Join(stepDir, "out"),
				Runner:  &fakeResultsWriter{resultsToWrite: map[string]string{filepath.Join(resultsDir, "output"): strings.Repeat("x", termination.MaxContainerTerminationMessageLength)}},
				Results: []string{"output"}, ResultsDirectory: resultsDir,
				StepMetadataDir: stepDir, TerminationPath: filepath.Join(stepDir, "termination"),
				OnError: onError, ResultExtractionMethod: ResultExtractionMethodTerminationMessage,
			}
			var lengthErr termination.MessageLengthError
			if err := e.Go(); !errors.As(err, &lengthErr) {
				t.Fatalf("got %v, want termination length error", err)
			}
			if pw.wrote == nil || *pw.wrote != e.PostFile+".err" {
				t.Fatalf("expected failure post file, got %v", pw.wrote)
			}
			hashes, err := readTaskResultHashes(stepDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(hashes) != 0 {
				t.Errorf("unpublished results were recorded: %v", hashes)
			}
		})
	}
}

func TestEntrypointerSidecarResultsDoNotUseFingerprints(t *testing.T) {
	resultsDir, stepDir, previousDir := t.TempDir(), t.TempDir(), t.TempDir()
	// Invalid metadata must not affect sidecar-based extraction.
	if err := os.WriteFile(filepath.Join(previousDir, taskResultHashesFile), []byte("invalid"), 0o666); err != nil {
		t.Fatal(err)
	}
	e := Entrypointer{
		Waiter: &fakeWaiter{}, PostWriter: &fakePostWriter{},
		Runner:  &fakeResultsWriter{resultsToWrite: map[string]string{filepath.Join(resultsDir, "output"): "value"}},
		Results: []string{"output"}, ResultsDirectory: resultsDir,
		StepMetadataDir: stepDir, PreviousStepMetadataDir: previousDir,
		TerminationPath: filepath.Join(stepDir, "termination"), ResultExtractionMethod: "sidecar-logs",
	}
	if err := e.Go(); err != nil {
		t.Fatal(err)
	}
	if got := taskResultsInMessage(t, e.TerminationPath); len(got) != 0 {
		t.Errorf("task results in termination message: %v", got)
	}
	if _, err := os.Stat(filepath.Join(stepDir, taskResultHashesFile)); !os.IsNotExist(err) {
		t.Errorf("unexpected fingerprint file: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(resultsDir, "output"))
	if err != nil || string(data) != "value" {
		t.Fatalf("sidecar result file: %q, %v", data, err)
	}
}

func TestEntrypointerInvalidPreviousResults(t *testing.T) {
	previousDir, stepDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(previousDir, taskResultHashesFile), []byte("invalid"), 0o666); err != nil {
		t.Fatal(err)
	}
	ran := false
	pw := &fakePostWriter{}
	e := Entrypointer{
		Waiter: &fakeWaiter{}, PostWriter: pw, PostFile: filepath.Join(stepDir, "out"),
		Runner:  resultRunnerFunc(func(context.Context, ...string) error { ran = true; return nil }),
		Results: []string{"output"}, ResultsDirectory: t.TempDir(),
		StepMetadataDir: stepDir, PreviousStepMetadataDir: previousDir,
		TerminationPath: filepath.Join(stepDir, "termination"), ResultExtractionMethod: ResultExtractionMethodTerminationMessage,
	}
	if err := e.Go(); err == nil || !strings.Contains(err.Error(), "parse previous task result fingerprints") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ran {
		t.Error("ran command after metadata error")
	}
	if pw.wrote == nil || *pw.wrote != e.PostFile+".err" {
		t.Error("did not signal failure")
	}
}

type resultRunnerFunc func(context.Context, ...string) error

func (f resultRunnerFunc) Run(ctx context.Context, args ...string) error { return f(ctx, args...) }

type resultPostWriterFunc func(string, string)

func (f resultPostWriterFunc) Write(file, content string) { f(file, content) }

func TestEntrypointerSignsOnlyReportedTaskResults(t *testing.T) {
	resultsDir := t.TempDir()
	signClient, verifyClient, tr := getMockSpireClient(t.Context())
	previousDir := ""
	for _, name := range []string{"first", "second"} {
		stepDir := t.TempDir()
		e := Entrypointer{
			Waiter: &fakeWaiter{}, PostWriter: &fakePostWriter{},
			Runner:  &fakeResultsWriter{resultsToWrite: map[string]string{filepath.Join(resultsDir, name): name}},
			Results: []string{"first", "second"}, ResultsDirectory: resultsDir,
			StepMetadataDir: stepDir, PreviousStepMetadataDir: previousDir,
			TerminationPath: filepath.Join(stepDir, "termination"), ResultExtractionMethod: ResultExtractionMethodTerminationMessage,
			SpireWorkloadAPI: signClient,
		}
		if err := e.Go(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(e.TerminationPath)
		if err != nil {
			t.Fatal(err)
		}
		logger, _ := logging.NewLogger("", "signed-results-test")
		entries, err := termination.ParseMessage(logger, string(data))
		if err != nil {
			t.Fatal(err)
		}
		if err := verifyClient.VerifyTaskRunResults(t.Context(), entries, tr); err != nil {
			t.Fatalf("signature verification: %v", err)
		}
		if name == "second" {
			for _, r := range entries {
				if r.Key == "first" {
					t.Error("re-emitted the first step's result")
				}
			}
		}
		hashes, err := readTaskResultHashes(stepDir)
		if err != nil {
			t.Fatal(err)
		}
		for key := range hashes {
			if key != "first" && key != "second" {
				t.Errorf("fingerprinted signing metadata: %q", key)
			}
		}
		previousDir = stepDir
	}
}
