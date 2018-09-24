/*
Copyright 2018 The Knative Authors

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

package build

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/knative/build/pkg/builder"
	"github.com/knative/build/pkg/builder/nop"
	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/build/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/build/pkg/client/informers/externalversions"
)

const (
	noErrorMessage = ""
)

const (
	noResyncPeriod time.Duration = 0
)

type fixture struct {
	t *testing.T

	client     *fake.Clientset
	kubeclient *k8sfake.Clientset
	// Objects to put in the store.
	buildLister []*v1alpha1.Build
	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
	objects     []runtime.Object
	eventCh     chan string
}

func newBuild(name string) *v1alpha1.Build {
	return &v1alpha1.Build{
		TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1alpha1.BuildSpec{
			Timeout: metav1.Duration{Duration: 20 * time.Minute},
		},
	}
}

func (f *fixture) newController(b builder.Interface, eventCh chan string) (*Controller, informers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	i := informers.NewSharedInformerFactory(f.client, noResyncPeriod)
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriod)
	logger := zap.NewExample().Sugar()
	c := NewController(b, f.kubeclient, f.client, k8sI, i, logger).(*Controller)

	c.buildsSynced = func() bool { return true }
	c.recorder = &record.FakeRecorder{
		Events: eventCh,
	}

	return c, i, k8sI
}

func (f *fixture) updateIndex(i informers.SharedInformerFactory, bl []*v1alpha1.Build) {
	for _, f := range bl {
		i.Build().V1alpha1().Builds().Informer().GetIndexer().Add(f)
	}
}

func getKey(build *v1alpha1.Build, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(build)
	if err != nil {
		t.Errorf("Unexpected error getting key for build %v: %v", build.Name, err)
		return ""
	}
	return key
}

func TestBasicFlows(t *testing.T) {
	tests := []struct {
		bldr                 builder.Interface
		setup                func()
		expectedErrorMessage string
	}{{
		bldr:                 &nop.Builder{},
		expectedErrorMessage: noErrorMessage,
	}, {
		bldr:                 &nop.Builder{ErrorMessage: "boom"},
		expectedErrorMessage: "boom",
	}}

	for idx, test := range tests {
		build := newBuild("test")
		f := &fixture{
			t:           t,
			objects:     []runtime.Object{build},
			kubeobjects: nil,
			client:      fake.NewSimpleClientset(build),
			kubeclient:  k8sfake.NewSimpleClientset(),
		}

		stopCh := make(chan struct{})
		eventCh := make(chan string, 1024)
		defer close(stopCh)
		defer close(eventCh)

		c, i, k8sI := f.newController(test.bldr, eventCh)
		f.updateIndex(i, []*v1alpha1.Build{build})
		i.Start(stopCh)
		k8sI.Start(stopCh)

		// Run a single iteration of the syncHandler.
		if err := c.syncHandler(getKey(build, t)); err != nil {
			t.Errorf("error syncing build: %v", err)
		}

		buildClient := f.client.BuildV1alpha1().Builds(build.Namespace)
		first, err := buildClient.Get(build.Name, metav1.GetOptions{})
		if err != nil {
			t.Errorf("error fetching build: %v", err)
		}
		// Update status to current time
		first.Status.StartTime = metav1.Now()

		if builder.IsDone(&first.Status) {
			t.Errorf("First IsDone(%d); wanted not done, got done.", idx)
		}
		if msg, failed := builder.ErrorMessage(&first.Status); failed {
			t.Errorf("First ErrorMessage(%d); wanted not failed, got %q.", idx, msg)
		}

		// We have to manually update the index, or the controller won't see the update.
		f.updateIndex(i, []*v1alpha1.Build{first})

		// Run a second iteration of the syncHandler.
		if err := c.syncHandler(getKey(build, t)); err != nil {
			t.Errorf("error syncing build: %v", err)
		}
		// A second reconciliation will trigger an asynchronous "Wait()", which
		// should immediately return and trigger an update.  Sleep to ensure that
		// is all done before further checks.
		time.Sleep(1 * time.Second)

		second, err := buildClient.Get(build.Name, metav1.GetOptions{})
		if err != nil {
			t.Errorf("error fetching build: %v", err)
		}

		if !builder.IsDone(&second.Status) {
			t.Errorf("Second IsDone(%d, %v); wanted done, got not done.", idx, second.Status)
		}
		if msg, _ := builder.ErrorMessage(&second.Status); test.expectedErrorMessage != msg {
			t.Errorf("Second ErrorMessage(%d); wanted %q, got %q.", idx, test.expectedErrorMessage, msg)
		}

		successEvent := "Normal Synced Build synced successfully"

		select {
		case statusEvent := <-eventCh:
			if statusEvent != successEvent {
				t.Errorf("Event; wanted %q, got %q", successEvent, statusEvent)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("No events published")
		}
	}
}

func TestErrFlows(t *testing.T) {
	bldr := &nop.Builder{Err: errors.New("not okay")}
	expectedErrEventMsg := "Warning BuildExecuteFailed Failed to execute Build"

	build := newBuild("test")
	f := &fixture{
		t:           t,
		objects:     []runtime.Object{build},
		kubeobjects: nil,
		client:      fake.NewSimpleClientset(build),
		kubeclient:  k8sfake.NewSimpleClientset(),
	}

	stopCh := make(chan struct{})
	eventCh := make(chan string, 1024)
	defer close(stopCh)
	defer close(eventCh)

	c, i, k8sI := f.newController(bldr, eventCh)
	f.updateIndex(i, []*v1alpha1.Build{build})
	i.Start(stopCh)
	k8sI.Start(stopCh)

	if err := c.syncHandler(getKey(build, t)); err == nil {
		t.Errorf("Expect error syncing build")
	}

	select {
	case statusEvent := <-eventCh:
		if !strings.Contains(statusEvent, expectedErrEventMsg) {
			t.Errorf("Event message; wanted %q, got %q", expectedErrEventMsg, statusEvent)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("No events published")
	}
}

func TestTimeoutFlows(t *testing.T) {
	bldr := &nop.Builder{}

	build := newBuild("test")
	buffer, err := time.ParseDuration("10m")
	if err != nil {
		t.Errorf("Error parsing duration")
	}

	build.Spec.Timeout = metav1.Duration{Duration: 1 * time.Second}

	f := &fixture{
		t:           t,
		objects:     []runtime.Object{build},
		kubeobjects: nil,
		client:      fake.NewSimpleClientset(build),
		kubeclient:  k8sfake.NewSimpleClientset(),
	}

	stopCh := make(chan struct{})
	eventCh := make(chan string, 1024)
	defer close(stopCh)
	defer close(eventCh)

	c, i, k8sI := f.newController(bldr, eventCh)

	f.updateIndex(i, []*v1alpha1.Build{build})
	i.Start(stopCh)
	k8sI.Start(stopCh)

	if err := c.syncHandler(getKey(build, t)); err != nil {
		t.Errorf("Not Expect error when syncing build")
	}

	buildClient := f.client.BuildV1alpha1().Builds(build.Namespace)
	first, err := buildClient.Get(build.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("error fetching build: %v", err)
	}

	// Update status to past time by substracting buffer time
	first.Status.CreationTime.Time = metav1.Now().Time.Add(-buffer)

	if builder.IsDone(&first.Status) {
		t.Error("First IsDone; wanted not done, got done.")
	}
	if msg, failed := builder.ErrorMessage(&first.Status); failed {
		t.Errorf("First ErrorMessage(%v); wanted not failed, got failed", msg)
	}

	// We have to manually update the index, or the controller won't see the update.
	f.updateIndex(i, []*v1alpha1.Build{first})

	// Run a second iteration of the syncHandler.
	if err := c.syncHandler(getKey(build, t)); err != nil {
		t.Errorf("Unexpected error while syncing build: %v", err)
	}

	expectedTimeoutMsg := "Warning BuildTimeout Build \"test\" failed to finish within \"1s\""
	for i := 0; i < 2; i++ {
		select {
		case statusEvent := <-eventCh:
			// Check 2nd event for timeout error msg. First event will sync build successfully
			if !strings.Contains(statusEvent, expectedTimeoutMsg) && i != 0 {
				t.Errorf("Event message; wanted %q got %q", expectedTimeoutMsg, statusEvent)
			}
		case <-time.After(4 * time.Second):
			t.Fatalf("No events published")
		}
	}
}
