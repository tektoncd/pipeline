// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pods

import (
	"testing"
	"time"

	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	k8stest "k8s.io/client-go/testing"
)

func Test_wait_pod_initialized(t *testing.T) {
	podname := "test"
	ns := "ns"

	initial := tb.Pod(podname, ns,
		cb.PodStatus(
			cb.PodPhase(corev1.PodPending),
		),
	)
	later := tb.Pod(podname, ns,
		cb.PodStatus(
			cb.PodPhase(corev1.PodRunning),
		),
	)
	kc := simulateAddWatch(t, initial, later)

	pod := NewWithDefaults(podname, ns, kc)
	p, err := pod.Wait()

	if p == nil {
		t.Errorf("Unexpected p mismatch: \n%s\n", p)
	}

	if err != nil {
		t.Errorf("Unexpected error: \n%s\n", err)
	}
}

func Test_wait_pod_success(t *testing.T) {
	podname := "test"
	ns := "ns"

	initial := tb.Pod(podname, ns,
		cb.PodStatus(
			cb.PodPhase(corev1.PodPending),
		),
	)
	later := tb.Pod(podname, ns,
		cb.PodStatus(
			cb.PodPhase(corev1.PodSucceeded),
		),
	)
	kc := simulateAddWatch(t, initial, later)

	pod := NewWithDefaults(podname, ns, kc)
	p, err := pod.Wait()

	if p == nil {
		t.Errorf("Unexpected output mismatch: \n%s\n", p)
	}

	if err != nil {
		t.Errorf("Unexpected error: \n%s\n", err)
	}
}

func Test_wait_pod_fail(t *testing.T) {
	podname := "test"
	ns := "ns"

	initial := tb.Pod(podname, ns,
		cb.PodStatus(
			cb.PodPhase(corev1.PodPending),
		),
	)
	later := tb.Pod(podname, ns,
		cb.PodStatus(
			cb.PodPhase(corev1.PodFailed),
			cb.PodCondition(corev1.PodInitialized, corev1.ConditionTrue),
		),
	)
	kc := simulateAddWatch(t, initial, later)

	pod := NewWithDefaults(podname, ns, kc)
	p, err := pod.Wait()

	if p == nil {
		t.Errorf("Unexpected output mismatch: \n%s\n", p)
	}

	if err != nil {
		t.Errorf("Unexpected error: \n%s\n", err)
	}
}

func Test_wait_pod_imagepull_error(t *testing.T) {
	podname := "test"
	ns := "ns"

	initial := tb.Pod(podname, ns,
		cb.PodStatus(
			cb.PodPhase(corev1.PodPending),
		),
	)

	deletionTime := metav1.Now()
	later := tb.Pod(podname, ns,
		cb.PodDeletionTime(&deletionTime),
		cb.PodStatus(
			cb.PodPhase(corev1.PodFailed),
		),
	)

	kc := simulateDeleteWatch(t, initial, later)
	pod := NewWithDefaults(podname, ns, kc)
	p, err := pod.Wait()

	if p == nil {
		t.Errorf("Unexpected output mismatch: \n%s\n", p)
	}

	if err == nil {
		t.Errorf("Unexpected error type %v", err)
	}
}

func simulateAddWatch(t *testing.T, initial *corev1.Pod, later *corev1.Pod) k8s.Interface {
	ps := []*corev1.Pod{
		initial,
	}

	clients, _ := test.SeedTestData(t, pipelinetest.Data{Pods: ps})
	watcher := watch.NewFake()
	clients.Kube.PrependWatchReactor("pods", k8stest.DefaultWatchReactor(watcher, nil))

	go func() {
		time.Sleep(2 * time.Second)
		watcher.Add(later)
	}()

	return clients.Kube
}

func simulateDeleteWatch(t *testing.T, initial *corev1.Pod, later *corev1.Pod) k8s.Interface {
	ps := []*corev1.Pod{
		initial,
	}

	clients, _ := test.SeedTestData(t, pipelinetest.Data{Pods: ps})
	watcher := watch.NewFake()
	clients.Kube.PrependWatchReactor("pods", k8stest.DefaultWatchReactor(watcher, nil))

	go func() {
		time.Sleep(2 * time.Second)
		watcher.Delete(later)
	}()

	return clients.Kube
}
