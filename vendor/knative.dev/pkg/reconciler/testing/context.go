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
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	logtesting "knative.dev/pkg/logging/testing"
)

// SetupFakeContext sets up the the Context and the fake informers for the tests.
func SetupFakeContext(t *testing.T) (context.Context, []controller.Informer) {
	c, _, is := SetupFakeContextWithCancel(t)
	return c, is
}

// SetupFakeContextWithCancel sets up the the Context and the fake informers for the tests
// The provided context can be canceled using provided callback.
func SetupFakeContextWithCancel(t *testing.T) (context.Context, context.CancelFunc, []controller.Informer) {
	ctx, c := context.WithCancel(logtesting.TestContextWithLogger(t))
	ctx = controller.WithEventRecorder(ctx, record.NewFakeRecorder(1000))
	ctx, is := injection.Fake.SetupInformers(ctx, &rest.Config{})
	return ctx, c, is
}
