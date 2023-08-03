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

	"github.com/google/uuid"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	rprp "github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/pipelinespec"
	"github.com/tektoncd/pipeline/pkg/remote"
	"github.com/tektoncd/pipeline/pkg/remote/resolution"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrReferencedPipelineValidationFailed = errors.New("validation failed for referenced Pipeline")
	ErrCouldntValidatePipelineRetryable   = errors.New("retryable error validating referenced Pipeline")
	ErrCouldntValidatePipelinePermanent   = errors.New("permanent error validating referenced Pipeline")
)

// GetPipelineFunc is a factory function that will use the given PipelineRef to return a valid GetPipeline function that
// looks up the pipeline. It uses as context a k8s client, tekton client, namespace, and service account name to return
// the pipeline. It knows whether it needs to look in the cluster or in a remote location to fetch the reference.
// OCI bundle and remote resolution pipelines will be verified by trusted resources if the feature is enabled
func GetPipelineFunc(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, requester remoteresource.Requester, pipelineRun *v1.PipelineRun, verificationPolicies []*v1alpha1.VerificationPolicy) rprp.GetPipeline {
	pr := pipelineRun.Spec.PipelineRef
	namespace := pipelineRun.Namespace
	// if the spec is already in the status, do not try to fetch it again, just use it as source of truth.
	// Same for the RefSource field in the Status.Provenance.
	if pipelineRun.Status.PipelineSpec != nil {
		return func(_ context.Context, name string) (*v1.Pipeline, *v1.RefSource, *trustedresources.VerificationResult, error) {
			var refSource *v1.RefSource
			if pipelineRun.Status.Provenance != nil {
				refSource = pipelineRun.Status.Provenance.RefSource
			}
			return &v1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: *pipelineRun.Status.PipelineSpec,
			}, refSource, nil, nil
		}
	}

	switch {
	case pr != nil && pr.Resolver != "" && requester != nil:
		return func(ctx context.Context, name string) (*v1.Pipeline, *v1.RefSource, *trustedresources.VerificationResult, error) {
			stringReplacements, arrayReplacements, objectReplacements := paramsFromPipelineRun(ctx, pipelineRun)
			for k, v := range GetContextReplacements("", pipelineRun) {
				stringReplacements[k] = v
			}
			replacedParams := pr.Params.ReplaceVariables(stringReplacements, arrayReplacements, objectReplacements)

			resolver := resolution.NewResolver(requester, pipelineRun, string(pr.Resolver), "", "", replacedParams)
			return resolvePipeline(ctx, resolver, name, namespace, k8s, tekton, verificationPolicies)
		}
	default:
		// Even if there is no pipeline ref, we should try to return a local resolver.
		local := &LocalPipelineRefResolver{
			Namespace:    namespace,
			Tektonclient: tekton,
		}
		return local.GetPipeline
	}
}

// LocalPipelineRefResolver uses the current cluster to resolve a pipeline reference.
type LocalPipelineRefResolver struct {
	Namespace    string
	Tektonclient clientset.Interface
}

