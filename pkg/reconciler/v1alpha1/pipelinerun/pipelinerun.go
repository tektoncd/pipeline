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
	"fmt"
	"reflect"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/build-pipeline/pkg/util"
	"github.com/knative/pkg/controller"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// pipelineRunAgentName defines logging agent name for PipelineRun Controller
	pipelineRunAgentName = "pipeline-controller"
	// pipelineRunControllerName defines name for PipelineRun Controller
	pipelineRunControllerName = "PipelineRun"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	pipelineRunLister listers.PipelineRunLister
	pipelineLister    listers.PipelineLister
	taskLister        listers.TaskLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController creates a new Configuration controller
func NewController(
	opt reconciler.Options,
	pipelineRunInformer informers.PipelineRunInformer,
	pipelineInformer informers.PipelineInformer,
	taskInformer informers.TaskInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:              reconciler.NewBase(opt, pipelineRunAgentName),
		pipelineRunLister: pipelineRunInformer.Lister(),
		pipelineLister:    pipelineInformer.Lister(),
	}

	impl := controller.NewImpl(r, r.Logger, pipelineRunControllerName)

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
// converge the two. It then updates the Status block of the Pipeline Run
// resource with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the Pipeline Run resource with this namespace/name
	original, err := c.pipelineRunLister.PipelineRuns(namespace).Get(name)
	if errors.IsNotFound(err) {
		// The resource no longer exists, in which case we stop processing.
		c.Logger.Errorf("pipeline run %q in work queue no longer exists", key)
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
		c.Logger.Warn("Failed to update PipelineRun status", zap.Error(err))
		return err
	}
	return err
}

func (c *Reconciler) reconcile(ctx context.Context, pr *v1alpha1.PipelineRun) error {
	// fetch the equivelant pipeline for this pipelinerun Run
	if _, err := c.getPipelineIfExists(pr); err != nil {
		return err
	}
	// TODO fetch the taskruns status for this pipeline run.

	// TODO check status of tasks and update status of PipelineRuns

	return nil
}

func (c *Reconciler) createPipelineRun(ctx context.Context, p *v1alpha1.Pipeline) error {
	tasks, err := c.getTasks(p)
	if err != nil {
		return errors.NewBadRequest(fmt.Sprintf("pipeline %q is not valid due to %v", p.Name, err))
	}
	for _, t := range tasks {
		tr, err := createTaskRun(t)
		if err != nil {
			return err
		}
		c.PipelineClientSet.PipelineV1alpha1().TaskRuns(p.Namespace).Create(tr)
	}
	return nil
}

func (c *Reconciler) getTasks(p *v1alpha1.Pipeline) ([]*v1alpha1.Task, error) {
	tasks := make([]*v1alpha1.Task, len(p.Spec.Tasks))
	for i, t := range p.Spec.Tasks {
		tObj, err := c.taskLister.Tasks(p.Namespace).Get(t.TaskRef.Name)
		if err != nil {
			c.Logger.Errorf("failed to get tasks for Pipeline %q: Error getting task %q : %s",
				fmt.Sprintf("%s/%s", p.Namespace, p.Name),
				fmt.Sprintf("%s/%s", p.Namespace, t.Name), err)
			return nil, err
		}
		tasks[i] = tObj
	}
	return tasks, nil
}

func createTaskRun(t *v1alpha1.Task) (*v1alpha1.TaskRun, error) {
	// Create empty tasks
	trName := randomizeTaskRunName(t)
	tr := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trName,
			Namespace: t.Namespace,
		},
	}
	return tr, nil
}

func randomizeTaskRunName(t *v1alpha1.Task) string {
	return util.AppendRandomString(t.Name)
}

func (c *Reconciler) getPipelineIfExists(pr *v1alpha1.PipelineRun) (*v1alpha1.Pipeline, error) {
	p, err := c.pipelineLister.Pipelines(pr.Namespace).Get(pr.Spec.PipelineRef.Name)
	if errors.IsNotFound(err) {
		// to stop processing if pipeline is not found, return nil
		c.Logger.Errorf("%q failed to Get Pipeline: %q. not found",
			fmt.Sprintf("%s/%s", pr.Namespace, pr.Name),
			fmt.Sprintf("%s/%s", pr.Namespace, pr.Spec.PipelineRef.Name))
		return nil, nil
	}
	return p, err
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
