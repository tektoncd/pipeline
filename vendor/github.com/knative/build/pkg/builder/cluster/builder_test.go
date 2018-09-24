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

package cluster

import (
	"strings"
	"time"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/pkg/apis"
	"go.uber.org/zap"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	fakek8s "k8s.io/client-go/kubernetes/fake"

	buildercommon "github.com/knative/build/pkg/builder"
	"github.com/knative/build/pkg/buildtest"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"

	"testing"
)

const (
	namespace            = ""
	expectedErrorMessage = "stuff broke"
	expectedErrorReason  = "it was bad"
	expectedPendingMsg   = "build step \"\" is pending with reason \"stuff broke\""
)

var ignoreVolatileTime = cmp.Comparer(func(_, _ apis.VolatileTime) bool {
	return true
})

func newBuilder(cs kubernetes.Interface) *builder {
	kif := kubeinformers.NewSharedInformerFactory(cs, time.Second*30)
	logger := zap.NewExample().Sugar()
	return NewBuilder(cs, kif, logger).(*builder)
}

func TestBasicFlow(t *testing.T) {
	cs := fakek8s.NewSimpleClientset(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	builder := newBuilder(cs)
	b, err := builder.BuildFromSpec(&v1alpha1.Build{})
	if err != nil {
		t.Fatalf("Unexpected error creating builder.Build from Spec: %v", err)
	}
	op, err := b.Execute()
	if err != nil {
		t.Fatalf("Unexpected error executing builder.Build: %v", err)
	}

	build := &v1alpha1.Build{
		Status: v1alpha1.BuildStatus{},
	}
	if err := op.Checkpoint(build, &build.Status); err != nil {
		t.Fatalf("Unexpected error executing op.Checkpoint: %v", err)
	}
	if buildercommon.IsDone(&build.Status) {
		t.Errorf("IsDone(%v); wanted not done, got done.", build.Status)
	}
	if build.Status.CreationTime.IsZero() {
		t.Errorf("build.Status.CreationTime; want zero, got %v", build.Status.CreationTime)
	}
	if !build.Status.CompletionTime.IsZero() {
		t.Errorf("build.Status.CompletionTime; want zero, got %v", build.Status.CompletionTime)
	}
	if !build.Status.StartTime.IsZero() {
		t.Errorf("build.Status.StartTime; want non-zero, got %v", build.Status.StartTime)
	}
	op, err = builder.OperationFromStatus(&build.Status)
	if err != nil {
		t.Fatalf("Unexpected error executing OperationFromStatus: %v", err)
	}

	checksComplete := buildtest.NewWait()
	readyForUpdate := buildtest.NewWait()
	go func() {
		// Wait sufficiently long for Wait() to have been called and then
		// signal to the main test thread that it should perform the update.
		readyForUpdate.In(1 * time.Second)

		defer checksComplete.Done()
		status, err := op.Wait()
		if err != nil {
			t.Fatalf("Unexpected error waiting for builder.Operation: %v", err)
		}

		// Check that status came out how we expect.
		if !buildercommon.IsDone(status) {
			t.Errorf("IsDone(%v); wanted true, got false", status)
		}
		if status.Cluster.PodName != op.Name() {
			t.Errorf("status.Cluster.PodName; wanted %q, got %q", op.Name(), status.Cluster.PodName)
		}
		if msg, failed := buildercommon.ErrorMessage(status); failed {
			t.Errorf("ErrorMessage(%v); wanted not failed, got %q", status, msg)
		}
		if status.CompletionTime.IsZero() {
			t.Errorf("status.CompletionTime; want non-zero, got %v", status.CompletionTime)
		}
		if status.StartTime.IsZero() {
			t.Errorf("status.StartTime; want non-zero, got %v", status.StartTime)
		}
		if status.CreationTime.IsZero() {
			t.Errorf("status.CreationTime; want non-zero, got %v", status.CreationTime)
		}
	}()
	// Wait until the test thread is ready for us to update things.
	readyForUpdate.Wait()

	// We should be able to fetch the Pod that b.Execute() created in our fake client.
	podsclient := cs.CoreV1().Pods(namespace)
	pod, err := podsclient.Get(op.Name(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching Pod: %v", err)
	}
	pod.Status.StartTime = &metav1.Time{Time: time.Now()}

	// Now modify it to look done.
	pod.Status.Phase = corev1.PodSucceeded
	pod, err = podsclient.Update(pod)
	if err != nil {
		t.Fatalf("Unexpected error updating Pod: %v", err)
	}

	// The informer doesn't seem to properly pick up this update via the fake,
	// so trigger the update event manually.
	builder.updatePodEvent(nil, pod)

	checksComplete.WaitUntil(5*time.Second, buildtest.WaitNop, func() {
		t.Fatal("timed out in op.Wait()")
	})

	// Trigger termination of pod
	err = op.Terminate()
	if err != nil {
		t.Errorf("Expected no error while terminating operation")
	}
	// Verify pod is not available
	if _, err = podsclient.Get(op.Name(), metav1.GetOptions{}); err == nil {
		t.Fatalf("Expected 'not found' error while fetching Pod")
	}
}

func TestNonFinalUpdateFlow(t *testing.T) {
	cs := fakek8s.NewSimpleClientset(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	builder := newBuilder(cs)
	b, err := builder.BuildFromSpec(&v1alpha1.Build{})
	if err != nil {
		t.Fatalf("Unexpected error creating builder.Build from Spec: %v", err)
	}
	op, err := b.Execute()
	if err != nil {
		t.Fatalf("Unexpected error executing builder.Build: %v", err)
	}

	build := &v1alpha1.Build{
		Status: v1alpha1.BuildStatus{},
	}
	if err := op.Checkpoint(build, &build.Status); err != nil {
		t.Fatalf("Unexpected error executing op.Checkpoint: %v", err)
	}
	if buildercommon.IsDone(&build.Status) {
		t.Errorf("IsDone(%v); wanted not done, got done.", build.Status)
	}
	if build.Status.CreationTime.IsZero() {
		t.Errorf("build.Status.CreationTime; want zero, got %v", build.Status.CreationTime)
	}
	if !build.Status.CompletionTime.IsZero() {
		t.Errorf("build.Status.CompletionTime; want zero, got %v", build.Status.CompletionTime)
	}
	if !build.Status.StartTime.IsZero() {
		t.Errorf("build.Status.StartTime; want non-zero, got %v", build.Status.StartTime)
	}
	op, err = builder.OperationFromStatus(&build.Status)
	if err != nil {
		t.Fatalf("Unexpected error executing OperationFromStatus: %v", err)
	}

	checksComplete := buildtest.NewWait()
	readyForUpdate := buildtest.NewWait()
	go func() {
		// Wait sufficiently long for Wait() to have been called and then
		// signal to the main test thread that it should perform the update.
		readyForUpdate.In(1 * time.Second)

		defer checksComplete.Done()
		status, err := op.Wait()
		if err != nil {
			t.Fatalf("Unexpected error waiting for builder.Operation: %v", err)
		}
		if status.CreationTime.IsZero() {
			t.Errorf("status.CreationTime; want non-zero, got %v", status.CreationTime)
		}
		if status.CompletionTime.IsZero() {
			t.Errorf("status.CompletionTime; want non-zero, got %v", status.CompletionTime)
		}
	}()
	// Wait until the test thread is ready for us to update things.
	readyForUpdate.Wait()

	// We should be able to fetch the Pod that b.Execute() created in our fake client.
	podsclient := cs.CoreV1().Pods(namespace)
	pod, err := podsclient.Get(op.Name(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching Pod: %v", err)
	}
	// Make a non-terminal modification
	pod.Status.Phase = corev1.PodRunning
	pod.Status.StartTime = &metav1.Time{Time: time.Now()}

	pod, err = podsclient.Update(pod)
	if err != nil {
		t.Fatalf("Unexpected error updating Pod: %v", err)
	}

	// The informer doesn't seem to properly pick up this update via the fake,
	// so trigger the update event manually.
	builder.updatePodEvent(nil, pod)

	// If we do not get a message from our Wait(), then we ignored the
	// benign update.  If we still haven't heard anything after 5 seconds, then
	// throw an error.
	checksComplete.WaitUntil(5*time.Second, buildtest.WaitNop, func() {
		t.Fatal("timed out in op.Wait()")
	})

	// Now make it look done.
	pod.Status.Phase = corev1.PodSucceeded
	pod, err = podsclient.Update(pod)
	if err != nil {
		t.Fatalf("Unexpected error updating Pod: %v", err)
	}

	// The informer doesn't seem to properly pick up this update via the fake,
	// so trigger the update event manually.
	builder.updatePodEvent(nil, pod)

	checksComplete.WaitUntil(5*time.Second, buildtest.WaitNop, func() {
		t.Fatal("timed out in op.Wait()")
	})
}

func TestFailureFlow(t *testing.T) {
	cs := fakek8s.NewSimpleClientset(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	builder := newBuilder(cs)
	b, err := builder.BuildFromSpec(&v1alpha1.Build{})
	if err != nil {
		t.Fatalf("Unexpected error creating builder.Build from Spec: %v", err)
	}
	op, err := b.Execute()
	if err != nil {
		t.Fatalf("Unexpected error executing builder.Build: %v", err)
	}

	build := &v1alpha1.Build{
		Status: v1alpha1.BuildStatus{},
	}
	if err := op.Checkpoint(build, &build.Status); err != nil {
		t.Fatalf("Unexpected error executing op.Checkpoint: %v", err)
	}
	if buildercommon.IsDone(&build.Status) {
		t.Errorf("IsDone(%v); wanted not done, got done.", build.Status)
	}
	if build.Status.CreationTime.IsZero() {
		t.Errorf("build.Status.CreationTime; want zero, got %v", build.Status.CreationTime)
	}
	if !build.Status.CompletionTime.IsZero() {
		t.Errorf("build.Status.CompletionTime; want zero, got %v", build.Status.CompletionTime)
	}
	if !build.Status.StartTime.IsZero() {
		t.Errorf("build.Status.StartTime; want non-zero, got %v", build.Status.StartTime)
	}
	op, err = builder.OperationFromStatus(&build.Status)
	if err != nil {
		t.Fatalf("Unexpected error executing OperationFromStatus: %v", err)
	}

	checksComplete := buildtest.NewWait()
	readyForUpdate := buildtest.NewWait()
	go func() {
		// Wait sufficiently long for Wait() to have been called and then
		// signal to the main test thread that it should perform the update.
		readyForUpdate.In(1 * time.Second)

		defer checksComplete.Done()
		status, err := op.Wait()
		if err != nil {
			t.Fatalf("Unexpected error waiting for builder.Operation: %v", err)
		}

		// Check that status came out how we expect.
		if !buildercommon.IsDone(status) {
			t.Errorf("IsDone(%v); wanted true, got false", status)
		}
		if status.Cluster.PodName != op.Name() {
			t.Errorf("status.Cluster.PodName; wanted %q, got %q", op.Name(), status.Cluster.PodName)
		}
		if msg, failed := buildercommon.ErrorMessage(status); !failed || msg != expectedErrorMessage {
			t.Errorf("ErrorMessage(%v); wanted %q, got %q", status, expectedErrorMessage, msg)
		}
		if status.CreationTime.IsZero() {
			t.Errorf("status.CreationTime; want non-zero, got %v", status.CreationTime)
		}
		if status.CompletionTime.IsZero() {
			t.Errorf("status.CompletionTime; want non-zero, got %v", status.CompletionTime)
		}
		if status.StartTime.IsZero() {
			t.Errorf("status.StartTime; want non-zero, got %v", status.StartTime)
		}
		if len(status.StepStates) != 1 {
			t.Errorf("StepStates contained %d states, want 1: %+v", len(status.StepStates), status.StepStates)
		} else if status.StepStates[0].Terminated.Reason != expectedErrorReason {
			t.Errorf("StepStates[0] reason got %q, want %q", status.StepStates[0].Terminated.Reason, expectedErrorReason)
		}
	}()
	// Wait until the test thread is ready for us to update things.
	readyForUpdate.Wait()

	// We should be able to fetch the Pod that b.Execute() created in our fake client.
	podsclient := cs.CoreV1().Pods(namespace)
	pod, err := podsclient.Get(op.Name(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching Pod: %v", err)
	}
	// Now modify it to look done.
	pod.Status.Phase = corev1.PodFailed
	pod.Status.Message = expectedErrorMessage
	pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		State: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				Reason: expectedErrorReason,
			},
		},
	}}
	pod.Status.StartTime = &metav1.Time{Time: time.Now()}
	pod, err = podsclient.Update(pod)
	if err != nil {
		t.Fatalf("Unexpected error updating Pod: %v", err)
	}

	// The informer doesn't seem to properly pick up this update via the fake,
	// so trigger the update event manually.
	builder.updatePodEvent(nil, pod)

	checksComplete.WaitUntil(5*time.Second, buildtest.WaitNop, func() {
		t.Fatal("timed out in op.Wait()")
	})
}

func TestPodPendingFlow(t *testing.T) {
	cs := fakek8s.NewSimpleClientset(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	builder := newBuilder(cs)
	b, err := builder.BuildFromSpec(&v1alpha1.Build{})
	if err != nil {
		t.Fatalf("Unexpected error creating builder.Build from Spec: %v", err)
	}
	op, err := b.Execute()
	if err != nil {
		t.Fatalf("Unexpected error executing builder.Build: %v", err)
	}

	build := &v1alpha1.Build{
		Status: v1alpha1.BuildStatus{},
	}
	if err := op.Checkpoint(build, &build.Status); err != nil {
		t.Fatalf("Unexpected error executing op.Checkpoint: %v", err)
	}
	if buildercommon.IsDone(&build.Status) {
		t.Errorf("IsDone(%v); wanted not done, got done.", build.Status)
	}
	if build.Status.CreationTime.IsZero() {
		t.Errorf("build.Status.CreationTime; want zero, got %v", build.Status.CreationTime)
	}
	if !build.Status.CompletionTime.IsZero() {
		t.Errorf("build.Status.CompletionTime; want zero, got %v", build.Status.CompletionTime)
	}
	if !build.Status.StartTime.IsZero() {
		t.Errorf("build.Status.StartTime; want non-zero, got %v", build.Status.StartTime)
	}
	op, err = builder.OperationFromStatus(&build.Status)
	if err != nil {
		t.Fatalf("Unexpected error executing OperationFromStatus: %v", err)
	}

	checksComplete := buildtest.NewWait()
	readyForUpdate := buildtest.NewWait()
	go func() {
		// Wait sufficiently long for Wait() to have been called and then
		// signal to the main test thread that it should perform the update.
		readyForUpdate.In(1 * time.Second)

		defer checksComplete.Done()
		status, err := op.Wait()
		if err != nil {
			t.Fatalf("Unexpected error waiting for builder.Operation: %v", err)
		}

		// Check that status came out how we expect.
		if buildercommon.IsDone(status) {
			t.Errorf("IsDone(%v); wanted false, got true", status)
		}
		if status.Cluster.PodName != op.Name() {
			t.Errorf("status.Cluster.PodName; wanted %q, got %q", op.Name(), status.Cluster.PodName)
		}
		if msg := statusMessage(status); msg != expectedPendingMsg {
			t.Errorf("ErrorMessage(%v); wanted %q, got %q", status, expectedPendingMsg, msg)
		}
		if status.CreationTime.IsZero() {
			t.Errorf("status.CreationTime; want non-zero, got %v", status.CreationTime)
		}
		if status.StartTime.IsZero() {
			t.Errorf("status.StartTime; want non-zero, got %v", status.StartTime)
		}
		if status.CompletionTime.IsZero() {
			t.Errorf("status.CompletionTime; want non-zero, got %v", status.CompletionTime)
		}
		if len(status.StepStates) != 1 {
			t.Errorf("StepStates contained %d states, want 1: %+v", len(status.StepStates), status.StepStates)
		} else if status.StepStates[0].Waiting.Reason != expectedErrorReason {
			t.Errorf("StepStates[0] reason got %q, want %q", status.StepStates[0].Waiting.Reason, expectedErrorReason)
		}
	}()
	// Wait until the test thread is ready for us to update things.
	readyForUpdate.Wait()

	// We should be able to fetch the Pod that b.Execute() created in our fake client.
	podsclient := cs.CoreV1().Pods(namespace)
	pod, err := podsclient.Get(op.Name(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching Pod: %v", err)
	}

	pod.Status.Phase = corev1.PodPending
	pod.Status.Message = expectedErrorMessage
	pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Message: expectedErrorMessage,
				Reason:  expectedErrorReason,
			},
		},
	}}
	pod.Status.StartTime = &metav1.Time{Time: time.Now()}
	pod, err = podsclient.Update(pod)
	if err != nil {
		t.Fatalf("Unexpected error updating Pod: %v", err)
	}

	// The informer doesn't seem to properly pick up this update via the fake,
	// so trigger the update event manually.
	builder.updatePodEvent(nil, pod)

	checksComplete.WaitUntil(5*time.Second, buildtest.WaitNop, func() {
		t.Fatal("timed out in op.Wait()")
	})
}

