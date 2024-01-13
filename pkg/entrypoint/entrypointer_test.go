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

package entrypoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/pkg/spire"
	"github.com/tektoncd/pipeline/pkg/termination"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

func TestEntrypointerFailures(t *testing.T) {
	for _, c := range []struct {
		desc, postFile string
		waitFiles      []string
		waiter         Waiter
		runner         Runner
		expectedError  string
		timeout        time.Duration
	}{{
		desc:          "failing runner with postFile",
		runner:        &fakeErrorRunner{},
		expectedError: "runner failed",
		postFile:      "foo",
		timeout:       time.Duration(0),
	}, {
		desc:          "failing waiter with no postFile",
		waitFiles:     []string{"foo"},
		waiter:        &fakeErrorWaiter{},
		expectedError: "waiter failed",
		timeout:       time.Duration(0),
	}, {
		desc:          "failing waiter with postFile",
		waitFiles:     []string{"foo"},
		waiter:        &fakeErrorWaiter{},
		expectedError: "waiter failed",
		postFile:      "bar",
		timeout:       time.Duration(0),
	}, {
		desc:          "negative timeout",
		runner:        &fakeErrorRunner{},
		timeout:       -10 * time.Second,
		expectedError: `negative timeout specified`,
	}, {
		desc:          "zero timeout string does not time out",
		runner:        &fakeZeroTimeoutRunner{},
		timeout:       time.Duration(0),
		expectedError: `runner failed`,
	}, {
		desc:          "timeout leads to runner",
		runner:        &fakeTimeoutRunner{},
		timeout:       1 * time.Millisecond,
		expectedError: `runner failed`,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fw := c.waiter
			if fw == nil {
				fw = &fakeWaiter{}
			}
			fr := c.runner
			if fr == nil {
				fr = &fakeRunner{}
			}
			fpw := &fakePostWriter{}
			terminationPath := "termination"
			if terminationFile, err := os.CreateTemp("", "termination"); err != nil {
				t.Fatalf("unexpected error creating temporary termination file: %v", err)
			} else {
				terminationPath = terminationFile.Name()
				defer os.Remove(terminationFile.Name())
			}
			err := Entrypointer{
				Command:         []string{"echo", "some", "args"},
				WaitFiles:       c.waitFiles,
				PostFile:        c.postFile,
				Waiter:          fw,
				Runner:          fr,
				PostWriter:      fpw,
				TerminationPath: terminationPath,
				Timeout:         &c.timeout,
			}.Go()
			if err == nil {
				t.Fatalf("Entrypointer didn't fail")
			}
			if d := cmp.Diff(c.expectedError, err.Error()); d != "" {
				t.Errorf("Entrypointer error diff %s", diff.PrintWantGot(d))
			}

			if c.postFile != "" {
				if fpw.wrote == nil {
					t.Error("Wanted post file written, got nil")
				} else if *fpw.wrote != c.postFile+".err" {
					t.Errorf("Wrote post file %q, want %q", *fpw.wrote, c.postFile)
				}
			}
			if c.postFile == "" && fpw.wrote != nil {
				t.Errorf("Wrote post file when not required")
			}
		})
	}
}

