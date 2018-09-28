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
package taskrun

import (
	"context"
	"reflect"
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/build-pipeline/pkg/reconciler/constants"
	buildinformers "github.com/knative/build/pkg/client/informers/externalversions/build/v1alpha1"
	buildlisters "github.com/knative/build/pkg/client/listers/build/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/tracker"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	taskRunLister              listers.TaskRunLister
	taskLister                 listers.TaskLister
	buildLister                buildlisters.BuildLister
	buildTemplateLister        buildlisters.BuildTemplateLister
	clusterBuildTemplateLister buildlisters.ClusterBuildTemplateLister
	tracker                    tracker.Interface
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController creates a new Configuration controller
func NewController(
	opt reconciler.Options,
	taskRunInformer informers.TaskRunInformer,
	taskInformer informers.TaskInformer,
	buildInformer buildinformers.BuildInformer,
	buildTemplateInformer buildinformers.BuildTemplateInformer,
	clusterBuildTemplateInformer buildinformers.ClusterBuildTemplateInformer,

) *controller.Impl {

	c := &Reconciler{
		Base:                       reconciler.NewBase(opt, constants.TaskRunAgentName),
		taskRunLister:              taskRunInformer.Lister(),
		taskLister:                 taskInformer.Lister(),
		buildLister:                buildInformer.Lister(),
		buildTemplateLister:        buildTemplateInformer.Lister(),
		clusterBuildTemplateLister: clusterBuildTemplateInformer.Lister(),
	}
	impl := controller.NewImpl(c, c.Logger, constants.TaskRunControllerName)

	c.Logger.Info("Setting up event handlers")
	taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
		DeleteFunc: impl.Enqueue,
	})

	taskInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
		DeleteFunc: impl.Enqueue,
	})

	c.tracker = tracker.New(impl.EnqueueKey, 30*time.Minute)
	buildInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.tracker.OnChanged,
		UpdateFunc: controller.PassNew(c.tracker.OnChanged),
	})

	buildTemplateInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.tracker.OnChanged,
		UpdateFunc: controller.PassNew(c.tracker.OnChanged),
	})

	clusterBuildTemplateInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.tracker.OnChanged,
		UpdateFunc: controller.PassNew(c.tracker.OnChanged),
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

	// Get the Task Run resource with this namespace/name
	original, err := c.taskRunLister.TaskRuns(namespace).Get(name)
	if errors.IsNotFound(err) {

		// TODO: create taskRun here

		// The resource no longer exists, in which case we stop processing.
		logger.Errorf("task run %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informer's copy.
	tr := original.DeepCopy()

	// Reconcile this copy of the task run and then write back any status
	// updates regardless of whether the reconciliation errored out.
	err = c.reconcile(ctx, tr)
	if equality.Semantic.DeepEqual(original.Status, tr.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := c.updateStatus(tr); err != nil {
		logger.Warn("Failed to update taskRun status", zap.Error(err))
		return err
	}
	return err
}

func (c *Reconciler) reconcile(ctx context.Context, tr *v1alpha1.TaskRun) error {
	logger := logging.FromContext(ctx)

	// fetch the equivelant task for this task Run
	taskName := tr.Spec.TaskRef.Name
	task, err := c.taskLister.Tasks(tr.Namespace).Get(taskName)
	if err != nil {
		logger.Errorf("Failed to reconcile TaskRun: %q failed to Get Task: %q", tr.Name, taskName)
		return err
	}
	// fetch the build that should exist for the current task run
	buildSpec := task.Spec.BuildSpec
	if len(buildSpec.Steps) == 0 && buildSpec.Template.Name == "" {
		logger.Errorf("Failed to reconcile TaskRun: %q failed to Get BuildSpec: %q", tr.Name, taskName)
		return err
	}
	// TODO use build tracker to fetch build

	// check status of build and update status of taskRun

	return nil
}

func (c *Reconciler) updateStatus(taskrun *v1alpha1.TaskRun) (*v1alpha1.TaskRun, error) {
	newtaskrun, err := c.taskRunLister.TaskRuns(taskrun.Namespace).Get(taskrun.Name)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(newtaskrun.Status, taskrun.Status) {
		newtaskrun.Status = taskrun.Status
		// TODO: for CRD there's no updatestatus, so use normal update
		return c.PipelineClientSet.PipelineV1alpha1().TaskRuns(taskrun.Namespace).Update(newtaskrun)
		//	return configClient.UpdateStatus(newtaskrun)
	}
	return newtaskrun, nil
}
