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

package names_test

import (
	"strings"
	"testing"

	pkgnames "github.com/tektoncd/pipeline/pkg/names"
	"github.com/tektoncd/pipeline/test/names"
)

func TestRestrictLengthWithRandomSuffix(t *testing.T) {
	for _, c := range []struct {
		in, want string
	}{{
		in:   "hello",
		want: "hello-9l9zj",
	}, {
		in:   strings.Repeat("a", 100),
		want: strings.Repeat("a", 57) + "-9l9zj",
	}} {
		t.Run(c.in, func(t *testing.T) {
			names.TestingSeed()
			got := pkgnames.SimpleNameGenerator.RestrictLengthWithRandomSuffix(c.in)
			if got != c.want {
				t.Errorf("RestrictLengthWithRandomSuffix:\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestRestrictLength(t *testing.T) {
	for _, c := range []struct {
		in, want string
	}{{
		in:   "hello",
		want: "hello",
	}, {
		in:   strings.Repeat("a", 100),
		want: strings.Repeat("a", 63),
	}, {
		// Values that don't end with an alphanumeric value are
		// trimmed until they do.
		in:   "abcdefg   !@#!$",
		want: "abcdefg",
	}} {
		t.Run(c.in, func(t *testing.T) {
			got := pkgnames.SimpleNameGenerator.RestrictLength(c.in)
			if got != c.want {
				t.Errorf("RestrictLength:\n got %q\nwant %q", got, c.want)
			}
		})
	}
}