func TestEntrypointer(t *testing.T) {
	for _, c := range []struct {
		desc, entrypoint, postFile, stepDir, stepDirLink string
		waitFiles, args                                  []string
		breakpointOnFailure                              bool
	}{{
		desc: "do nothing",
	}, {
		desc:       "just entrypoint",
		entrypoint: "echo",
	}, {
		desc:       "entrypoint and args",
		entrypoint: "echo", args: []string{"some", "args"},
	}, {
		desc: "just args",
		args: []string{"just", "args"},
	}, {
		desc:      "wait file",
		waitFiles: []string{"waitforme"},
	}, {
		desc:     "post file",
		postFile: "writeme",
		stepDir:  ".",
	}, {
		desc:       "all together now",
		entrypoint: "echo", args: []string{"some", "args"},
		waitFiles: []string{"waitforme"},
		postFile:  "writeme",
		stepDir:   ".",
	}, {
		desc:      "multiple wait files",
		waitFiles: []string{"waitforme", "metoo", "methree"},
	}, {
		desc:                "breakpointOnFailure to wait or not to wait ",
		breakpointOnFailure: true,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fw, fr, fpw := &fakeWaiter{}, &fakeRunner{}, &fakePostWriter{}
			timeout := time.Duration(0)
			terminationPath := "termination"
			if terminationFile, err := os.CreateTemp("", "termination"); err != nil {
				t.Fatalf("unexpected error creating temporary termination file: %v", err)
			} else {
				terminationPath = terminationFile.Name()
				defer os.Remove(terminationFile.Name())
			}
			err := Entrypointer{
				Command:             append([]string{c.entrypoint}, c.args...),
				WaitFiles:           c.waitFiles,
				PostFile:            c.postFile,
				Waiter:              fw,
				Runner:              fr,
				PostWriter:          fpw,
				TerminationPath:     terminationPath,
				Timeout:             &timeout,
				BreakpointOnFailure: c.breakpointOnFailure,
				StepMetadataDir:     c.stepDir,
			}.Go()
			if err != nil {
				t.Fatalf("Entrypointer failed: %v", err)
			}

			if len(c.waitFiles) > 0 {
				if fw.waited == nil {
					t.Error("Wanted waited file, got nil")
				} else if !reflect.DeepEqual(fw.waited, c.waitFiles) {
					t.Errorf("Waited for %v, want %v", fw.waited, c.waitFiles)
				}
			}
			if len(c.waitFiles) == 0 && fw.waited != nil {
				t.Errorf("Waited for file when not required")
			}

			wantArgs := append([]string{c.entrypoint}, c.args...)
			if len(wantArgs) != 0 {
				if fr.args == nil {
					t.Error("Wanted command to be run, got nil")
				} else if !reflect.DeepEqual(*fr.args, wantArgs) {
					t.Errorf("Ran %s, want %s", *fr.args, wantArgs)
				}
			}
			if len(wantArgs) == 0 && c.args != nil {
				t.Errorf("Ran command when not required")
			}

			if c.postFile != "" {
				if fpw.wrote == nil {
					t.Error("Wanted post file written, got nil")
				} else if *fpw.wrote != c.postFile {
					t.Errorf("Wrote post file %q, want %q", *fpw.wrote, c.postFile)
				}

				if d := filepath.Dir(*fpw.wrote); d != c.stepDir {
					t.Errorf("Post file written to wrong directory %q, want %q", d, c.stepDir)
				}
			}
			if c.postFile == "" && fpw.wrote != nil {
				t.Errorf("Wrote post file when not required")
			}
			fileContents, err := os.ReadFile(terminationPath)
			if err == nil {
				var entries []result.RunResult
				if err := json.Unmarshal(fileContents, &entries); err == nil {
					var found = false
					for _, result := range entries {
						if result.Key == "StartedAt" {
							found = true
							break
						}
					}
					if !found {
						t.Error("Didn't find the startedAt entry")
					}
				}
			} else if !os.IsNotExist(err) {
				t.Error("Wanted termination file written, got nil")
			}
			if err := os.Remove(terminationPath); err != nil {
				t.Errorf("Could not remove termination path: %s", err)
			}
		})
	}
}

