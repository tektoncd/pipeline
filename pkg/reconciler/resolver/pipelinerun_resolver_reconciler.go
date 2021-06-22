/*
Copyright 2021 The Tekton Authors

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

package resolver

import (
	"context"
	"fmt"
	"log"

	"github.com/tektoncd/pipeline/internal/resolution"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

type PipelineRunResolverReconciler struct {
	// Implements reconciler.LeaderAware
	reconciler.LeaderAwareFuncs

	kubeClientSet     kubernetes.Interface
	pipelineClientSet clientset.Interface
	pipelinerunLister listers.PipelineRunLister
	configStore       reconciler.ConfigStore
}

var _ controller.Reconciler = &PipelineRunResolverReconciler{}

func (r *PipelineRunResolverReconciler) Reconcile(ctx context.Context, key string) error {
	if r.configStore != nil {
		ctx = r.configStore.ToContext(ctx)
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return controller.NewPermanentError(&ErrorInvalidResourceKey{key: key, original: err})
	}

	pr, err := r.pipelinerunLister.PipelineRuns(namespace).Get(name)
	if err != nil {
		return &ErrorGettingResource{kind: "pipelinerun", key: key, original: err}
	}

	_, err = ResolvePipelineRun(ctx, r.kubeClientSet, r.pipelineClientSet, pr)
	if err == resolution.ErrorPipelineRunAlreadyResolved {
		return nil
	} else if err != nil {
		updateErr := UpdatePipelineRunWithError(ctx, r.pipelineClientSet, pr.Namespace, pr.Name, err)
		log.Printf("UPDATE PIPELINERUN WITH ERR UPDATEERR: %v", updateErr)
		if updateErr != nil {
			// Don't return a permanent error here - it needs to be
			// reconciled again in case the underlying resource is
			// still in a transitional state.
			return err
		}
		return controller.NewPermanentError(err)
	}

	if _, err := PatchResolvedPipelineRun(ctx, r.kubeClientSet, r.pipelineClientSet, pr); err != nil {
		// We don't mark the pipelinerun failed here because error
		// responses from the api server might be transient.
		return err
	}

	return nil
}

func ResolvePipelineRun(ctx context.Context, kClient kubernetes.Interface, pClient clientset.Interface, pr *v1beta1.PipelineRun) (*v1beta1.PipelineRun, error) {
	req := resolution.PipelineRunResolutionRequest{
		KubeClientSet:     kClient,
		PipelineClientSet: pClient,
		PipelineRun:       pr,
	}

	if err := req.Resolve(ctx); err != nil {
		return nil, err
	}

	resolution.CopyPipelineMetaToPipelineRun(req.ResolvedPipelineMeta, pr)
	pr.Status.PipelineSpec = req.ResolvedPipelineSpec

	return pr, nil
}

// UpdatePipelineRunWithError updates a PipelineRun with a resolution error.
// Available publicly so that unit tests can leverage resolution machinery
// without relying on Patch, which our current fakes don't support.
//
// Returns an error if updating the PipelineRun doesn't work.
func UpdatePipelineRunWithError(ctx context.Context, client clientset.Interface, namespace, name string, resolutionError error) error {
	key := fmt.Sprintf("%s/%s", namespace, name)
	reason, err := PipelineRunResolutionReasonError(resolutionError)
	latestGenerationPR, err := client.TektonV1beta1().PipelineRuns(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logging.FromContext(ctx).Warnf("error getting pipelinerun %q to update it as failed: %v", key, err)
		return err
	}

	latestGenerationPR.Status.MarkFailed(reason, resolutionError.Error())
	_, err = client.TektonV1beta1().PipelineRuns(namespace).UpdateStatus(ctx, latestGenerationPR, metav1.UpdateOptions{})
	if err != nil {
		logging.FromContext(ctx).Warnf("error marking pipelinerun %q as failed: %v", key, err)
		return err
	}
	return nil
}

// PipelineRunResolutionReasonError extracts the reason and underlying error
// embedded in the given resolution.Error, or returns some sane defaults
// if the error isn't a resolution.Error.
func PipelineRunResolutionReasonError(err error) (string, error) {
	reason := resolution.ReasonPipelineRunResolutionFailed
	resolutionError := err

	if e, ok := err.(*resolution.Error); ok {
		reason = e.Reason
		resolutionError = e.Unwrap()
	}

	return reason, resolutionError
}
