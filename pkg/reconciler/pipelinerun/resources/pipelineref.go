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
	"knative.dev/pkg/logging"
)

// GetPipelineFunc is a factory function that will use the given PipelineRef to return a valid GetPipeline function that
// looks up the pipeline. It uses as context a k8s client, tekton client, namespace, and service account name to return
// the pipeline. It knows whether it needs to look in the cluster or in a remote location to fetch the reference.
func GetPipelineFunc(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, requester remoteresource.Requester, pipelineRun *v1beta1.PipelineRun) rprp.GetPipeline {
	cfg := config.FromContextOrDefaults(ctx)
	pr := pipelineRun.Spec.PipelineRef
	namespace := pipelineRun.Namespace
	// if the spec is already in the status, do not try to fetch it again, just use it as source of truth.
	// Same for the Source field in the Status.Provenance.
	if pipelineRun.Status.PipelineSpec != nil {
		return func(_ context.Context, name string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
			var configSource *v1beta1.ConfigSource
			if pipelineRun.Status.Provenance != nil {
				configSource = pipelineRun.Status.Provenance.ConfigSource
			}
			return &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: *pipelineRun.Status.PipelineSpec,
			}, configSource, nil
		}
	}

	switch {
	case cfg.FeatureFlags.EnableTektonOCIBundles && pr != nil && pr.Bundle != "":
		// Return an inline function that implements GetTask by calling Resolver.Get with the specified task type and
		// casting it to a PipelineObject.
		return func(ctx context.Context, name string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
			// If there is a bundle url at all, construct an OCI resolver to fetch the pipeline.
			kc, err := k8schain.New(ctx, k8s, k8schain.Options{
				Namespace:          namespace,
				ServiceAccountName: pipelineRun.Spec.ServiceAccountName,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get keychain: %w", err)
			}
			resolver := oci.NewResolver(pr.Bundle, kc)
			return resolvePipeline(ctx, resolver, name)
		}
	case pr != nil && pr.Resolver != "" && requester != nil:
		return func(ctx context.Context, name string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
			stringReplacements, arrayReplacements, objectReplacements := paramsFromPipelineRun(ctx, pipelineRun)
			for k, v := range GetContextReplacements("", pipelineRun) {
				stringReplacements[k] = v
			}
			replacedParams := replaceParamValues(pr.Params, stringReplacements, arrayReplacements, objectReplacements)
			resolver := resolution.NewResolver(requester, pipelineRun, string(pr.Resolver), "", "", replacedParams)
			return resolvePipeline(ctx, resolver, name)
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

// GetVerifiedPipelineFunc is a wrapper of GetPipelineFunc and return the function to
// verify the pipeline if resource-verification-mode is not "skip"
func GetVerifiedPipelineFunc(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, requester remoteresource.Requester, pipelineRun *v1beta1.PipelineRun, verificationpolicies []*v1alpha1.VerificationPolicy) rprp.GetPipeline {
	get := GetPipelineFunc(ctx, k8s, tekton, requester, pipelineRun)
	return func(context.Context, string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
		p, s, err := get(ctx, pipelineRun.Spec.PipelineRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get pipeline: %w", err)
		}
		// if the pipeline is in status, then it has been verified and no need to verify again
		if pipelineRun.Status.PipelineSpec != nil {
			return p, s, nil
		}
		var source string
		if s != nil {
			source = s.URI
		}
		logger := logging.FromContext(ctx)
		if config.CheckEnforceResourceVerificationMode(ctx) || config.CheckWarnResourceVerificationMode(ctx) {
			if err := trustedresources.VerifyPipeline(ctx, p, k8s, source, verificationpolicies); err != nil {
				if config.CheckEnforceResourceVerificationMode(ctx) {
					logger.Errorf("GetVerifiedPipelineFunc failed: %v", err)
					return nil, nil, fmt.Errorf("GetVerifiedPipelineFunc failed: %w: %v", trustedresources.ErrorResourceVerificationFailed, err)
				}
				logger.Warnf("GetVerifiedPipelineFunc failed: %v", err)
				return p, s, nil
			}
		}
		return p, s, nil
	}
}

// LocalPipelineRefResolver uses the current cluster to resolve a pipeline reference.
type LocalPipelineRefResolver struct {
	Namespace    string
	Tektonclient clientset.Interface
}

// GetPipeline will resolve a Pipeline from the local cluster using a versioned Tekton client. It will
// return an error if it can't find an appropriate Pipeline for any reason.
// TODO: if we want to set source for in-cluster pipeline, set it here.
// https://github.com/tektoncd/pipeline/issues/5522
func (l *LocalPipelineRefResolver) GetPipeline(ctx context.Context, name string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
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
// fetch a pipeline with given name. An error is returned if the
// resolution doesn't work or the returned data isn't a valid
// v1beta1.PipelineObject.
func resolvePipeline(ctx context.Context, resolver remote.Resolver, name string) (v1beta1.PipelineObject, *v1beta1.ConfigSource, error) {
	obj, configSource, err := resolver.Get(ctx, "pipeline", name)
	if err != nil {
		return nil, nil, err
	}
	pipelineObj, err := readRuntimeObjectAsPipeline(ctx, obj)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert obj %s into Pipeline", obj.GetObjectKind().GroupVersionKind().String())
	}
	return pipelineObj, configSource, nil
}

// readRuntimeObjectAsPipeline tries to convert a generic runtime.Object
// into a v1beta1.PipelineObject type so that its meta and spec fields
// can be read. An error is returned if the given object is not a
// PipelineObject or if there is an error validating or upgrading an
// older PipelineObject into its v1beta1 equivalent.
func readRuntimeObjectAsPipeline(ctx context.Context, obj runtime.Object) (v1beta1.PipelineObject, error) {
	if pipeline, ok := obj.(v1beta1.PipelineObject); ok {
		return pipeline, nil
	}

	return nil, errors.New("resource is not a pipeline")
}