func TestReadResultsFromDisk(t *testing.T) {
	for _, c := range []struct {
		desc          string
		results       []string
		resultContent []v1beta1.ResultValue
		resultType    result.ResultType
		want          []result.RunResult
	}{{
		desc:          "read string result file",
		results:       []string{"results"},
		resultContent: []v1beta1.ResultValue{*v1beta1.NewStructuredValues("hello world")},
		resultType:    result.TaskRunResultType,
		want: []result.RunResult{
			{Value: `"hello world"`,
				ResultType: 1}},
	}, {
		desc:          "read array result file",
		results:       []string{"results"},
		resultContent: []v1beta1.ResultValue{*v1beta1.NewStructuredValues("hello", "world")},
		resultType:    result.TaskRunResultType,
		want: []result.RunResult{
			{Value: `["hello","world"]`,
				ResultType: 1}},
	}, {
		desc:          "read string and array result files",
		results:       []string{"resultsArray", "resultsString"},
		resultContent: []v1beta1.ResultValue{*v1beta1.NewStructuredValues("hello", "world"), *v1beta1.NewStructuredValues("hello world")},
		resultType:    result.TaskRunResultType,
		want: []result.RunResult{
			{Value: `["hello","world"]`,
				ResultType: 1},
			{Value: `"hello world"`,
				ResultType: 1},
		},
	}, {
		desc:          "read string step result file",
		results:       []string{"results"},
		resultContent: []v1beta1.ResultValue{*v1beta1.NewStructuredValues("hello world")},
		resultType:    result.StepResultType,
		want: []result.RunResult{
			{Value: `"hello world"`,
				ResultType: 4}},
	}, {
		desc:          "read array step result file",
		results:       []string{"results"},
		resultContent: []v1beta1.ResultValue{*v1beta1.NewStructuredValues("hello", "world")},
		resultType:    result.StepResultType,
		want: []result.RunResult{
			{Value: `["hello","world"]`,
				ResultType: 4}},
	}, {
		desc:          "read string and array step result files",
		results:       []string{"resultsArray", "resultsString"},
		resultContent: []v1beta1.ResultValue{*v1beta1.NewStructuredValues("hello", "world"), *v1beta1.NewStructuredValues("hello world")},
		resultType:    result.StepResultType,
		want: []result.RunResult{
			{Value: `["hello","world"]`,
				ResultType: 4},
			{Value: `"hello world"`,
				ResultType: 4},
		},
	},
	} {
		t.Run(c.desc, func(t *testing.T) {
			ctx := context.Background()
			terminationPath := "termination"
			if terminationFile, err := os.CreateTemp("", "termination"); err != nil {
				t.Fatalf("unexpected error creating temporary termination file: %v", err)
			} else {
				terminationPath = terminationFile.Name()
				defer os.Remove(terminationFile.Name())
			}
			resultsFilePath := []string{}
			for i, r := range c.results {
				if resultsFile, err := os.CreateTemp("", r); err != nil {
					t.Fatalf("unexpected error creating temporary termination file: %v", err)
				} else {
					resultName := resultsFile.Name()
					c.want[i].Key = resultName
					resultsFilePath = append(resultsFilePath, resultName)
					d, err := json.Marshal(c.resultContent[i])
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(resultName, d, 0777); err != nil {
						t.Fatal(err)
					}
					defer os.Remove(resultName)
				}
			}

			e := Entrypointer{
				Results:                resultsFilePath,
				StepResults:            resultsFilePath,
				TerminationPath:        terminationPath,
				ResultExtractionMethod: config.ResultExtractionMethodTerminationMessage,
			}
			if err := e.readResultsFromDisk(ctx, "", c.resultType); err != nil {
				t.Fatal(err)
			}
			msg, err := os.ReadFile(terminationPath)
			if err != nil {
				t.Fatal(err)
			}
			logger, _ := logging.NewLogger("", "status")
			got, _ := termination.ParseMessage(logger, string(msg))
			for _, g := range got {
				v := v1beta1.ResultValue{}
				v.UnmarshalJSON([]byte(g.Value))
			}
			if d := cmp.Diff(got, c.want); d != "" {
				t.Fatalf("Diff(-want,+got): %v", d)
			}
		})
	}
}

func TestEntrypointer_ReadBreakpointExitCodeFromDisk(t *testing.T) {
	expectedExitCode := 1
	// setup test
	tmp, err := os.CreateTemp("", "1*.err")
	if err != nil {
		t.Errorf("error while creating temp file for testing exit code written by breakpoint")
	}
	// write exit code to file
	if err = os.WriteFile(tmp.Name(), []byte(fmt.Sprintf("%d", expectedExitCode)), 0700); err != nil {
		t.Errorf("error while writing to temp file create temp file for testing exit code written by breakpoint")
	}
	e := Entrypointer{}
	// test reading the exit code from error waitfile
	actualExitCode, err := e.BreakpointExitCode(tmp.Name())
	if actualExitCode != expectedExitCode {
		t.Errorf("error while parsing exit code. want %d , got %d", expectedExitCode, actualExitCode)
	}
}

func TestEntrypointer_OnError(t *testing.T) {
	for _, c := range []struct {
		desc, postFile, onError string
		runner                  Runner
		expectedError           bool
	}{{
		desc:          "the step is exiting with 1, ignore the step error when onError is set to continue",
		runner:        &fakeExitErrorRunner{},
		postFile:      "step-one",
		onError:       ContinueOnError,
		expectedError: true,
	}, {
		desc:          "the step is exiting with 0, ignore the step error irrespective of no error with onError set to continue",
		runner:        &fakeRunner{},
		postFile:      "step-one",
		onError:       ContinueOnError,
		expectedError: false,
	}, {
		desc:          "the step is exiting with 1, treat the step error as failure with onError set to stopAndFail",
		runner:        &fakeExitErrorRunner{},
		expectedError: true,
		postFile:      "step-one",
		onError:       FailOnError,
	}, {
		desc:          "the step is exiting with 0, treat the step error (but there is none) as failure with onError set to stopAndFail",
		runner:        &fakeRunner{},
		postFile:      "step-one",
		onError:       FailOnError,
		expectedError: false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fpw := &fakePostWriter{}
			terminationPath := "termination"
			if terminationFile, err := os.CreateTemp("", "termination"); err != nil {
				t.Fatalf("unexpected error creating temporary termination file: %v", err)
			} else {
				terminationPath = terminationFile.Name()
				defer os.Remove(terminationFile.Name())
			}
			err := Entrypointer{
				Command:         []string{"echo", "some", "args"},
				WaitFiles:       []string{},
				PostFile:        c.postFile,
				Waiter:          &fakeWaiter{},
				Runner:          c.runner,
				PostWriter:      fpw,
				TerminationPath: terminationPath,
				OnError:         c.onError,
			}.Go()

			if c.expectedError && err == nil {
				t.Fatalf("Entrypointer didn't fail")
			}

			if c.onError == ContinueOnError {
				switch {
				case fpw.wrote == nil:
					t.Error("Wanted post file written, got nil")
				case fpw.exitCodeFile == nil:
					t.Error("Wanted exitCode file written, got nil")
				case *fpw.wrote != c.postFile:
					t.Errorf("Wrote post file %q, want %q", *fpw.wrote, c.postFile)
				case *fpw.exitCodeFile != "exitCode":
					t.Errorf("Wrote exitCode file %q, want %q", *fpw.exitCodeFile, "exitCode")
				case c.expectedError && *fpw.exitCode == "0":
					t.Errorf("Wrote zero exit code but want non-zero when expecting an error")
				}
			}

			if c.onError == FailOnError {
				switch {
				case fpw.wrote == nil:
					t.Error("Wanted post file written, got nil")
				case c.expectedError && *fpw.wrote != c.postFile+".err":
					t.Errorf("Wrote post file %q, want %q", *fpw.wrote, c.postFile+".err")
				}
			}
		})
	}
}

