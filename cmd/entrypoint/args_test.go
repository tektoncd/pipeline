/*
Copyright 2022 The Tekton Authors

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

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExtractArgs(t *testing.T) {
	for _, c := range []struct {
		desc                string
		args                []string
		expectedArgs        []string
		expectedCommandArgs []string
	}{{
		desc:                "empty arguments",
		args:                []string{},
		expectedArgs:        []string{},
		expectedCommandArgs: []string{},
	}, {
		desc:                "with args",
		args:                []string{"--foo", "bar", "a", "b", "c"},
		expectedArgs:        []string{"--foo", "bar", "a", "b", "c"},
		expectedCommandArgs: []string{},
	}, {
		desc:                "with terminator on its own",
		args:                []string{"--foo", "bar", "--"},
		expectedArgs:        []string{"--foo", "bar"},
		expectedCommandArgs: []string{},
	}, {
		desc:                "with terminator",
		args:                []string{"--foo", "bar", "--", "a", "b", "c"},
		expectedArgs:        []string{"--foo", "bar"},
		expectedCommandArgs: []string{"a", "b", "c"},
	}, {
		desc:                "with args and terminator",
		args:                []string{"--foo", "bar", "baz", "--", "a", "b", "c"},
		expectedArgs:        []string{"--foo", "bar", "baz"},
		expectedCommandArgs: []string{"a", "b", "c"},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			args, commandArgs := extractArgs(c.args)
			if d := cmp.Diff(c.expectedArgs, args); d != "" {
				t.Errorf("args: diff(-want,+got):\n%s", d)
			}
			if d := cmp.Diff(c.expectedCommandArgs, commandArgs); d != "" {
				t.Errorf("commandArgs: diff(-want,+got):\n%s", d)
			}
		})
	}
}
