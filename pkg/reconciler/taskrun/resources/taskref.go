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
	"errors"
	"fmt"
	"strings"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/remote"
	"github.com/tektoncd/pipeline/pkg/remote/resolution"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/kmeta"
)

// This error is defined in etcd at
// https://github.com/etcd-io/etcd/blob/5b226e0abf4100253c94bb71f47d6815877ed5a2/server/etcdserver/errors.go#L30
// TODO: If/when https://github.com/kubernetes/kubernetes/issues/106491 is addressed,
// we should stop relying on a hardcoded string.
var errEtcdLeaderChange = "etcdserver: leader changed"

// GetTaskKind returns the referenced Task kind (Task, ClusterTask, ...) if the TaskRun is using TaskRef.
func GetTaskKind(taskrun *v1.TaskRun) v1.TaskKind {
	kind := v1.NamespacedTaskKind
	if taskrun.Spec.TaskRef != nil && taskrun.Spec.TaskRef.Kind != "" {
		kind = taskrun.Spec.TaskRef.Kind
	}
	return kind
}

// GetTaskFuncFromTaskRun is a factory function that will use the given TaskRef as context to return a valid GetTask function. It
// also requires a kubeclient, tektonclient, namespace, and service account in case it needs to find that task in
// cluster or authorize against an external repositroy. It will figure out whether it needs to look in the cluster or in
// a remote image to fetch the  reference. It will also return the "kind" of the task being referenced.
// OCI bundle and remote resolution tasks will be verified by trusted resources if the feature is enabled
func GetTaskFuncFromTaskRun(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, requester remoteresource.Requester, taskrun *v1.TaskRun, verificationPolicies []*v1alpha1.VerificationPolicy) GetTask {
	// if the spec is already in the status, do not try to fetch it again, just use it as source of truth.
	// Same for the RefSource field in the Status.Provenance.
	if taskrun.Status.TaskSpec != nil {
		return func(_ context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
			var refSource *v1.RefSource
			if taskrun.Status.Provenance != nil {
				refSource = taskrun.Status.Provenance.RefSource
			}
			return &v1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: taskrun.Namespace,
				},
				Spec: *taskrun.Status.TaskSpec,
			}, refSource, nil, nil
		}
	}
	return GetTaskFunc(ctx, k8s, tekton, requester, taskrun, taskrun.Spec.TaskRef, taskrun.Name, taskrun.Namespace, taskrun.Spec.ServiceAccountName, verificationPolicies)
}

// GetTaskFunc is a factory function that will use the given TaskRef as context to return a valid GetTask function. It
// also requires a kubeclient, tektonclient, namespace, and service account in case it needs to find that task in
// cluster or authorize against an external repositroy. It will figure out whether it needs to look in the cluster or in
// a remote image to fetch the  reference. It will also return the "kind" of the task being referenced.
// OCI bundle and remote resolution tasks will be verified by trusted resources if the feature is enabled
func GetTaskFunc(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, requester remoteresource.Requester,
	owner kmeta.OwnerRefable, tr *v1.TaskRef, trName string, namespace, saName string, verificationPolicies []*v1alpha1.VerificationPolicy) GetTask {
	kind := v1.NamespacedTaskKind
	if tr != nil && tr.Kind != "" {
		kind = tr.Kind
	}

	switch {
	case tr != nil && tr.Resolver != "" && requester != nil:
		// Return an inline function that implements GetTask by calling Resolver.Get with the specified task type and
		// casting it to a TaskObject.
		return func(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
			var replacedParams v1.Params
			if ownerAsTR, ok := owner.(*v1.TaskRun); ok {
				stringReplacements, arrayReplacements := paramsFromTaskRun(ctx, ownerAsTR)
				for k, v := range getContextReplacements("", ownerAsTR) {
					stringReplacements[k] = v
				}
				for _, p := range tr.Params {
					p.Value.ApplyReplacements(stringReplacements, arrayReplacements, nil)
					replacedParams = append(replacedParams, p)
				}
			} else {
				replacedParams = append(replacedParams, tr.Params...)
			}
			resolver := resolution.NewResolver(requester, owner, string(tr.Resolver), trName, namespace, replacedParams)
			return resolveTask(ctx, resolver, name, kind, k8s, verificationPolicies)
		}

	default:
		// Even if there is no task ref, we should try to return a local resolver.
		local := &LocalTaskRefResolver{
			Namespace:    namespace,
			Kind:         kind,
			Tektonclient: tekton,
		}
		return local.GetTask
	}
}

