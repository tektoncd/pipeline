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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
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
			if terminationFile, err := ioutil.TempFile("", "termination"); err != nil {
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
			if terminationFile, err := ioutil.TempFile("", "termination"); err != nil {
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
			fileContents, err := ioutil.ReadFile(terminationPath)
			if err == nil {
				var entries []v1alpha1.PipelineResourceResult
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

func TestEntrypointer_ReadBreakpointExitCodeFromDisk(t *testing.T) {
	expectedExitCode := 1
	// setup test
	tmp, err := ioutil.TempFile("", "1*.err")
	if err != nil {
		t.Errorf("error while creating temp file for testing exit code written by breakpoint")
	}
	// write exit code to file
	if err = ioutil.WriteFile(tmp.Name(), []byte(fmt.Sprintf("%d", expectedExitCode)), 0700); err != nil {
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
			if terminationFile, err := ioutil.TempFile("", "termination"); err != nil {
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

type fakeWaiter struct{ waited []string }

func (f *fakeWaiter) Wait(file string, _ bool, _ bool) error {
	f.waited = append(f.waited, file)
	return nil
}

type fakeRunner struct{ args *[]string }

func (f *fakeRunner) Run(ctx context.Context, args ...string) error {
	f.args = &args
	return nil
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

func (f *fakeErrorWaiter) Wait(file string, expectContent bool, breakpointOnFailure bool) error {
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