func TestEntrypointerResults(t *testing.T) {
	for _, c := range []struct {
		desc, entrypoint, postFile, stepDir, stepDirLink string
		waitFiles, args                                  []string
		resultsToWrite                                   map[string]string
		resultsOverride                                  []string
		breakpointOnFailure                              bool
		sign                                             bool
		signVerify                                       bool
	}{{
		desc: "do nothing",
	}, {
		desc:       "no results",
		entrypoint: "echo",
	}, {
		desc:       "write single result",
		entrypoint: "echo",
		resultsToWrite: map[string]string{
			"foo": "abc",
		},
	}, {
		desc:       "write single step result",
		entrypoint: "echo",
		resultsToWrite: map[string]string{
			"foo": "abc",
		},
	}, {
		desc:       "write multiple result",
		entrypoint: "echo",
		resultsToWrite: map[string]string{
			"foo": "abc",
			"bar": "def",
		},
	}, {
		// These next two tests show that if not results are defined in the entrypointer, then no signature is produced
		// indicating that no signature was created. However, it is important to note that results were defined,
		// but no results were created, that signature is still produced.
		desc:       "no results signed",
		entrypoint: "echo",
		sign:       true,
		signVerify: false,
	}, {
		desc:            "defined results but no results produced signed",
		entrypoint:      "echo",
		resultsOverride: []string{"foo"},
		sign:            true,
		signVerify:      true,
	}, {
		desc:       "write single result",
		entrypoint: "echo",
		resultsToWrite: map[string]string{
			"foo": "abc",
		},
		sign:       true,
		signVerify: true,
	}, {
		desc:       "write multiple result",
		entrypoint: "echo",
		resultsToWrite: map[string]string{
			"foo": "abc",
			"bar": "def",
		},
		sign:       true,
		signVerify: true,
	}, {
		desc:       "write n/m results",
		entrypoint: "echo",
		resultsToWrite: map[string]string{
			"foo": "abc",
		},
		resultsOverride: []string{"foo", "bar"},
		sign:            true,
		signVerify:      true,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			ctx := context.Background()
			fw, fpw := &fakeWaiter{}, &fakePostWriter{}
			var fr Runner = &fakeRunner{}
			timeout := time.Duration(0)
			terminationPath := "termination"
			if terminationFile, err := os.CreateTemp("", "termination"); err != nil {
				t.Fatalf("unexpected error creating temporary termination file: %v", err)
			} else {
				terminationPath = terminationFile.Name()
				defer os.Remove(terminationFile.Name())
			}

			resultsDir := createTmpDir(t, "results")
			var results []string
			if c.resultsToWrite != nil {
				tmpResultsToWrite := map[string]string{}
				for k, v := range c.resultsToWrite {
					resultFile := path.Join(resultsDir, k)
					tmpResultsToWrite[resultFile] = v
					results = append(results, k)
				}

				fr = &fakeResultsWriter{
					resultsToWrite: tmpResultsToWrite,
				}
			}

			signClient, verifyClient, tr := getMockSpireClient(ctx)
			if !c.sign {
				signClient = nil
			}

			if c.resultsOverride != nil {
				results = c.resultsOverride
			}

			err := Entrypointer{
				Command:                append([]string{c.entrypoint}, c.args...),
				WaitFiles:              c.waitFiles,
				PostFile:               c.postFile,
				Waiter:                 fw,
				Runner:                 fr,
				PostWriter:             fpw,
				Results:                results,
				StepResults:            results,
				ResultsDirectory:       resultsDir,
				ResultExtractionMethod: config.ResultExtractionMethodTerminationMessage,
				TerminationPath:        terminationPath,
				Timeout:                &timeout,
				BreakpointOnFailure:    c.breakpointOnFailure,
				StepMetadataDir:        c.stepDir,
				SpireWorkloadAPI:       signClient,
			}.Go()
			if err != nil {
				t.Fatalf("Entrypointer failed: %v", err)
			}

			fileContents, err := os.ReadFile(terminationPath)
			if err == nil {
				resultCheck := map[string]bool{}
				var entries []result.RunResult
				if err := json.Unmarshal(fileContents, &entries); err != nil {
					t.Fatalf("failed to unmarshal results: %v", err)
				}

				for _, result := range entries {
					if _, ok := c.resultsToWrite[result.Key]; ok {
						if c.resultsToWrite[result.Key] == result.Value {
							resultCheck[result.Key] = true
						} else {
							t.Errorf("expected result (%v) to have value %v, got %v", result.Key, result.Value, c.resultsToWrite[result.Key])
						}
					}
				}

				if len(resultCheck) != len(c.resultsToWrite) {
					t.Error("number of results matching did not add up")
				}

				// Check signature
				verified := verifyClient.VerifyTaskRunResults(ctx, entries, tr) == nil
				if verified != c.signVerify {
					t.Errorf("expected signature verify result %v, got %v", c.signVerify, verified)
				}
			} else if !os.IsNotExist(err) {
				t.Error("Wanted termination file written, got nil")
			}
			if err := os.Remove(terminationPath); err != nil {
				t.Errorf("Could not remove termination path: %s", err)
			}
		})
	}
}

