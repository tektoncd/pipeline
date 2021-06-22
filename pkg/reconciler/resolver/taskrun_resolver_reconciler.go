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

type TaskRunResolverReconciler struct {
	// Implements reconciler.LeaderAware
	reconciler.LeaderAwareFuncs

	kubeClientSet     kubernetes.Interface
	pipelineClientSet clientset.Interface
	taskrunLister     listers.TaskRunLister
	configStore       reconciler.ConfigStore
}

const defaultTaskRunResolutionErrorReason = resolution.ReasonTaskRunResolutionFailed

var _ controller.Reconciler = &TaskRunResolverReconciler{}

func (r *TaskRunResolverReconciler) Reconcile(ctx context.Context, key string) error {
	if r.configStore != nil {
		ctx = r.configStore.ToContext(ctx)
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return controller.NewPermanentError(&ErrorInvalidResourceKey{key: key, original: err})
	}

	tr, err := r.taskrunLister.TaskRuns(namespace).Get(name)
	if err != nil {
		return &ErrorGettingResource{kind: "taskrun", key: key, original: err}
	}

	if _, err := ResolveTaskRun(ctx, r.kubeClientSet, r.pipelineClientSet, tr); err == resolution.ErrorTaskRunAlreadyResolved {
		// Nothing to do: another process has already resolved the taskrun.
		return nil
	} else if err != nil {
		resolutionError := err

		latestGenerationTR, err := r.pipelineClientSet.TektonV1beta1().TaskRuns(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			logging.FromContext(ctx).Warnf("error getting taskrun %q to update it as failed: %v", key, err)
			return resolutionError
		}

		MarkTaskRunResolutionError(latestGenerationTR, resolutionError)

		latestGenerationTR, err = r.pipelineClientSet.TektonV1beta1().TaskRuns(namespace).UpdateStatus(ctx, latestGenerationTR, metav1.UpdateOptions{})
		if err != nil {
			logging.FromContext(ctx).Warnf("error updating taskrun %q as failed: %v", key, err)
			return resolutionError
		}

		return controller.NewPermanentError(resolutionError)
	}

	if _, err := PatchResolvedTaskRun(ctx, r.kubeClientSet, r.pipelineClientSet, tr); err != nil {
		// We don't mark the taskrun failed here because error
		// responses from the api server might be transient.
		return err
	}

	return nil
}

// ResolveTaskRun executes default resolution behaviour: first fetching the
// task that the taskrun references and then copying metadata over from task to
// taskrun.
//
// This func is public so that unit tests can leverage resolution machinery
// without creating an entire resolver controller and, more importantly,
// without requiring a faked api server to handle patch requests, which our
// current fake does not.
func ResolveTaskRun(ctx context.Context, kClient kubernetes.Interface, pClient clientset.Interface, tr *v1beta1.TaskRun) (*v1beta1.TaskRun, error) {
	req := resolution.TaskRunResolutionRequest{
		KubeClientSet:     kClient,
		PipelineClientSet: pClient,
		TaskRun:           tr,
	}

	if err := req.Resolve(ctx); err != nil {
		return nil, err
	}

	resolution.CopyTaskMetaToTaskRun(req.ResolvedTaskMeta, tr)
	tr.Status.TaskSpec = req.ResolvedTaskSpec

	return tr, nil
}

// MarkTaskRunResolutionError updates a TaskRun's condition with error
// information.
//
// This func is public so that unit tests can leverage resolution machinery
// without creating an entire resolver controller and, more importantly,
// without requiring a faked api server to handle patch requests, which our
// current fake does not.
func MarkTaskRunResolutionError(tr *v1beta1.TaskRun, err error) {
	reason := defaultTaskRunResolutionErrorReason
	if e, ok := err.(*resolution.Error); ok {
		reason = e.Reason
	}
	tr.Status.MarkResourceFailed(v1beta1.TaskRunReason(reason), err)
}
