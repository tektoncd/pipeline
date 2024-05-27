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
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	logtesting "knative.dev/pkg/logging/testing"
)

// SetupFakeContext sets up the the Context and the fake informers for the tests.
// The optional fs() can be used to edit ctx before the SetupInformer steps
func SetupFakeContext(t testing.TB, fs ...func(context.Context) context.Context) (context.Context, []controller.Informer) {
	c, _, is := SetupFakeContextWithCancel(t, fs...)
	return c, is
}

// SetupFakeContextWithCancel sets up the the Context and the fake informers for the tests
// The provided context can be canceled using provided callback.
// The optional fs() can be used to edit ctx before the SetupInformer steps
func SetupFakeContextWithCancel(t testing.TB, fs ...func(context.Context) context.Context) (context.Context, context.CancelFunc, []controller.Informer) {
	ctx, c := context.WithCancel(logtesting.TestContextWithLogger(t))
	ctx = controller.WithEventRecorder(ctx, record.NewFakeRecorder(1000))
	for _, f := range fs {
		ctx = f(ctx)
	}
	ctx = injection.WithConfig(ctx, &rest.Config{})

	ctx, is := injection.Fake.SetupInformers(ctx, injection.GetConfig(ctx))
	return ctx, c, is
}

// fakeClient is an interface capturing the two functions we need from fake clients.
type fakeClient interface {
	PrependWatchReactor(resource string, reaction clientgotesting.WatchReactionFunc)
	PrependReactor(verb, resource string, reaction clientgotesting.ReactionFunc)
}

// withTracker is an interface capturing only the Tracker function. The dynamic client
// currently does not have that, so we need to special-case it.
type withTracker interface {
	Tracker() clientgotesting.ObjectTracker
}

// RunAndSyncInformers runs the given informers, then makes sure their caches are all
// synced and in addition makes sure that all the Watch calls have been properly setup.
// See https://github.com/kubernetes/kubernetes/issues/95372 for background on the Watch
// calls tragedy.
func RunAndSyncInformers(ctx context.Context, informers ...controller.Informer) (func(), error) {
	var watchesPending atomic.Int32

	for _, client := range injection.Fake.FetchAllClients(ctx) {
		c := client.(fakeClient)

		var tracker clientgotesting.ObjectTracker
		if withTracker, ok := c.(withTracker); ok {
			tracker = withTracker.Tracker()
		} else {
			// Required setup for the dynamic client as it doesn't define a Tracker() function.
			// TODO(markusthoemmes): Drop this if https://github.com/kubernetes/kubernetes/pull/100085 lands.
			scheme := runtime.NewScheme()
			scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "fake-dynamic-client-group", Version: "v1", Kind: "List"}, &unstructured.UnstructuredList{})
			codecs := serializer.NewCodecFactory(scheme)
			tracker = clientgotesting.NewObjectTracker(scheme, codecs.UniversalDecoder())
		}

		c.PrependReactor("list", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			// Every list (before actual informer usage) is going to be followed by a Watch call.
			watchesPending.Add(1)
			return false, nil, nil
		})

		c.PrependWatchReactor("*", func(action clientgotesting.Action) (handled bool, ret watch.Interface, err error) {
			// The actual Watch call. This is a reimplementation of the default Watch
			// calls in fakes to guarantee we have actually **done** the work.
			gvr := action.GetResource()
			ns := action.GetNamespace()
			watch, err := tracker.Watch(gvr, ns)
			if err != nil {
				return false, nil, err
			}

			watchesPending.Add(-1)

			return true, watch, nil
		})
	}

	wf, err := controller.RunInformers(ctx.Done(), informers...)
	if err != nil {
		return wf, err
	}

	err = wait.PollUntilContextTimeout(ctx, time.Microsecond, wait.ForeverTestTimeout, true, func(ctx context.Context) (bool, error) {
		if watchesPending.Load() == 0 {
			return true, nil
		}
		return false, nil
	})
	return wf, err
}