func Test_waitingCancellation(t *testing.T) {
	testCases := []struct {
		name         string
		expectCtxErr error
	}{
		{
			name:         "stopOnCancel is true and want context canceled",
			expectCtxErr: context.Canceled,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			fw := &fakeWaiter{}
			err := Entrypointer{
				Waiter: fw,
			}.waitingCancellation(ctx, cancel)
			if err != nil {
				t.Fatalf("Entrypointer waitingCancellation failed: %v", err)
			}
			if tc.expectCtxErr != nil && !errors.Is(ctx.Err(), tc.expectCtxErr) {
				t.Errorf("expected context error %v, got %v", tc.expectCtxErr, ctx.Err())
			}
		})
	}
}

func TestEntrypointerStopOnCancel(t *testing.T) {
	testCases := []struct {
		name                   string
		runningDuration        time.Duration
		waitingDuration        time.Duration
		runningWaitingDuration time.Duration
		expectError            error
	}{
		{
			name:                   "generally running, expect no error",
			runningDuration:        1 * time.Second,
			runningWaitingDuration: 1 * time.Second,
			waitingDuration:        2 * time.Second,
			expectError:            nil,
		},
		{
			name:                   "context canceled during running, expect context canceled error",
			runningDuration:        2 * time.Second,
			runningWaitingDuration: 2 * time.Second,
			waitingDuration:        1 * time.Second,
			expectError:            ErrContextCanceled,
		},
		{
			name:                   "time exceeded during running, expect context deadline exceeded error",
			runningDuration:        2 * time.Second,
			runningWaitingDuration: 1 * time.Second,
			waitingDuration:        1 * time.Second,
			expectError:            ErrContextDeadlineExceeded,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			terminationPath := "termination"
			if terminationFile, err := os.CreateTemp("", "termination"); err != nil {
				t.Fatalf("unexpected error creating temporary termination file: %v", err)
			} else {
				terminationPath = terminationFile.Name()
				defer os.Remove(terminationFile.Name())
			}
			fw := &fakeWaiter{waitCancelDuration: tc.waitingDuration}
			fr := &fakeLongRunner{runningDuration: tc.runningDuration, waitingDuration: tc.runningWaitingDuration}
			fp := &fakePostWriter{}
			err := Entrypointer{
				Waiter:          fw,
				Runner:          fr,
				PostWriter:      fp,
				TerminationPath: terminationPath,
			}.Go()
			if !errors.Is(err, tc.expectError) {
				t.Errorf("expected error %v, got %v", tc.expectError, err)
			}
		})
	}
}

