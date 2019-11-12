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
	"reflect"
	"strings"
	"testing"

	"golang.org/x/xerrors"
)

func TestEntrypointerFailures(t *testing.T) {
	for _, c := range []struct {
		desc, postFile string
		errorStrategy  ErrorStrategy
		waitFiles      []string
		waiter         Waiter
		runner         Runner
		expectedError  string
	}{{
		desc:          "failing runner with no postFile",
		runner:        &fakeErrorRunner{},
		expectedError: "runner failed",
	}, {
		desc:          "failing runner with postFile",
		runner:        &fakeErrorRunner{},
		expectedError: "runner failed",
		postFile:      "foo",
	}, {
		desc:          "failing waiter with no postFile",
		waitFiles:     []string{"foo"},
		waiter:        &fakeErrorWaiter{},
		expectedError: "waiter failed",
	}, {
		desc:          "failing waiter with postFile",
		waitFiles:     []string{"foo"},
		waiter:        &fakeErrorWaiter{},
		expectedError: "waiter failed",
		postFile:      "bar",
	}, {
		desc:          "invalid error strategy",
		runner:        &fakeErrorRunner{},
		errorStrategy: "foo_strategy",
		expectedError: "invalid error strategy",
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
				Entrypoint:    "echo",
				WaitFiles:     c.waitFiles,
				PostFile:      c.postFile,
				ErrorStrategy: c.errorStrategy,
				Args:          []string{"some", "args"},
				Waiter:        fw,
				Runner:        fr,
				PostWriter:    fpw,
			}.Go()
			if err == nil {
				t.Fatalf("Entrpointer didn't fail")
			}
			if !strings.Contains(err.Error(), c.expectedError) {
				t.Errorf("Expected error containing %q, received %q", c.expectedError, err.Error())
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
		errorStrategy              ErrorStrategy
		waitFiles, args            []string
		fakeWaiter                 *fakeWaiter
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
	}, {
		desc:          "ignore errors from prior steps",
		fakeWaiter:    &fakeWaiter{waitError: SkipError("an error from prior step")},
		waitFiles:     []string{"waitforme"},
		errorStrategy: IgnorePriorStepErrors,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fw := c.fakeWaiter
			if fw == nil {
				fw = &fakeWaiter{}
			}
			fr, fpw := &fakeRunner{}, &fakePostWriter{}
			err := Entrypointer{
				Entrypoint:    c.entrypoint,
				WaitFiles:     c.waitFiles,
				PostFile:      c.postFile,
				ErrorStrategy: c.errorStrategy,
				Args:          c.args,
				Waiter:        fw,
				Runner:        fr,
				PostWriter:    fpw,
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
		})
	}
}

type fakeWaiter struct {
	waitError error
	waited    []string
}

func (f *fakeWaiter) Wait(file string, _ bool) error {
	f.waited = append(f.waited, file)
	if f.waitError != nil {
		return f.waitError
	}
	return nil
}

type fakeRunner struct{ args *[]string }

func (f *fakeRunner) Run(args ...string) error {
	f.args = &args
	return nil
}

type fakePostWriter struct{ wrote *string }

func (f *fakePostWriter) Write(file string) { f.wrote = &file }

type fakeErrorWaiter struct{ waited *string }

func (f *fakeErrorWaiter) Wait(file string, expectContent bool) error {
	f.waited = &file
	return xerrors.New("waiter failed")
}

type fakeErrorRunner struct{ args *[]string }

func (f *fakeErrorRunner) Run(args ...string) error {
	f.args = &args
	return xerrors.New("runner failed")
}
