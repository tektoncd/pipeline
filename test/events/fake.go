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

package events

import (
	"context"
	"sync"
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	logtesting "knative.dev/pkg/logging/testing"
)

// FakeRecorder is used as a fake during tests. It is thread safe. It is usable
// when created manually and not by NewFakeRecorder, however all events may be
// thrown away in this case.
// use addCount and decreaseCount for if event is sent in gorontines
type FakeRecorder struct {
	record.FakeRecorder
	// waitGroup is used to block until all events have been sent
	waitGroup *sync.WaitGroup
}

// NewFakeRecorder creates new fake event recorder with event channel with
// buffer of given size.
func NewFakeRecorder(bufferSize int) *FakeRecorder {
	return &FakeRecorder{
		FakeRecorder: record.FakeRecorder{
			Events: make(chan string, bufferSize),
		},
		waitGroup: &sync.WaitGroup{},
	}
}

// addCount can be used to add the count when each event is going to be sent
func (f FakeRecorder) addCount() {
	f.waitGroup.Add(1)
}

// decreaseCount can be used to the decrease the count when each event is sent
func (f FakeRecorder) decreaseCount() {
	f.waitGroup.Done()
}

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
	ctx = controller.WithEventRecorder(ctx, NewFakeRecorder(1000))
	for _, f := range fs {
		ctx = f(ctx)
	}
	ctx, is := injection.Fake.SetupInformers(ctx, &rest.Config{})
	return ctx, c, is
}
