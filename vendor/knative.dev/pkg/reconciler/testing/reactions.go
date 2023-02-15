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

package testing

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
)

// InduceFailure is used in conjunction with TableTest's WithReactors field.
// Tests that want to induce a failure in a row of a TableTest would add:
//
//	WithReactors: []clientgotesting.ReactionFunc{
//	   // Makes calls to create revisions return an error.
//	   InduceFailure("create", "revisions"),
//	},
//
// Or to target a subresource, say a patch to InMemoryChannel.Status, you would add:
//
//	WithReactors: []clientgotesting.ReactionFunc{
//	   // Makes calls to patch inmemorychannels status subresource return an error.
//	   InduceFailure("patch", "inmemorychannels/status"),
//	},
func InduceFailure(verb, resource string) clientgotesting.ReactionFunc {
	return func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if !action.Matches(verb, resource) {
			return false, nil, nil
		}
		return true, nil, fmt.Errorf("inducing failure for %s %s", action.GetVerb(), action.GetResource().Resource)
	}
}

func ValidateCreates(ctx context.Context, action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
	got := action.(clientgotesting.CreateAction).GetObject()
	obj, ok := got.(apis.Validatable)
	if !ok {
		return false, nil, nil
	}
	// Only return error-level errors; warnings should not block API calls.
	if err := obj.Validate(ctx).Filter(apis.ErrorLevel); err != nil {
		return true, nil, err
	}
	return false, nil, nil
}

func ValidateUpdates(ctx context.Context, action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
	got := action.(clientgotesting.UpdateAction).GetObject()
	obj, ok := got.(apis.Validatable)
	if !ok {
		return false, nil, nil
	}
	// Only return error-level errors; warnings should not block API calls.
	if err := obj.Validate(ctx).Filter(apis.ErrorLevel); err != nil {
		return true, nil, err
	}
	return false, nil, nil
}
