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
	"fmt"
	"sync"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	buildercommon "github.com/knative/build/pkg/builder"
	"github.com/knative/build/pkg/builder/cluster/convert"
)

type operation struct {
	builder   *builder
	namespace string
	name      string
	startTime metav1.Time
	statuses  []corev1.ContainerStatus
}

func (op *operation) Name() string {
	return op.name
}

func (op *operation) Checkpoint(status *v1alpha1.BuildStatus) error {
	status.Builder = v1alpha1.ClusterBuildProvider
	if status.Cluster == nil {
		status.Cluster = &v1alpha1.ClusterSpec{}
	}
	status.Cluster.Namespace = op.namespace
	status.Cluster.PodName = op.Name()
	status.StartTime = op.startTime
	status.StepStates = nil
	for _, s := range op.statuses {
		status.StepStates = append(status.StepStates, s.State)
	}
	status.SetCondition(&v1alpha1.BuildCondition{
		Type:   v1alpha1.BuildSucceeded,
		Status: corev1.ConditionUnknown,
		Reason: "Building",
	})
	return nil
}

func (op *operation) Wait() (*v1alpha1.BuildStatus, error) {
	podCh := make(chan *corev1.Pod)
	defer close(podCh)

	// Ask the builder's watch loop to send a message on our channel when it sees our Pod complete.
	if err := op.builder.registerDoneCallback(op.namespace, op.name, podCh); err != nil {
		return nil, err
	}

	glog.Infof("Waiting for %q", op.Name())
	pod := <-podCh
	op.statuses = pod.Status.InitContainerStatuses

	states := []corev1.ContainerState{}
	for _, status := range pod.Status.InitContainerStatuses {
		states = append(states, status.State)
	}

	bs := &v1alpha1.BuildStatus{
		Builder: v1alpha1.ClusterBuildProvider,
		Cluster: &v1alpha1.ClusterSpec{
			Namespace: op.namespace,
			PodName:   op.Name(),
		},
		StartTime:      op.startTime,
		CompletionTime: metav1.Now(),
		StepStates:     states,
	}
	if pod.Status.Phase == corev1.PodFailed {
		msg := getFailureMessage(pod)
		bs.SetCondition(&v1alpha1.BuildCondition{
			Type:    v1alpha1.BuildSucceeded,
			Status:  corev1.ConditionFalse,
			Message: msg,
		})
	} else {
		bs.SetCondition(&v1alpha1.BuildCondition{
			Type:   v1alpha1.BuildSucceeded,
			Status: corev1.ConditionTrue,
		})
	}
	return bs, nil
}

type build struct {
	builder *builder
	body    *corev1.Pod
}

func (b *build) Execute() (buildercommon.Operation, error) {
	pod, err := b.builder.kubeclient.CoreV1().Pods(b.body.Namespace).Create(b.body)
	if err != nil {
		return nil, err
	}
	return &operation{
		builder:   b.builder,
		namespace: pod.Namespace,
		name:      pod.Name,
		startTime: metav1.Now(),
		statuses:  pod.Status.InitContainerStatuses,
	}, nil
}

// NewBuilder constructs an on-cluster builder.Interface for executing Build custom resources.
func NewBuilder(kubeclient kubernetes.Interface, kubeinformers kubeinformers.SharedInformerFactory) buildercommon.Interface {
	b := &builder{
		kubeclient: kubeclient,
		callbacks:  make(map[string]chan *corev1.Pod),
	}

	podInformer := kubeinformers.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    b.addPodEvent,
		UpdateFunc: b.updatePodEvent,
		DeleteFunc: b.deletePodEvent,
	})

	return b
}

type builder struct {
	kubeclient kubernetes.Interface

	// mux guards modifications to callbacks
	mux sync.Mutex
	// callbacks is keyed by Pod names and stores the channel on which to
	// send a completion notification when we see that Pod complete.
	// On success, an empty string is sent.
	// On failure, the Message of the failure PodCondition is sent.
	callbacks map[string]chan *corev1.Pod
}

