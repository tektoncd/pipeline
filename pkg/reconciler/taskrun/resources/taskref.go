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
	resolutionV1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/reconciler/apiserver"
	"github.com/tektoncd/pipeline/pkg/remote"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/remote/resolution"
	remoteresource "github.com/tektoncd/pipeline/pkg/remoteresolution/resource"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/kmeta"
)

// GetTaskKind returns the referenced Task kind (Task, ...) if the TaskRun is using TaskRef.
func GetTaskKind(taskrun *v1.TaskRun) v1.TaskKind {
	kind := v1.NamespacedTaskKind
	if taskrun.Spec.TaskRef != nil && taskrun.Spec.TaskRef.Kind != "" {
		kind = taskrun.Spec.TaskRef.Kind
	}
	return kind
}

// GetTaskFuncFromTaskRun is a factory function that will use the given TaskRef as context to return a valid GetTask function.
// It also requires a kubeclient, tektonclient, namespace, and service account in case it needs to find that task in
// cluster or authorize against an external repository. It will figure out whether it needs to look in the cluster or in
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

// GetTaskFunc is a factory function that will use the given TaskRef as context to return a valid GetTask function.
// It also requires a kubeclient, tektonclient, namespace, and service account in case it needs to find that task in
// cluster or authorize against an external repository. It will figure out whether it needs to look in the cluster or in
// a remote image to fetch the  reference. It will also return the "kind" of the task being referenced.
// OCI bundle and remote resolution tasks will be verified by trusted resources if the feature is enabled
func GetTaskFunc(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, requester remoteresource.Requester,
	owner kmeta.OwnerRefable, tr *v1.TaskRef, trName string, namespace, saName string, verificationPolicies []*v1alpha1.VerificationPolicy,
) GetTask {
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
			var url string
			if ownerAsTR, ok := owner.(*v1.TaskRun); ok {
				stringReplacements, arrayReplacements, _ := replacementsFromParams(ownerAsTR.Spec.Params)
				for k, v := range getContextReplacements("", ownerAsTR) {
					stringReplacements[k] = v
				}
				for _, p := range tr.Params {
					p.Value.ApplyReplacements(stringReplacements, arrayReplacements, nil)
					replacedParams = append(replacedParams, p)
				}
				if err := v1.RefNameLikeUrl(tr.Name); err == nil {
					// The name is url-like so its not a local reference.
					tr.Name = substitution.ApplyReplacements(tr.Name, stringReplacements)
					url = tr.Name
				}
			} else {
				replacedParams = append(replacedParams, tr.Params...)
			}
			resolverPayload := remoteresource.ResolverPayload{
				Name:      trName,
				Namespace: namespace,
				ResolutionSpec: &resolutionV1beta1.ResolutionRequestSpec{
					Params: replacedParams,
					URL:    url,
				},
			}
			resolver := resolution.NewResolver(requester, owner, string(tr.Resolver), resolverPayload)
			return resolveTask(ctx, resolver, name, namespace, kind, k8s, tekton, verificationPolicies)
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

// GetStepActionFunc is a factory function that will use the given Ref as context to return a valid GetStepAction function.
// It also requires a kubeclient, tektonclient, requester in case it needs to find that task in
// cluster or authorize against an external repository. It will figure out whether it needs to look in the cluster or in
// a remote location to fetch the reference.
func GetStepActionFunc(tekton clientset.Interface, k8s kubernetes.Interface, requester remoteresource.Requester, tr *v1.TaskRun, taskSpec v1.TaskSpec, step *v1.Step, taskNamespace string) GetStepAction {
	trName := tr.Name
	if step.Ref != nil && step.Ref.Resolver != "" && requester != nil {
		// Return an inline function that implements GetStepAction by calling Resolver.Get with the specified StepAction type and
		// casting it to a StepAction.
		return func(ctx context.Context, name string) (*v1beta1.StepAction, *v1.RefSource, error) {
			// Perform params replacements for StepAction resolver params
			ApplyParameterSubstitutionInResolverParams(tr, taskSpec, step)
			resolverPayload := remoteresource.ResolverPayload{
				Name:      trName,
				Namespace: taskNamespace,
				ResolutionSpec: &resolutionV1beta1.ResolutionRequestSpec{
					Params: step.Ref.Params,
					URL:    step.Ref.Name,
				},
			}
			resolver := resolution.NewResolver(requester, tr, string(step.Ref.Resolver), resolverPayload)
			return resolveStepAction(ctx, resolver, name, taskNamespace, k8s, tekton)
		}
	}
	local := &LocalStepActionRefResolver{
		Namespace:    taskNamespace,
		Tektonclient: tekton,
	}
	return local.GetStepAction
}

// ApplyParameterSubstitutionInResolverParams applies parameter substitutions in resolver params for Step Ref.
func ApplyParameterSubstitutionInResolverParams(tr *v1.TaskRun, taskSpec v1.TaskSpec, step *v1.Step) {
	stringReplacements := make(map[string]string)
	arrayReplacements := make(map[string][]string)
	objectReplacements := make(map[string]map[string]string)

	defaultSR, defaultAR, defaultOR := replacementsFromDefaultParams(taskSpec.Params)
	stringReplacements, arrayReplacements, objectReplacements = extendReplacements(stringReplacements, arrayReplacements, objectReplacements, defaultSR, defaultAR, defaultOR)

	paramSR, paramAR, paramOR := replacementsFromParams(tr.Spec.Params)
	stringReplacements, arrayReplacements, objectReplacements = extendReplacements(stringReplacements, arrayReplacements, objectReplacements, paramSR, paramAR, paramOR)
	step.Ref.Params = step.Ref.Params.ReplaceVariables(stringReplacements, arrayReplacements, objectReplacements)
}

func extendReplacements(stringReplacements map[string]string, arrayReplacements map[string][]string, objectReplacements map[string]map[string]string, stringReplacementsToAdd map[string]string, arrayReplacementsToAdd map[string][]string, objectReplacementsToAdd map[string]map[string]string) (map[string]string, map[string][]string, map[string]map[string]string) {
	for k, v := range stringReplacementsToAdd {
		stringReplacements[k] = v
	}
	for k, v := range arrayReplacementsToAdd {
		arrayReplacements[k] = v
	}
	objectReplacements = extendObjectReplacements(objectReplacements, objectReplacementsToAdd)
	return stringReplacements, arrayReplacements, objectReplacements
}

func extendObjectReplacements(objectReplacements map[string]map[string]string, objectReplacementsToAdd map[string]map[string]string) map[string]map[string]string {
	for k, v := range objectReplacementsToAdd {
		for key, val := range v {
			if objectReplacements != nil {
				if objectReplacements[k] != nil {
					objectReplacements[k][key] = val
				} else {
					objectReplacements[k] = v
				}
			}
		}
	}
	return objectReplacements
}

// resolveTask accepts an impl of remote.Resolver and attempts to
// fetch a task with given name and verify the v1beta1 task if trusted resources is enabled.
// An error is returned if the remoteresource doesn't work
// A VerificationResult is returned if trusted resources is enabled, VerificationResult contains the result type and err.
// or the returned data isn't a valid *v1beta1.Task.
func resolveTask(ctx context.Context, resolver remote.Resolver, name, namespace string, kind v1.TaskKind, k8s kubernetes.Interface, tekton clientset.Interface, verificationPolicies []*v1alpha1.VerificationPolicy) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
	// Because the resolver will only return references with the same kind, this will ensure we
	// don't accidentally return a Task with the same name but different kind.
	obj, refSource, err := resolver.Get(ctx, strings.TrimSuffix(strings.ToLower(string(kind)), "s"), name)
	if err != nil {
		return nil, nil, nil, err
	}
	taskObj, vr, err := readRuntimeObjectAsTask(ctx, namespace, obj, k8s, tekton, refSource, verificationPolicies)
	if err != nil {
		return nil, nil, nil, err
	}
	return taskObj, refSource, vr, nil
}

