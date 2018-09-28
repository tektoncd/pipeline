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

package pipelinerun

import (
	"context"
	"reflect"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/build-pipeline/pkg/reconciler/constants"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
)

type reconcilerConfig struct {
	pipelineRunLister listers.PipelineRunLister
	pipelineLister    listers.PipelineLister
}

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	pipelineRunLister listers.PipelineRunLister
	pipelineLister    listers.PipelineLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController creates a new Configuration controller
func NewController(
	opt reconciler.Options,
	pipelineRunInformer informers.PipelineRunInformer,
	pipelineInformer informers.PipelineInformer,
) *controller.Impl {

	rc := &reconcilerConfig{
		pipelineRunLister: pipelineRunInformer.Lister(),
		pipelineLister:    pipelineInformer.Lister(),
	}
	return newController(opt, pipelineRunInformer, pipelineInformer, rc)
}

func newController(opt reconciler.Options,
	pipelineRunInformer informers.PipelineRunInformer,
	pipelineInformer informers.PipelineInformer,
	rc *reconcilerConfig,
) *controller.Impl {

	r := &Reconciler{
		Base:              reconciler.NewBase(opt, constants.PipelineRunAgentName),
		pipelineRunLister: pipelineRunInformer.Lister(),
		pipelineLister:    pipelineInformer.Lister(),
	}

	impl := controller.NewImpl(r, r.Logger, constants.PipelineRunControllerName)

	r.Logger.Info("Setting up event handlers")
	pipelineRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
		DeleteFunc: impl.Enqueue,
	})

	pipelineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
		DeleteFunc: impl.Enqueue,
	})

	return impl
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Task Run
// resource with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}
	logger := logging.FromContext(ctx)

	// Get the Pipeline Run resource with this namespace/name
	original, err := c.pipelineRunLister.PipelineRuns(namespace).Get(name)
	if errors.IsNotFound(err) {
		// TODO: create pipelineRun here
		// The resource no longer exists, in which case we stop processing.
		logger.Errorf("pipeline run %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informer's copy.
	pr := original.DeepCopy()

	// Reconcile this copy of the task run and then write back any status
	// updates regardless of whether the reconciliation errored out.
	err = c.reconcile(ctx, pr)
	if equality.Semantic.DeepEqual(original.Status, pr.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := c.updateStatus(pr); err != nil {
		logger.Warn("Failed to update taskPipeline status", zap.Error(err))
		return err
	}
	return err
}

func (c *Reconciler) reconcile(ctx context.Context, pr *v1alpha1.PipelineRun) error {
	logger := logging.FromContext(ctx)

	// fetch the equivelant task for this task Run
	name := pr.Spec.PipelineRef.Name
	p, err := c.pipelineLister.Pipelines(pr.Namespace).Get(name)
	if err != nil {
		logger.Errorf("Failed to reconcile PipelineRun: %q failed to Get Pipeline: %q", pr.Name, p)
		return err
	}

	// TODO fetch the taskruns status

	// check status of tasks and update status of PipelineRuns

	return nil
}

func (c *Reconciler) updateStatus(pr *v1alpha1.PipelineRun) (*v1alpha1.PipelineRun, error) {
	newPr, err := c.pipelineRunLister.PipelineRuns(pr.Namespace).Get(pr.Name)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(newPr.Status, pr.Status) {
		newPr.Status = pr.Status
		return c.PipelineClientSet.PipelineV1alpha1().PipelineRuns(pr.Namespace).Update(newPr)
	}
	return newPr, nil
}