func (b *builder) Builder() v1alpha1.BuildProvider {
	return v1alpha1.ClusterBuildProvider
}

func (b *builder) Validate(u *v1alpha1.Build, tmpl *v1alpha1.BuildTemplate) error {
	_, err := convert.FromCRD(u, b.kubeclient)
	return err
}

func (b *builder) BuildFromSpec(u *v1alpha1.Build) (buildercommon.Build, error) {
	bld, err := convert.FromCRD(u, b.kubeclient)
	if err != nil {
		return nil, err
	}
	return &build{
		builder: b,
		body:    bld,
	}, nil
}

func (b *builder) OperationFromStatus(status *v1alpha1.BuildStatus) (buildercommon.Operation, error) {
	if status.Builder != v1alpha1.ClusterBuildProvider {
		return nil, fmt.Errorf("not a 'Cluster' builder: %v", status.Builder)
	}
	if status.Cluster == nil {
		return nil, fmt.Errorf("status.cluster cannot be empty: %v", status)
	}
	var statuses []corev1.ContainerStatus
	for _, state := range status.StepStates {
		statuses = append(statuses, corev1.ContainerStatus{State: state})
	}
	return &operation{
		builder:   b,
		namespace: status.Cluster.Namespace,
		name:      status.Cluster.PodName,
		startTime: status.StartTime,
		statuses:  statuses,
	}, nil
}

func getKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// registerDoneCallback directs the builders to send a completion notification on podCh
// when the named Pod completes.  An empty message is sent on successful completion.
func (b *builder) registerDoneCallback(namespace, name string, podCh chan *corev1.Pod) error {
	b.mux.Lock()
	defer b.mux.Unlock()
	k := getKey(namespace, name)
	if _, ok := b.callbacks[k]; ok {
		return fmt.Errorf("another process is already waiting on %q", k)
	}
	b.callbacks[k] = podCh
	return nil
}

// addPodEvent handles the informer's AddFunc event for Pods.
func (b *builder) addPodEvent(obj interface{}) {
	pod := obj.(*corev1.Pod)
	ownerRef := metav1.GetControllerOf(pod)

	// If this object is not owned by a Build, we should not do anything more with it.
	if ownerRef == nil || ownerRef.Kind != "Build" {
		return
	}

	// We only take action on pods that have completed, in some way.
	if !isDone(pod) {
		return
	}

	// Once we have a complete Pod to act on, take the lock and see if anyone's watching.
	b.mux.Lock()
	defer b.mux.Unlock()
	key := getKey(pod.Namespace, pod.Name)
	if ch, ok := b.callbacks[key]; ok {
		// Send the person listening the message and remove this callback from our map.
		ch <- pod
		delete(b.callbacks, key)
	} else {
		glog.Errorf("Saw %q complete, but nothing was watching for it!", key)
	}
}

// updatePodEvent handles the informer's UpdateFunc event for Pods.
func (b *builder) updatePodEvent(old, new interface{}) {
	// Same as addPodEvent(new)
	b.addPodEvent(new)
}

// deletePodEvent handles the informer's DeleteFunc event for Pods.
func (b *builder) deletePodEvent(obj interface{}) {
	// TODO(mattmoor): If a pod gets deleted and someone's watching, we should propagate our
	// own error message so that we don't leak a go routine waiting forever.
	glog.Errorf("NYI: delete event for: %v", obj)
}

func isDone(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed
}

func getFailureMessage(pod *corev1.Pod) string {
	// First, try to surface an error about the actual build step that failed.
	for _, status := range pod.Status.InitContainerStatuses {
		term := status.State.Terminated
		if term != nil && term.ExitCode != 0 {
			return fmt.Sprintf("build step %q exited with code %d (image: %q); for logs run: kubectl -n %s logs %s -c %s",
				status.Name, term.ExitCode, status.ImageID,
				pod.Namespace, pod.Name, status.Name)
		}
	}
	// Next, return the Pod's status message if it has one.
	if pod.Status.Message != "" {
		return pod.Status.Message
	}
	// Lastly fall back on a generic error message.
	return "build failed for unspecified reasons."
}
