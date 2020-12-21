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
	"io/ioutil"
	"os"
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
			err := Entrypointer{
				Entrypoint:      "echo",
				WaitFiles:       c.waitFiles,
				PostFile:        c.postFile,
				Args:            []string{"some", "args"},
				Waiter:          fw,
				Runner:          fr,
				PostWriter:      fpw,
				TerminationPath: "termination",
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
		desc, entrypoint, postFile string
		waitFiles, args            []string
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
	}, {
		desc:       "all together now",
		entrypoint: "echo", args: []string{"some", "args"},
		waitFiles: []string{"waitforme"},
		postFile:  "writeme",
	}, {
		desc:      "multiple wait files",
		waitFiles: []string{"waitforme", "metoo", "methree"},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fw, fr, fpw := &fakeWaiter{}, &fakeRunner{}, &fakePostWriter{}
			timeout := time.Duration(0)
			err := Entrypointer{
				Entrypoint:      c.entrypoint,
				WaitFiles:       c.waitFiles,
				PostFile:        c.postFile,
				Args:            c.args,
				Waiter:          fw,
				Runner:          fr,
				PostWriter:      fpw,
				TerminationPath: "termination",
				Timeout:         &timeout,
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

			wantArgs := c.args
			if c.entrypoint != "" {
				wantArgs = append([]string{c.entrypoint}, c.args...)
			}
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
			}
			if c.postFile == "" && fpw.wrote != nil {
				t.Errorf("Wrote post file when not required")
			}
			fileContents, err := ioutil.ReadFile("termination")
			if err == nil {
				var entries []v1alpha1.PipelineResourceResult
				if err := json.Unmarshal([]byte(fileContents), &entries); err == nil {
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
			if err := os.Remove("termination"); err != nil {
				t.Errorf("Could not remove termination path: %s", err)
			}
		})
	}
}

type fakeWaiter struct{ waited []string }

func (f *fakeWaiter) Wait(file string, _ bool) error {
	f.waited = append(f.waited, file)
	return nil
}

type fakeRunner struct{ args *[]string }

func (f *fakeRunner) Run(ctx context.Context, args ...string) error {
	f.args = &args
	return nil
}

type fakePostWriter struct{ wrote *string }

func (f *fakePostWriter) Write(file string) { f.wrote = &file }

type fakeErrorWaiter struct{ waited *string }

func (f *fakeErrorWaiter) Wait(file string, expectContent bool) error {
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
