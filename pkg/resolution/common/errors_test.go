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

package common_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	common "github.com/tektoncd/pipeline/pkg/resolution/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TestError struct{}

var _ error = &TestError{}

func (*TestError) Error() string {
	return "test error"
}

func TestResolutionErrorUnwrap(t *testing.T) {
	originalError := &TestError{}
	resolutionError := common.NewError("", originalError)
	if !errors.Is(resolutionError, &TestError{}) {
		t.Errorf("resolution error expected to unwrap successfully")
	}
}

func TestResolutionErrorMessage(t *testing.T) {
	originalError := errors.New("this is just a test message")
	resolutionError := common.NewError("", originalError)
	if resolutionError.Error() != originalError.Error() {
		t.Errorf("resolution error message expected to equal that of original error")
	}
}

func TestIsErrTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{{
		name: "conflict",
		err:  apierrors.NewConflict(schema.GroupResource{Resource: "tasks"}, "foo", errors.New("conflict")),
		want: true,
	}, {
		name: "too many requests",
		err:  apierrors.NewTooManyRequests("busy", 1),
		want: true,
	}, {
		name: "etcd leader change",
		err:  errors.New("error requesting remote resource: rpc error: etcdserver: leader changed"),
		want: true,
	}, {
		name: "sqlite database is locked",
		err:  errors.New("error requesting remote resource: rpc error: code = Unknown desc = exec (try: 500): database is locked"),
		want: true,
	}, {
		name: "context deadline exceeded",
		err:  fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
		want: true,
	}, {
		name: "not found is not transient",
		err:  apierrors.NewNotFound(schema.GroupResource{Resource: "tasks"}, "foo"),
		want: false,
	}, {
		name: "arbitrary error is not transient",
		err:  errors.New("some other error"),
		want: false,
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := common.IsErrTransient(tc.err); got != tc.want {
				t.Errorf("IsErrTransient(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
