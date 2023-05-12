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

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	rprp "github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/pipelinespec"
	"github.com/tektoncd/pipeline/pkg/remote"
	"github.com/tektoncd/pipeline/pkg/remote/oci"
	"github.com/tektoncd/pipeline/pkg/remote/resolution"
	remoteresource "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// GetPipelineFunc is a factory function that will use the given PipelineRef to return a valid GetPipeline function that
// looks up the pipeline. It uses as context a k8s client, tekton client, namespace, and service account name to return
// the pipeline. It knows whether it needs to look in the cluster or in a remote location to fetch the reference.
// OCI bundle and remote resolution pipelines will be verified by trusted resources if the feature is enabled
func GetPipelineFunc(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, requester remoteresource.Requester, pipelineRun *v1beta1.PipelineRun, verificationPolicies []*v1alpha1.VerificationPolicy) rprp.GetPipeline {
	cfg := config.FromContextOrDefaults(ctx)
	pr := pipelineRun.Spec.PipelineRef
	namespace := pipelineRun.Namespace
	// if the spec is already in the status, do not try to fetch it again, just use it as source of truth.
	// Same for the RefSource field in the Status.Provenance.
	if pipelineRun.Status.PipelineSpec != nil {
		return func(_ context.Context, name string) (*v1beta1.Pipeline, *v1beta1.RefSource, error) {
			var refSource *v1beta1.RefSource
			if pipelineRun.Status.Provenance != nil {
				refSource = pipelineRun.Status.Provenance.RefSource
			}
			return &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: *pipelineRun.Status.PipelineSpec,
			}, refSource, nil
		}
	}

	switch {
	case cfg.FeatureFlags.EnableTektonOCIBundles && pr != nil && pr.Bundle != "":
		// Return an inline function that implements GetTask by calling Resolver.Get with the specified task type and
		// casting it to a PipelineObject.
		return func(ctx context.Context, name string) (*v1beta1.Pipeline, *v1beta1.RefSource, error) {
			// If there is a bundle url at all, construct an OCI resolver to fetch the pipeline.
			kc, err := k8schain.New(ctx, k8s, k8schain.Options{
				Namespace:          namespace,
				ServiceAccountName: pipelineRun.Spec.ServiceAccountName,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get keychain: %w", err)
			}
			resolver := oci.NewResolver(pr.Bundle, kc)
			return resolvePipeline(ctx, resolver, name, k8s, verificationPolicies)
		}
	case pr != nil && pr.Resolver != "" && requester != nil:
		return func(ctx context.Context, name string) (*v1beta1.Pipeline, *v1beta1.RefSource, error) {
			stringReplacements, arrayReplacements, objectReplacements := paramsFromPipelineRun(ctx, pipelineRun)
			for k, v := range GetContextReplacements("", pipelineRun) {
				stringReplacements[k] = v
			}
			replacedParams := replaceParamValues(pr.Params, stringReplacements, arrayReplacements, objectReplacements)
			resolver := resolution.NewResolver(requester, pipelineRun, string(pr.Resolver), "", "", replacedParams)
			return resolvePipeline(ctx, resolver, name, k8s, verificationPolicies)
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
func (l *LocalPipelineRefResolver) GetPipeline(ctx context.Context, name string) (*v1beta1.Pipeline, *v1beta1.RefSource, error) {
	// If we are going to resolve this reference locally, we need a namespace scope.
	if l.Namespace == "" {
		return nil, nil, fmt.Errorf("Must specify namespace to resolve reference to pipeline %s", name)
	}

	pipeline, err := l.Tektonclient.TektonV1beta1().Pipelines(l.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return pipeline, nil, nil
}

// resolvePipeline accepts an impl of remote.Resolver and attempts to
// fetch a pipeline with given name and verify the v1beta1 pipeline if trusted resources is enabled.
// An error is returned if the resolution doesn't work, the verification fails
// or the returned data isn't a valid *v1beta1.Pipeline.
func resolvePipeline(ctx context.Context, resolver remote.Resolver, name string, k8s kubernetes.Interface, verificationPolicies []*v1alpha1.VerificationPolicy) (*v1beta1.Pipeline, *v1beta1.RefSource, error) {
	obj, refSource, err := resolver.Get(ctx, "pipeline", name)
	if err != nil {
		return nil, nil, err
	}
	pipelineObj, err := readRuntimeObjectAsPipeline(ctx, obj, k8s, refSource, verificationPolicies)
	if err != nil {
		return nil, nil, err
	}
	return pipelineObj, refSource, nil
}

// readRuntimeObjectAsPipeline tries to convert a generic runtime.Object
// into a *v1beta1.Pipeline type so that its meta and spec fields
// can be read. v1 object will be converted to v1beta1 and returned.
// v1beta1 Pipeline will be verified if trusted resources is enabled
// An error is returned if the given object is not a
// PipelineObject, trusted resources verification fails or if there is an error validating or upgrading an
// older PipelineObject into its v1beta1 equivalent.
// TODO(#5541): convert v1beta1 obj to v1 once we use v1 as the stored version
func readRuntimeObjectAsPipeline(ctx context.Context, obj runtime.Object, k8s kubernetes.Interface, refSource *v1beta1.RefSource, verificationPolicies []*v1alpha1.VerificationPolicy) (*v1beta1.Pipeline, error) {
	switch obj := obj.(type) {
	case *v1beta1.Pipeline:
		// Verify the Pipeline once we fetch from the remote resolution, mutating, validation and conversion of the pipeline should happen after the verification, since signatures are based on the remote pipeline contents
		err := trustedresources.VerifyPipeline(ctx, obj, k8s, refSource, verificationPolicies)
		if err != nil {
			return nil, fmt.Errorf("remote Pipeline verification failed for object %s: %w", obj.GetName(), err)
		}
		return obj, nil
	case *v1.Pipeline:
		// TODO(#6356): Support V1 Task verification
		t := &v1beta1.Pipeline{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pipeline",
				APIVersion: "tekton.dev/v1beta1",
			},
		}
		if err := t.ConvertFrom(ctx, obj); err != nil {
			return nil, fmt.Errorf("failed to convert obj %s into Pipeline", obj.GetObjectKind().GroupVersionKind().String())
		}
		return t, nil
	}

	return nil, errors.New("resource is not a pipeline")
}