func TestStepFailureFlow(t *testing.T) {
	cs := fakek8s.NewSimpleClientset(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	builder := newBuilder(cs)
	b, err := builder.BuildFromSpec(&v1alpha1.Build{
		Spec: v1alpha1.BuildSpec{
			Steps: []corev1.Container{{
				Name:    "step-name",
				Image:   "ubuntu:latest",
				Command: []string{"false"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Unexpected error creating builder.Build from Spec: %v", err)
	}
	op, err := b.Execute()
	if err != nil {
		t.Fatalf("Unexpected error executing builder.Build: %v", err)
	}

	build := &v1alpha1.Build{
		Status: v1alpha1.BuildStatus{},
	}
	if err := op.Checkpoint(build, &build.Status); err != nil {
		t.Fatalf("Unexpected error executing op.Checkpoint: %v", err)
	}
	if buildercommon.IsDone(&build.Status) {
		t.Errorf("IsDone(%v); wanted not done, got done.", build.Status)
	}
	if build.Status.CreationTime.IsZero() {
		t.Errorf("build.Status.CreationTime; want zero, got %v", build.Status.CreationTime)
	}
	if !build.Status.StartTime.IsZero() {
		t.Errorf("build.Status.StartTime; want non-zero, got %v", build.Status.StartTime)
	}
	if !build.Status.CompletionTime.IsZero() {
		t.Errorf("build.Status.CompletionTime; want zero, got %v", build.Status.CompletionTime)
	}
	op, err = builder.OperationFromStatus(&build.Status)
	if err != nil {
		t.Fatalf("Unexpected error executing OperationFromStatus: %v", err)
	}

	checksComplete := buildtest.NewWait()
	readyForUpdate := buildtest.NewWait()
	go func() {
		// Wait sufficiently long for Wait() to have been called and then
		// signal to the main test thread that it should perform the update.
		readyForUpdate.In(1 * time.Second)

		defer checksComplete.Done()
		status, err := op.Wait()
		if err != nil {
			t.Fatalf("Unexpected error waiting for builder.Operation: %v", err)
		}

		// Check that status came out how we expect.
		if !buildercommon.IsDone(status) {
			t.Errorf("IsDone(%v); got false, want true", status)
		}
		if status.Cluster.PodName != op.Name() {
			t.Errorf("status.Cluster.PodName; got %q, want %q", status.Cluster.PodName, op.Name())
		}
		if msg, failed := buildercommon.ErrorMessage(status); !failed ||
			// We expect the error to contain the step name and exit code.
			!strings.Contains(msg, `"step-name"`) || !strings.Contains(msg, "128") {
			t.Errorf("ErrorMessage(%v); got %q, want %q", status, msg, expectedErrorMessage)
		}
		if status.CreationTime.IsZero() {
			t.Errorf("status.CreationTime; got %v, want non-zero", status.CreationTime)
		}
		if status.StartTime.IsZero() {
			t.Errorf("status.StartTime; got %v, want non-zero", status.StartTime)
		}
		if status.CompletionTime.IsZero() {
			t.Errorf("status.CompletionTime; got %v, want non-zero", status.CompletionTime)
		}
	}()
	// Wait until the test thread is ready for us to update things.
	readyForUpdate.Wait()

	// We should be able to fetch the Pod that b.Execute() created in our fake client.
	podsclient := cs.CoreV1().Pods(namespace)
	pod, err := podsclient.Get(op.Name(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching Pod: %v", err)
	}
	// Now modify it to look done.
	pod.Status.Phase = corev1.PodFailed
	pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		Name: "step-name",
		State: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 128,
			},
		},
		ImageID: "docker-pullable://ubuntu@sha256:deadbeef",
	}}
	pod.Status.Message = "don't expect this!"
	pod.Status.StartTime = &metav1.Time{Time: time.Now()}

	pod, err = podsclient.Update(pod)
	if err != nil {
		t.Fatalf("Unexpected error updating Pod: %v", err)
	}

	// The informer doesn't seem to properly pick up this update via the fake,
	// so trigger the update event manually.
	builder.updatePodEvent(nil, pod)

	checksComplete.WaitUntil(5*time.Second, buildtest.WaitNop, func() {
		t.Fatal("timed out in op.Wait()")
	})
}

func TestBasicFlowWithCredentials(t *testing.T) {
	name := "my-secret-identity"
	cs := fakek8s.NewSimpleClientset(
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
			Secrets: []corev1.ObjectReference{{
				Name: name,
			}, {
				Name: "not-annotated",
			}},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					"build.knative.dev/docker-0": "https://gcr.io",
				},
			},
			Type: corev1.SecretTypeBasicAuth,
			Data: map[string][]byte{
				corev1.BasicAuthUsernameKey: []byte("user1"),
				corev1.BasicAuthPasswordKey: []byte("password"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "not-annotated",
			},
			Type: corev1.SecretTypeBasicAuth,
			Data: map[string][]byte{
				corev1.BasicAuthUsernameKey: []byte("user2"),
				corev1.BasicAuthPasswordKey: []byte("password"),
			},
		})
	builder := newBuilder(cs)
	b, err := builder.BuildFromSpec(&v1alpha1.Build{})
	if err != nil {
		t.Fatalf("Unexpected error creating builder.Build from Spec: %v", err)
	}
	op, err := b.Execute()
	if err != nil {
		t.Fatalf("Unexpected error executing builder.Build: %v", err)
	}

	// We should be able to fetch the Pod that b.Execute() created in our fake client.
	podsclient := cs.CoreV1().Pods(namespace)
	pod, err := podsclient.Get(op.Name(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error fetching Pod: %v", err)
	}

	credInit := pod.Spec.InitContainers[0]
	if got, want := len(credInit.Args), 1; got != want {
		t.Errorf("len(CredInit.Args); got %v, want %v", got, want)
	}
	if !strings.Contains(credInit.Args[0], name) {
		t.Errorf("arg[0]; got: %v, wanted string containing %q", credInit.Args[0], name)
	}
}

func statusMessage(status *v1alpha1.BuildStatus) string {
	for _, cond := range status.Conditions {
		if cond.Type == v1alpha1.BuildSucceeded && cond.Status == corev1.ConditionUnknown {
			return cond.Reason
		}
	}
	return ""
}

func TestStripStepStates(t *testing.T) {
	for _, c := range []struct {
		desc     string
		statuses []corev1.ContainerStatus
		build    *v1alpha1.Build
	}{{
		desc: "only creds-init",
		statuses: []corev1.ContainerStatus{{
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "creds-init: should be stripped"}},
		}, {
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "real step: should be retained"}},
		}},
		build: &v1alpha1.Build{
			// No source.
			Status: v1alpha1.BuildStatus{},
		},
	}, {
		desc: "has source",
		statuses: []corev1.ContainerStatus{{
			Name:  "creds-init",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "creds-init: should be stripped"}},
		}, {
			Name:  "git-init",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "git-init: should be stripped"}},
		}, {
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "real step: should be retained"}},
		}},
		build: &v1alpha1.Build{
			// No source.
			Spec: v1alpha1.BuildSpec{
				Source: &v1alpha1.SourceSpec{
					Git: &v1alpha1.GitSourceSpec{},
				},
			},
			Status: v1alpha1.BuildStatus{},
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			status := &v1alpha1.BuildStatus{}
			op := &operation{
				statuses:  c.statuses,
				builder:   &builder{},
				namespace: "namespace",
				name:      "podname",
			}
			if err := op.Checkpoint(c.build, status); err != nil {
				t.Fatalf("Checkpoint: %v", err)
			}

			// In both cases, we want the same status, stripped of implicit
			// steps' states.
			wantStatus := &v1alpha1.BuildStatus{
				Builder: v1alpha1.ClusterBuildProvider,
				Cluster: &v1alpha1.ClusterSpec{
					Namespace: "namespace",
					PodName:   "podname",
				},
				StepStates: []corev1.ContainerState{{
					Terminated: &corev1.ContainerStateTerminated{Reason: "real step: should be retained"},
				}},
				StepsCompleted: []string{""},
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.BuildSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: "Building",
				}},
			}
			if d := cmp.Diff(wantStatus, status, ignoreVolatileTime); d != "" {
				t.Errorf("Diff:\n%s", d)
			}
		})
	}
}