func resolveStepAction(ctx context.Context, resolver remote.Resolver, name, namespace string, k8s kubernetes.Interface, tekton clientset.Interface) (*v1beta1.StepAction, *v1.RefSource, error) {
	obj, refSource, err := resolver.Get(ctx, "StepAction", name)
	if err != nil {
		return nil, nil, err
	}
	switch obj := obj.(type) {
	case *v1beta1.StepAction:
		// Cleanup object from things we don't care about
		// FIXME: extract this in a function
		obj.ObjectMeta.OwnerReferences = nil
		o, err := apiserver.DryRunValidate(ctx, namespace, obj, tekton)
		if err != nil {
			return nil, nil, err
		}
		if mutatedStepAction, ok := o.(*v1beta1.StepAction); ok {
			mutatedStepAction.ObjectMeta = obj.ObjectMeta
			return mutatedStepAction, refSource, nil
		}
	case *v1alpha1.StepAction:
		obj.SetDefaults(ctx)
		// Cleanup object from things we don't care about
		// FIXME: extract this in a function
		obj.ObjectMeta.OwnerReferences = nil
		o, err := apiserver.DryRunValidate(ctx, namespace, obj, tekton)
		if err != nil {
			return nil, nil, err
		}
		if mutatedStepAction, ok := o.(*v1alpha1.StepAction); ok {
			mutatedStepAction.ObjectMeta = obj.ObjectMeta
			v1BetaStepAction := v1beta1.StepAction{
				TypeMeta: metav1.TypeMeta{
					Kind:       "StepAction",
					APIVersion: "tekton.dev/v1beta1",
				},
			}
			err := mutatedStepAction.ConvertTo(ctx, &v1BetaStepAction)
			if err != nil {
				return nil, nil, err
			}
			return &v1BetaStepAction, refSource, nil
		}
	}
	return nil, nil, errors.New("resource is not a StepAction")
}

