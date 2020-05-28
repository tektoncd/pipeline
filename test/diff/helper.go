/*
Copyright 2020 The Tekton Authors

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

package diff

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// ErrorWantGot Wrap cmp.Diff with a t.Helper(). It takes the testing.T
// want,got, format message and cmp.option as an argument. Therefore, if diff was found
// it will call t.Error of format and diff that wrapped by PrintWantGot func
func ErrorWantGot(t *testing.T, want interface{}, got interface{}, format string, opts ...cmp.Option) {
	t.Helper()
	if d := cmp.Diff(want, got, opts...); d != "" {
		t.Errorf(fmt.Sprintf("%s %s", format, PrintWantGot(d)))
	}
}

// FatalWantGot Wrap cmp.Diff with a t.Helper(). It takes the testing.T
// want,got, format message and cmp.option as an argument. Therefore, if diff was found
// it will call t.Fatalf of format and diff that wrapped by PrintWantGot func
func FatalWantGot(t *testing.T, want interface{}, got interface{}, format string, opts ...cmp.Option) {
	t.Helper()
	if d := cmp.Diff(want, got, opts...); d != "" {
		t.Fatalf(fmt.Sprintf("%s %s", format, PrintWantGot(d)))
	}
}
