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

package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/remote/oci"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// This error is defined in etcd at
// https://github.com/etcd-io/etcd/blob/5b226e0abf4100253c94bb71f47d6815877ed5a2/server/etcdserver/errors.go#L30
// TODO: If/when https://github.com/kubernetes/kubernetes/issues/106491 is addressed,
// we should stop relying on a hardcoded string.
var errEtcdLeaderChange = "etcdserver: leader changed"

// GetTaskKind returns the referenced Task kind (Task, ClusterTask, ...) if the TaskRun is using TaskRef.
func GetTaskKind(taskrun *v1beta1.TaskRun) v1beta1.TaskKind {
	kind := v1alpha1.NamespacedTaskKind
	if taskrun.Spec.TaskRef != nil && taskrun.Spec.TaskRef.Kind != "" {
		kind = taskrun.Spec.TaskRef.Kind
	}
	return kind
}

// GetTaskFuncFromTaskRun is a factory function that will use the given TaskRef as context to return a valid GetTask function. It
// also requires a kubeclient, tektonclient, namespace, and service account in case it needs to find that task in
// cluster or authorize against an external repositroy. It will figure out whether it needs to look in the cluster or in
// a remote image to fetch the  reference. It will also return the "kind" of the task being referenced.
func GetTaskFuncFromTaskRun(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, taskrun *v1beta1.TaskRun) (GetTask, error) {
	// if the spec is already in the status, do not try to fetch it again, just use it as source of truth
	if taskrun.Status.TaskSpec != nil {
		return func(_ context.Context, name string) (v1beta1.TaskObject, error) {
			return &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: taskrun.Namespace,
				},
				Spec: *taskrun.Status.TaskSpec,
			}, nil
		}, nil
	}
	return GetTaskFunc(ctx, k8s, tekton, taskrun.Spec.TaskRef, taskrun.Namespace, taskrun.Spec.ServiceAccountName)
}

// GetTaskFunc is a factory function that will use the given TaskRef as context to return a valid GetTask function. It
// also requires a kubeclient, tektonclient, namespace, and service account in case it needs to find that task in
// cluster or authorize against an external repositroy. It will figure out whether it needs to look in the cluster or in
// a remote image to fetch the  reference. It will also return the "kind" of the task being referenced.
func GetTaskFunc(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, tr *v1beta1.TaskRef, namespace, saName string) (GetTask, error) {
	cfg := config.FromContextOrDefaults(ctx)
	kind := v1alpha1.NamespacedTaskKind
	if tr != nil && tr.Kind != "" {
		kind = tr.Kind
	}

	switch {
	case cfg.FeatureFlags.EnableTektonOCIBundles && tr != nil && tr.Bundle != "":
		// Return an inline function that implements GetTask by calling Resolver.Get with the specified task type and
		// casting it to a TaskObject.
		return func(ctx context.Context, name string) (v1beta1.TaskObject, error) {
			// If there is a bundle url at all, construct an OCI resolver to fetch the task.
			kc, err := k8schain.New(ctx, k8s, k8schain.Options{
				Namespace:          namespace,
				ServiceAccountName: saName,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get keychain: %w", err)
			}
			resolver := oci.NewResolver(tr.Bundle, kc)

			// Because the resolver will only return references with the same kind (eg ClusterTask), this will ensure we
			// don't accidentally return a Task with the same name but different kind.
			obj, err := resolver.Get(strings.TrimSuffix(strings.ToLower(string(kind)), "s"), name)
			if err != nil {
				return nil, err
			}

			// If the resolved object is already a v1beta1.{Cluster}Task, it should be returnable as a
			// v1beta1.TaskObject.
			if ti, ok := obj.(v1beta1.TaskObject); ok {
				ti.SetDefaults(ctx)
				return ti, nil
			}

			// If this object is not already a v1beta1 object, figure out what type it is actually and try to coerce it
			// into a v1beta1.TaskInterface compatible object.
			switch tt := obj.(type) {
			case *v1alpha1.Task:
				betaTask := &v1beta1.Task{}
				err := tt.ConvertTo(ctx, betaTask)
				betaTask.SetDefaults(ctx)
				return betaTask, err
			case *v1alpha1.ClusterTask:
				betaTask := &v1beta1.ClusterTask{}
				err := tt.ConvertTo(ctx, betaTask)
				betaTask.SetDefaults(ctx)
				return betaTask, err
			}

			return nil, fmt.Errorf("failed to convert obj %s into Task", obj.GetObjectKind().GroupVersionKind().String())
		}, nil
	default:
		// Even if there is no task ref, we should try to return a local resolver.
		local := &LocalTaskRefResolver{
			Namespace:    namespace,
			Kind:         kind,
			Tektonclient: tekton,
		}
		return local.GetTask, nil
	}
}

// LocalTaskRefResolver uses the current cluster to resolve a task reference.
type LocalTaskRefResolver struct {
	Namespace    string
	Kind         v1beta1.TaskKind
	Tektonclient clientset.Interface
}

// GetTask will resolve either a Task or ClusterTask from the local cluster using a versioned Tekton client. It will
// return an error if it can't find an appropriate Task for any reason.
func (l *LocalTaskRefResolver) GetTask(ctx context.Context, name string) (v1beta1.TaskObject, error) {
	if l.Kind == v1beta1.ClusterTaskKind {
		task, err := l.Tektonclient.TektonV1beta1().ClusterTasks().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return task, nil
	}

	// If we are going to resolve this reference locally, we need a namespace scope.
	if l.Namespace == "" {
		return nil, fmt.Errorf("must specify namespace to resolve reference to task %s", name)
	}
	return l.Tektonclient.TektonV1beta1().Tasks(l.Namespace).Get(ctx, name, metav1.GetOptions{})
}

// IsGetTaskErrTransient returns true if an error returned by GetTask is retryable.
func IsGetTaskErrTransient(err error) bool {
	return strings.Contains(err.Error(), errEtcdLeaderChange)
}