// readRuntimeObjectAsTask tries to convert a generic runtime.Object
// into a *v1.Task type so that its meta and spec fields
// can be read. v1beta1 object will be converted to v1 and returned.
// An error is returned if the given object is not a Task
// or if there is an error validating or upgrading an older TaskObject into
// its v1beta1 equivalent.
// A VerificationResult is returned if trusted resources is enabled, VerificationResult contains the result type and err.
// v1beta1 task will be verified by trusted resources if the feature is enabled
// TODO(#5541): convert v1beta1 obj to v1 once we use v1 as the stored version
func readRuntimeObjectAsTask(ctx context.Context, namespace string, obj runtime.Object, k8s kubernetes.Interface, tekton clientset.Interface, refSource *v1.RefSource, verificationPolicies []*v1alpha1.VerificationPolicy) (*v1.Task, *trustedresources.VerificationResult, error) {
	switch obj := obj.(type) {
	case *v1beta1.Task:
		obj.SetDefaults(ctx)
		// Cleanup object from things we don't care about
		// FIXME: extract this in a function
		obj.ObjectMeta.OwnerReferences = nil
		// Verify the Task once we fetch from the remote resolution, mutating, validation and conversion of the task should happen after the verification, since signatures are based on the remote task contents
		vr := trustedresources.VerifyResource(ctx, obj, k8s, refSource, verificationPolicies)
		// Issue a dry-run request to create the remote Task, so that it can undergo validation from validating admission webhooks
		// without actually creating the Task on the cluster.
		o, err := apiserver.DryRunValidate(ctx, namespace, obj, tekton)
		if err != nil {
			return nil, nil, err
		}
		if mutatedTask, ok := o.(*v1beta1.Task); ok {
			t := &v1.Task{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Task",
					APIVersion: "tekton.dev/v1",
				},
			}
			mutatedTask.ObjectMeta = obj.ObjectMeta
			if err := mutatedTask.ConvertTo(ctx, t); err != nil {
				return nil, nil, fmt.Errorf("failed to convert obj %s into Pipeline", mutatedTask.GetObjectKind().GroupVersionKind().String())
			}
			return t, &vr, nil
		}
	case *v1.Task:
		// This SetDefaults is currently not necessary, but for consistency, it is recommended to add it.
		// Avoid forgetting to add it in the future when there is a v2 version, causing similar problems.
		obj.SetDefaults(ctx)
		// Cleanup object from things we don't care about
		// FIXME: extract this in a function
		obj.ObjectMeta.OwnerReferences = nil
		vr := trustedresources.VerifyResource(ctx, obj, k8s, refSource, verificationPolicies)
		// Issue a dry-run request to create the remote Task, so that it can undergo validation from validating admission webhooks
		// without actually creating the Task on the cluster
		o, err := apiserver.DryRunValidate(ctx, namespace, obj, tekton)
		if err != nil {
			return nil, nil, err
		}
		if mutatedTask, ok := o.(*v1.Task); ok {
			mutatedTask.ObjectMeta = obj.ObjectMeta
			return mutatedTask, &vr, nil
		}
	}
	return nil, nil, errors.New("resource is not a task")
}

// LocalTaskRefResolver uses the current cluster to resolve a task reference.
type LocalTaskRefResolver struct {
	Namespace    string
	Kind         v1.TaskKind
	Tektonclient clientset.Interface
}

// GetTask will resolve a Task from the local cluster using a versioned Tekton client. It will
// return an error if it can't find an appropriate Task for any reason.
// TODO(#6666): support local task verification
func (l *LocalTaskRefResolver) GetTask(ctx context.Context, name string) (*v1.Task, *v1.RefSource, *trustedresources.VerificationResult, error) {
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

// LocalStepActionRefResolver uses the current cluster to resolve a StepAction reference.
type LocalStepActionRefResolver struct {
	Namespace    string
	Tektonclient clientset.Interface
}

// GetStepAction will resolve a StepAction from the local cluster using a versioned Tekton client.
// It will return an error if it can't find an appropriate StepAction for any reason.
func (l *LocalStepActionRefResolver) GetStepAction(ctx context.Context, name string) (*v1beta1.StepAction, *v1.RefSource, error) {
	// If we are going to resolve this reference locally, we need a namespace scope.
	if l.Namespace == "" {
		return nil, nil, fmt.Errorf("must specify namespace to resolve reference to step action %s", name)
	}
	stepAction, err := l.Tektonclient.TektonV1beta1().StepActions(l.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return stepAction, nil, nil
}