// resolveTask accepts an impl of remote.Resolver and attempts to
// fetch a task with given name and verify the v1beta1 task if trusted resources is enabled.
// An error is returned if the remoteresource doesn't work
// A VerificationResult is returned if trusted resources is enabled, VerificationResult contains the result type and err.
// or the returned data isn't a valid *v1beta1.Task.
func resolveTask(ctx context.Context, resolver remote.Resolver, name string, kind v1.TaskKind, k8s kubernetes.Interface, verificationPolicies []*v1alpha1.VerificationPolicy) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
	// Because the resolver will only return references with the same kind (eg ClusterTask), this will ensure we
	// don't accidentally return a Task with the same name but different kind.
	obj, refSource, err := resolver.Get(ctx, strings.TrimSuffix(strings.ToLower(string(kind)), "s"), name)
	if err != nil {
		return nil, nil, nil, err
	}
	taskObj, vr, err := readRuntimeObjectAsTask(ctx, obj, k8s, refSource, verificationPolicies)
	if err != nil {
		return nil, nil, nil, err
	}
	return taskObj, refSource, vr, nil
}

// readRuntimeObjectAsTask tries to convert a generic runtime.Object
// into a *v1.Task type so that its meta and spec fields
// can be read. v1beta1 object will be converted to v1 and returned.
// An error is returned if the given object is not a Task nor a ClusterTask
// or if there is an error validating or upgrading an older TaskObject into
// its v1beta1 equivalent.
// A VerificationResult is returned if trusted resources is enabled, VerificationResult contains the result type and err.
// v1beta1 task will be verified by trusted resources if the feature is enabled
// TODO(#5541): convert v1beta1 obj to v1 once we use v1 as the stored version
func readRuntimeObjectAsTask(ctx context.Context, obj runtime.Object, k8s kubernetes.Interface, refSource *v1.RefSource, verificationPolicies []*v1alpha1.VerificationPolicy) (*v1.Task, *trustedresources.VerificationResult, error) {
	switch obj := obj.(type) {
	case *v1beta1.Task:
		// Verify the Task once we fetch from the remote resolution, mutating, validation and conversion of the task should happen after the verification, since signatures are based on the remote task contents
		vr := trustedresources.VerifyResource(ctx, obj, k8s, refSource, verificationPolicies)
		t := &v1.Task{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Task",
				APIVersion: "tekton.dev/v1",
			},
		}
		if err := obj.ConvertTo(ctx, t); err != nil {
			return nil, nil, fmt.Errorf("failed to convert obj %s into Pipeline", obj.GetObjectKind().GroupVersionKind().String())
		}
		return t, &vr, nil
	case *v1beta1.ClusterTask:
		t, err := convertClusterTaskToTask(ctx, *obj)
		return t, nil, err
	case *v1.Task:
		vr := trustedresources.VerifyResource(ctx, obj, k8s, refSource, verificationPolicies)
		// Validation of beta fields must happen before the V1 Task is converted into the storage version of the API.
		// TODO(#6592): Decouple API versioning from feature versioning
		if err := obj.Spec.ValidateBetaFields(ctx); err != nil {
			return nil, nil, fmt.Errorf("invalid Task %s: %w", obj.GetName(), err)
		}
		return obj, &vr, nil
	}
	return nil, nil, errors.New("resource is not a task")
}

// LocalTaskRefResolver uses the current cluster to resolve a task reference.
type LocalTaskRefResolver struct {
	Namespace    string
	Kind         v1.TaskKind
	Tektonclient clientset.Interface
}

// GetTask will resolve either a Task or ClusterTask from the local cluster using a versioned Tekton client. It will
// return an error if it can't find an appropriate Task for any reason.
// TODO(#6666): support local task verification
func (l *LocalTaskRefResolver) GetTask(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
	if l.Kind == v1.ClusterTaskRefKind {
		task, err := l.Tektonclient.TektonV1beta1().ClusterTasks().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, nil, err
		}
		v1task, err := convertClusterTaskToTask(ctx, *task)
		return v1task, nil, nil, err
	}

	// If we are going to resolve this reference locally, we need a namespace scope.
	if l.Namespace == "" {
		return nil, nil, nil, fmt.Errorf("must specify namespace to resolve reference to task %s", name)
	}
	task, err := l.Tektonclient.TektonV1().Tasks(l.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	return task, nil, nil, nil
}

// IsGetTaskErrTransient returns true if an error returned by GetTask is retryable.
func IsGetTaskErrTransient(err error) bool {
	return strings.Contains(err.Error(), errEtcdLeaderChange)
}

// convertClusterTaskToTask converts deprecated v1beta1 ClusterTasks to Tasks for
// the rest of reconciling process since GetTask func and its upstream callers only
// fetches the task spec and stores it in the taskrun status while the kind info
// is not being used.
func convertClusterTaskToTask(ctx context.Context, ct v1beta1.ClusterTask) (*v1.Task, error) {
	t := &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Task",
			APIVersion: "tekton.dev/v1beta1",
		},
	}

	t.Spec = ct.Spec
	t.ObjectMeta.Name = ct.ObjectMeta.Name

	v1Task := &v1.Task{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Task",
			APIVersion: "tekton.dev/v1",
		},
	}

	if err := t.ConvertTo(ctx, v1Task); err != nil {
		return nil, err
	}

	return v1Task, nil
}