// GetPipeline will resolve a Pipeline from the local cluster using a versioned Tekton client. It will
// return an error if it can't find an appropriate Pipeline for any reason.
// TODO: if we want to set RefSource for in-cluster pipeline, set it here.
// https://github.com/tektoncd/pipeline/issues/5522
// TODO(#6666): Support local resources verification
func (l *LocalPipelineRefResolver) GetPipeline(ctx context.Context, name string) (*v1.Pipeline, *v1.RefSource, *trustedresources.VerificationResult, error) {
	// If we are going to resolve this reference locally, we need a namespace scope.
	if l.Namespace == "" {
		return nil, nil, nil, fmt.Errorf("Must specify namespace to resolve reference to pipeline %s", name)
	}

	pipeline, err := l.Tektonclient.TektonV1().Pipelines(l.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	return pipeline, nil, nil, nil
}

// resolvePipeline accepts an impl of remote.Resolver and attempts to
// fetch a pipeline with given name and verify the v1beta1 pipeline if trusted resources is enabled.
// An error is returned if the remoteresource doesn't work
// A VerificationResult is returned if trusted resources is enabled, VerificationResult contains the result type and err.
// or the returned data isn't a valid *v1.Pipeline.
func resolvePipeline(ctx context.Context, resolver remote.Resolver, name string, namespace string, k8s kubernetes.Interface, tekton clientset.Interface, verificationPolicies []*v1alpha1.VerificationPolicy) (*v1.Pipeline, *v1.RefSource, *trustedresources.VerificationResult, error) {
	obj, refSource, err := resolver.Get(ctx, "pipeline", name)
	if err != nil {
		return nil, nil, nil, err
	}
	pipelineObj, vr, err := readRuntimeObjectAsPipeline(ctx, namespace, obj, k8s, tekton, refSource, verificationPolicies)
	if err != nil {
		return nil, nil, nil, err
	}
	return pipelineObj, refSource, vr, nil
}

// readRuntimeObjectAsPipeline tries to convert a generic runtime.Object
// into a *v1.Pipeline type so that its meta and spec fields
// can be read. v1 object will be converted to v1beta1 and returned.
// v1beta1 Pipeline will be verified if trusted resources is enabled
// A VerificationResult is returned if trusted resources is enabled, VerificationResult contains the result type and err.
// An error is returned if the given object is not a
// PipelineObject or if there is an error validating or upgrading an
// older PipelineObject into its v1beta1 equivalent.
// TODO(#5541): convert v1beta1 obj to v1 once we use v1 as the stored version
func readRuntimeObjectAsPipeline(ctx context.Context, namespace string, obj runtime.Object, k8s kubernetes.Interface, tekton clientset.Interface, refSource *v1.RefSource, verificationPolicies []*v1alpha1.VerificationPolicy) (*v1.Pipeline, *trustedresources.VerificationResult, error) {
	switch obj := obj.(type) {
	case *v1beta1.Pipeline:
		// Verify the Pipeline once we fetch from the remote resolution, mutating, validation and conversion of the pipeline should happen after the verification, since signatures are based on the remote pipeline contents
		vr := trustedresources.VerifyResource(ctx, obj, k8s, refSource, verificationPolicies)
		// Issue a dry-run request to create the remote Pipeline, so that it can undergo validation from validating admission webhooks
		// without actually creating the Pipeline on the cluster.
		// Validation must happen before the v1beta1 Pipeline is converted into the storage version of the API,
		// since validation of beta features differs between v1 and v1beta1
		// TODO(#6592): Decouple API versioning from feature versioning
		dryRunObj := obj.DeepCopy()
		dryRunObj.Name = uuid.NewString() // Use a randomized name for the Pipeline in case there is already another Pipeline of the same name
		dryRunObj.Namespace = namespace   // Make sure the namespace is the same as the PipelineRun
		if _, err := tekton.TektonV1beta1().Pipelines(namespace).Create(ctx, dryRunObj, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}}); err != nil {
			return nil, nil, handleDryRunCreateErr(err, obj.Name)
		}
		p := &v1.Pipeline{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pipeline",
				APIVersion: "tekton.dev/v1",
			},
		}
		if err := obj.ConvertTo(ctx, p); err != nil {
			return nil, nil, fmt.Errorf("failed to convert obj %s into Pipeline", obj.GetObjectKind().GroupVersionKind().String())
		}
		return p, &vr, nil
	case *v1.Pipeline:
		vr := trustedresources.VerifyResource(ctx, obj, k8s, refSource, verificationPolicies)
		// Issue a dry-run request to create the remote Pipeline, so that it can undergo validation from validating admission webhooks
		// without actually creating the Pipeline on the cluster
		dryRunObj := obj.DeepCopy()
		dryRunObj.Name = uuid.NewString() // Use a randomized name for the Pipeline in case there is already another Pipeline of the same name
		dryRunObj.Namespace = namespace   // Make sure the namespace is the same as the PipelineRun
		if _, err := tekton.TektonV1().Pipelines(namespace).Create(ctx, dryRunObj, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}}); err != nil {
			return nil, nil, handleDryRunCreateErr(err, obj.Name)
		}
		return obj, &vr, nil
	}
	return nil, nil, errors.New("resource is not a pipeline")
}

func handleDryRunCreateErr(err error, objectName string) error {
	var errType error
	switch {
	case apierrors.IsBadRequest(err): // Pipeline rejected by validating webhook
		errType = ErrReferencedPipelineValidationFailed
	case apierrors.IsInvalid(err), apierrors.IsMethodNotSupported(err):
		errType = ErrCouldntValidatePipelinePermanent
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err), apierrors.IsTooManyRequests(err):
		errType = ErrCouldntValidatePipelineRetryable
	default:
		// Assume unknown errors are retryable
		// Additional errors can be added to the switch statements as needed
		errType = ErrCouldntValidatePipelineRetryable
	}
	return fmt.Errorf("%w %s: %s", errType, objectName, err.Error())
}