func TestApplyStepResultSubstitutions_Env(t *testing.T) {
	testCases := []struct {
		name       string
		stepName   string
		resultName string
		result     string
		envValue   string
		want       string
		wantErr    bool
	}{{
		name:       "string param",
		stepName:   "foo",
		resultName: "res",
		result:     "Hello",
		envValue:   "$(steps.foo.results.res)",
		want:       "Hello",
		wantErr:    false,
	}, {
		name:       "array param",
		stepName:   "foo",
		resultName: "res",
		result:     "[\"Hello\",\"World\"]",
		envValue:   "$(steps.foo.results.res[1])",
		want:       "World",
		wantErr:    false,
	}, {
		name:       "object param",
		stepName:   "foo",
		resultName: "res",
		result:     "{\"hello\":\"World\"}",
		envValue:   "$(steps.foo.results.res.hello)",
		want:       "World",
		wantErr:    false,
	}, {
		name:       "bad-result-format",
		stepName:   "foo",
		resultName: "res",
		result:     "{\"hello\":\"World\"}",
		envValue:   "echo $(steps.foo.results.res.hello.bar)",
		want:       "echo $(steps.foo.results.res.hello.bar)",
		wantErr:    true,
	}}
	stepDir := createTmpDir(t, "env-steps")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resultPath := filepath.Join(stepDir, pod.GetContainerName(tc.stepName), "results")
			err := os.MkdirAll(resultPath, 0750)
			if err != nil {
				log.Fatal(err)
			}
			resultFile := filepath.Join(resultPath, tc.resultName)
			err = os.WriteFile(resultFile, []byte(tc.result), 0666)
			if err != nil {
				log.Fatal(err)
			}
			t.Setenv("FOO", tc.envValue)
			e := Entrypointer{
				Command: []string{},
			}
			err = e.applyStepResultSubstitutions(stepDir)
			if tc.wantErr == false && err != nil {
				t.Fatalf("Did not expect and error but got: %v", err)
			} else if tc.wantErr == true && err == nil {
				t.Fatalf("Expected and error but did not get any.")
			}
			got := os.Getenv("FOO")
			if got != tc.want {
				t.Errorf("applyStepResultSubstitutions(): got %v; want %v", got, tc.want)
			}
		})
	}
}

