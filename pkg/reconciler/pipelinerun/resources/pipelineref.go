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

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/remote/oci"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetPipelineFunc is a factory function that will use the given PipelineRef to return a valid GetPipeline function that
// looks up the pipeline. It uses as context a k8s client, tekton client, namespace, and service account name to return
// the pipeline. It knows whether it needs to look in the cluster or in a remote image to fetch the reference.
func GetPipelineFunc(ctx context.Context, k8s kubernetes.Interface, tekton clientset.Interface, pipelineRun *v1beta1.PipelineRun) (GetPipeline, error) {
	cfg := config.FromContextOrDefaults(ctx)
	pr := pipelineRun.Spec.PipelineRef
	namespace := pipelineRun.Namespace
	// if the spec is already in the status, do not try to fetch it again, just use it as source of truth
	if pipelineRun.Status.PipelineSpec != nil {
		return func(_ context.Context, name string) (v1beta1.PipelineObject, error) {
			return &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: *pipelineRun.Status.PipelineSpec,
			}, nil
		}, nil
	}
	switch {
	case cfg.FeatureFlags.EnableTektonOCIBundles && pr != nil && pr.Bundle != "":
		// Return an inline function that implements GetTask by calling Resolver.Get with the specified task type and
		// casting it to a PipelineObject.
		return func(ctx context.Context, name string) (v1beta1.PipelineObject, error) {
			// If there is a bundle url at all, construct an OCI resolver to fetch the pipeline.
			kc, err := k8schain.New(ctx, k8s, k8schain.Options{
				Namespace:          namespace,
				ServiceAccountName: pipelineRun.Spec.ServiceAccountName,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get keychain: %w", err)
			}
			resolver := oci.NewResolver(pr.Bundle, kc)

			obj, err := resolver.Get("pipeline", name)
			if err != nil {
				return nil, err
			}
			if pipeline, ok := obj.(v1beta1.PipelineObject); ok {
				pipeline.SetDefaults(ctx)
				return pipeline, nil
			}

			if pipeline, ok := obj.(*v1alpha1.Pipeline); ok {
				betaPipeline := &v1beta1.Pipeline{}
				err := pipeline.ConvertTo(ctx, betaPipeline)
				betaPipeline.SetDefaults(ctx)
				return betaPipeline, err
			}

			return nil, fmt.Errorf("failed to convert obj %s into Pipeline", obj.GetObjectKind().GroupVersionKind().String())
		}, nil
	default:
		// Even if there is no task ref, we should try to return a local resolver.
		local := &LocalPipelineRefResolver{
			Namespace:    namespace,
			Tektonclient: tekton,
		}
		return local.GetPipeline, nil
	}
}

// LocalPipelineRefResolver uses the current cluster to resolve a pipeline reference.
type LocalPipelineRefResolver struct {
	Namespace    string
	Tektonclient clientset.Interface
}

// GetPipeline will resolve a Pipeline from the local cluster using a versioned Tekton client. It will
// return an error if it can't find an appropriate Pipeline for any reason.
func (l *LocalPipelineRefResolver) GetPipeline(ctx context.Context, name string) (v1beta1.PipelineObject, error) {
	// If we are going to resolve this reference locally, we need a namespace scope.
	if l.Namespace == "" {
		return nil, fmt.Errorf("Must specify namespace to resolve reference to pipeline %s", name)
	}
	return l.Tektonclient.TektonV1beta1().Pipelines(l.Namespace).Get(ctx, name, metav1.GetOptions{})
}
