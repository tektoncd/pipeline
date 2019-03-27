/*
Copyright 2019 The Knative Authors.

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
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEntrypointerFailures(t *testing.T) {
	for _, c := range []struct {
		desc, waitFile, postFile string
		waiter                   Waiter
		runner                   Runner
		expectedError            string
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
		waitFile:      "foo",
		waiter:        &fakeErrorWaiter{},
		expectedError: "waiter failed",
	}, {
		desc:          "failing waiter with postFile",
		waitFile:      "foo",
		waiter:        &fakeErrorWaiter{},
		expectedError: "waiter failed",
		postFile:      "bar",
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
				Entrypoint: "echo",
				WaitFile:   c.waitFile,
				PostFile:   c.postFile,
				Args:       []string{"some", "args"},
				Waiter:     fw,
				Runner:     fr,
				PostWriter: fpw,
			}.Go()
			if err == nil {
				t.Fatalf("Entrpointer didn't fail")
			}
			if d := cmp.Diff(c.expectedError, err.Error()); d != "" {
				t.Errorf("Entrypointer error diff -want, +got: %v", d)
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
		desc, entrypoint, waitFile, postFile string
		args                                 []string
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
		desc:     "wait file",
		waitFile: "waitforme",
	}, {
		desc:     "post file",
		postFile: "writeme",
	}, {
		desc:       "all together now",
		entrypoint: "echo", args: []string{"some", "args"},
		waitFile: "waitforme",
		postFile: "writeme",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fw, fr, fpw := &fakeWaiter{}, &fakeRunner{}, &fakePostWriter{}
			err := Entrypointer{
				Entrypoint: c.entrypoint,
				WaitFile:   c.waitFile,
				PostFile:   c.postFile,
				Args:       c.args,
				Waiter:     fw,
				Runner:     fr,
				PostWriter: fpw,
			}.Go()
			if err != nil {
				t.Fatalf("Entrypointer failed: %v", err)
			}

			if c.waitFile != "" {
				if fw.waited == nil {
					t.Error("Wanted waited file, got nil")
				} else if *fw.waited != c.waitFile {
					t.Errorf("Waited for %q, want %q", *fw.waited, c.waitFile)
				}
			}
			if c.waitFile == "" && fw.waited != nil {
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

type fakeWaiter struct{ waited *string }

func (f *fakeWaiter) Wait(file string) error {
	f.waited = &file
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

func (f *fakeErrorWaiter) Wait(file string) error {
	f.waited = &file
	return fmt.Errorf("waiter failed")
}

type fakeErrorRunner struct{ args *[]string }

func (f *fakeErrorRunner) Run(args ...string) error {
	f.args = &args
	return fmt.Errorf("runner failed")
}