func TestApplyStepResultSubstitutions_Command(t *testing.T) {
	testCases := []struct {
		name       string
		stepName   string
		resultName string
		result     string
		command    []string
		want       []string
		wantErr    bool
	}{{
		name:       "string param",
		stepName:   "foo",
		resultName: "res1",
		result:     "Hello",
		command:    []string{"$(steps.foo.results.res1)"},
		want:       []string{"Hello"},
		wantErr:    false,
	}, {
		name:       "array param",
		stepName:   "foo",
		resultName: "res",
		result:     "[\"Hello\",\"World\"]",
		command:    []string{"$(steps.foo.results.res[1])"},
		want:       []string{"World"},
		wantErr:    false,
	}, {
		name:       "array param no index",
		stepName:   "foo",
		resultName: "res",
		result:     "[\"Hello\",\"World\"]",
		command:    []string{"start", "$(steps.foo.results.res[*])", "stop"},
		want:       []string{"start", "Hello", "World", "stop"},
		wantErr:    false,
	}, {
		name:       "object param",
		stepName:   "foo",
		resultName: "res",
		result:     "{\"hello\":\"World\"}",
		command:    []string{"$(steps.foo.results.res.hello)"},
		want:       []string{"World"},
		wantErr:    false,
	}, {
		name:       "bad-result-format",
		stepName:   "foo",
		resultName: "res",
		result:     "{\"hello\":\"World\"}",
		command:    []string{"echo $(steps.foo.results.res.hello.bar)"},
		want:       []string{"echo $(steps.foo.results.res.hello.bar)"},
		wantErr:    true,
	}}
	stepDir := createTmpDir(t, "command-steps")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resultPath := filepath.Join(stepDir, pod.GetContainerName(tc.stepName), "results")
			err := os.MkdirAll(resultPath, 0750)
			if err != nil {
				log.Fatal(err)
			}
			resultFile := filepath.Join(resultPath, tc.resultName)
			err = os.WriteFile(resultFile, []byte(tc.result), 0666)
			if err != nil {
				log.Fatal(err)
			}
			e := Entrypointer{
				Command: tc.command,
			}
			err = e.applyStepResultSubstitutions(stepDir)
			if tc.wantErr == false && err != nil {
				t.Fatalf("Did not expect and error but got: %v", err)
			} else if tc.wantErr == true && err == nil {
				t.Fatalf("Expected and error but did not get any.")
			}
			got := e.Command
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("Entrypointer error diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestIsContextDeadlineError(t *testing.T) {
	ctxErr := ContextError(context.DeadlineExceeded.Error())
	if !IsContextDeadlineError(ctxErr) {
		t.Errorf("expected context deadline error, got %v", ctxErr)
	}
	normalErr := ContextError("normal error")
	if IsContextDeadlineError(normalErr) {
		t.Errorf("expected normal error, got %v", normalErr)
	}
}

func TestIsContextCanceledError(t *testing.T) {
	ctxErr := ContextError(context.Canceled.Error())
	if !IsContextCanceledError(ctxErr) {
		t.Errorf("expected context canceled error, got %v", ctxErr)
	}
	normalErr := ContextError("normal error")
	if IsContextCanceledError(normalErr) {
		t.Errorf("expected normal error, got %v", normalErr)
	}
}

func TestTerminationReason(t *testing.T) {
	tests := []struct {
		desc              string
		waitFiles         []string
		onError           string
		runError          error
		expectedRunErr    error
		expectedExitCode  *string
		expectedWrotefile *string
		expectedStatus    []result.RunResult
	}{
		{
			desc:              "reason completed",
			expectedExitCode:  ptr("0"),
			expectedWrotefile: ptr("postfile"),
			expectedStatus: []result.RunResult{
				{
					Key:        "StartedAt",
					ResultType: result.InternalTektonResultType,
				},
			},
		},
		{
			desc:              "reason continued",
			onError:           ContinueOnError,
			runError:          ptr(exec.ExitError{}),
			expectedRunErr:    ptr(exec.ExitError{}),
			expectedExitCode:  ptr("-1"),
			expectedWrotefile: ptr("postfile"),
			expectedStatus: []result.RunResult{
				{
					Key:        "ExitCode",
					Value:      "-1",
					ResultType: result.InternalTektonResultType,
				},
				{
					Key:        "StartedAt",
					ResultType: result.InternalTektonResultType,
				},
			},
		},
		{
			desc:              "reason errored",
			runError:          ptr(exec.Error{}),
			expectedRunErr:    ptr(exec.Error{}),
			expectedWrotefile: ptr("postfile.err"),
			expectedStatus: []result.RunResult{
				{
					Key:        "StartedAt",
					ResultType: result.InternalTektonResultType,
				},
			},
		},
		{
			desc:              "reason timedout",
			runError:          ErrContextDeadlineExceeded,
			expectedRunErr:    ErrContextDeadlineExceeded,
			expectedWrotefile: ptr("postfile.err"),
			expectedStatus: []result.RunResult{
				{
					Key:        "Reason",
					Value:      pod.TerminationReasonTimeoutExceeded,
					ResultType: result.InternalTektonResultType,
				},
				{
					Key:        "StartedAt",
					ResultType: result.InternalTektonResultType,
				},
			},
		},
		{
			desc:              "reason skipped",
			waitFiles:         []string{"file"},
			expectedRunErr:    ErrSkipPreviousStepFailed,
			expectedWrotefile: ptr("postfile.err"),
			expectedStatus: []result.RunResult{
				{
					Key:        "Reason",
					Value:      pod.TerminationReasonSkipped,
					ResultType: result.InternalTektonResultType,
				},
				{
					Key:        "StartedAt",
					ResultType: result.InternalTektonResultType,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			fw, fr, fpw := &fakeWaiter{skipStep: true}, &fakeRunner{runError: test.runError}, &fakePostWriter{}

			tmpFolder, err := os.MkdirTemp("", "")
			if err != nil {
				t.Fatalf("unexpected error creating temporary folder: %v", err)
			} else {
				defer os.RemoveAll(tmpFolder)
			}

			terminationFile, err := os.CreateTemp(tmpFolder, "termination")
			if err != nil {
				t.Fatalf("unexpected error creating termination file: %v", err)
			}

			e := Entrypointer{
				Command:             append([]string{}, []string{}...),
				WaitFiles:           test.waitFiles,
				PostFile:            "postfile",
				Waiter:              fw,
				Runner:              fr,
				PostWriter:          fpw,
				TerminationPath:     terminationFile.Name(),
				BreakpointOnFailure: false,
				StepMetadataDir:     tmpFolder,
				OnError:             test.onError,
			}

			err = e.Go()

			if d := cmp.Diff(test.expectedRunErr, err); d != "" {
				t.Fatalf("entrypoint error doesn't match %s", diff.PrintWantGot(d))
			}

			if d := cmp.Diff(test.expectedExitCode, fpw.exitCode); d != "" {
				t.Fatalf("exitCode doesn't match %s", diff.PrintWantGot(d))
			}

			if d := cmp.Diff(test.expectedWrotefile, fpw.wrote); d != "" {
				t.Fatalf("wrote file doesn't match %s", diff.PrintWantGot(d))
			}

			termination, err := getTermination(t, terminationFile.Name())
			if err != nil {
				t.Fatalf("error getting termination output: %v", err)
			}

			if d := cmp.Diff(test.expectedStatus, termination); d != "" {
				t.Fatalf("termination status doesn't match %s", diff.PrintWantGot(d))
			}
		})
	}
}

func getTermination(t *testing.T, terminationFile string) ([]result.RunResult, error) {
	t.Helper()
	fileContents, err := os.ReadFile(terminationFile)
	if err != nil {
		return nil, err
	}

	logger, _ := logging.NewLogger("", "status")
	terminationStatus, err := termination.ParseMessage(logger, string(fileContents))
	if err != nil {
		return nil, err
	}

	for i, termination := range terminationStatus {
		if termination.Key == "StartedAt" {
			terminationStatus[i].Value = ""
		}
	}

	return terminationStatus, nil
}

type fakeWaiter struct {
	sync.Mutex
	waited             []string
	waitCancelDuration time.Duration
	skipStep           bool
}

func (f *fakeWaiter) Wait(ctx context.Context, file string, _ bool, _ bool) error {
	switch {
	case file == pod.DownwardMountCancelFile && f.waitCancelDuration > 0:
		time.Sleep(f.waitCancelDuration)
	case file == pod.DownwardMountCancelFile:
		return nil
	case f.skipStep:
		return ErrSkipPreviousStepFailed
	}

	f.Lock()
	f.waited = append(f.waited, file)
	f.Unlock()
	return nil
}

type fakeRunner struct {
	args     *[]string
	runError error
}

func (f *fakeRunner) Run(ctx context.Context, args ...string) error {
	f.args = &args
	return f.runError
}

type fakePostWriter struct {
	wrote        *string
	exitCodeFile *string
	exitCode     *string
}

func (f *fakePostWriter) Write(file, content string) {
	if content == "" {
		f.wrote = &file
	} else {
		f.exitCodeFile = &file
		f.exitCode = &content
	}
}

type fakeErrorWaiter struct{ waited *string }

func (f *fakeErrorWaiter) Wait(ctx context.Context, file string, expectContent bool, breakpointOnFailure bool) error {
	f.waited = &file
	return errors.New("waiter failed")
}

type fakeErrorRunner struct{ args *[]string }

func (f *fakeErrorRunner) Run(ctx context.Context, args ...string) error {
	f.args = &args
	return errors.New("runner failed")
}

type fakeZeroTimeoutRunner struct{ args *[]string }

func (f *fakeZeroTimeoutRunner) Run(ctx context.Context, args ...string) error {
	f.args = &args
	if _, ok := ctx.Deadline(); ok == true {
		return errors.New("context deadline should not be set with a zero timeout duration")
	}
	return errors.New("runner failed")
}

type fakeTimeoutRunner struct{ args *[]string }

func (f *fakeTimeoutRunner) Run(ctx context.Context, args ...string) error {
	f.args = &args
	if _, ok := ctx.Deadline(); ok == false {
		return errors.New("context deadline should have been set because of a timeout")
	}
	return errors.New("runner failed")
}

type fakeExitErrorRunner struct{ args *[]string }

func (f *fakeExitErrorRunner) Run(ctx context.Context, args ...string) error {
	f.args = &args
	return exec.Command("ls", "/bogus/path").Run()
}

type fakeLongRunner struct {
	runningDuration time.Duration
	waitingDuration time.Duration
}

func (f *fakeLongRunner) Run(ctx context.Context, _ ...string) error {
	if f.waitingDuration < f.runningDuration {
		return ErrContextDeadlineExceeded
	}
	select {
	case <-time.After(f.runningDuration):
		return nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return ErrContextCanceled
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrContextDeadlineExceeded
		}
		return nil
	}
}

type fakeResultsWriter struct {
	args           *[]string
	resultsToWrite map[string]string
}

func (f *fakeResultsWriter) Run(ctx context.Context, args ...string) error {
	f.args = &args
	for k, v := range f.resultsToWrite {
		err := os.WriteFile(k, []byte(v), 0666)
		if err != nil {
			return err
		}
	}
	return nil
}

func createTmpDir(t *testing.T, name string) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", name)
	if err != nil {
		t.Fatalf("unexpected error creating temporary dir: %v", err)
	}
	return tmpDir
}

func getMockSpireClient(ctx context.Context) (spire.EntrypointerAPIClient, spire.ControllerAPIClient, *v1beta1.TaskRun) {
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrun-example",
			Namespace: "foo",
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name:       "taskname",
				APIVersion: "a1",
			},
			ServiceAccountName: "test-sa",
		},
	}

	sc := &spire.MockClient{}

	_ = sc.CreateEntries(ctx, tr, nil, 10000)

	// bootstrap with about 20 calls to sign which should be enough for testing
	id := sc.GetIdentity(tr)
	for i := 0; i < 20; i++ {
		sc.SignIdentities = append(sc.SignIdentities, id)
	}

	return sc, sc, tr
}

func ptr[T any](value T) *T {
	return &value
}
