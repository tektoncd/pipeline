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
	"fmt"
	"reflect"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	resources "github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	buildinformers "github.com/knative/build/pkg/client/informers/externalversions/build/v1alpha1"
	buildlisters "github.com/knative/build/pkg/client/listers/build/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
)

const (
	// taskRunAgentName defines logging agent name for TaskRun Controller
	taskRunAgentName = "taskrun-controller"
	// taskRunControllerName defines name for TaskRun Controller
	taskRunControllerName = "TaskRun"
)

var (
	groupVersionKind = schema.GroupVersionKind{
		Group:   v1alpha1.SchemeGroupVersion.Group,
		Version: v1alpha1.SchemeGroupVersion.Version,
		Kind:    "TaskRun",
	}
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	taskRunLister  listers.TaskRunLister
	taskLister     listers.TaskLister
	buildLister    buildlisters.BuildLister
	resourceLister listers.PipelineResourceLister
	tracker        tracker.Interface
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController creates a new Configuration controller
func NewController(
	opt reconciler.Options,
	taskRunInformer informers.TaskRunInformer,
	taskInformer informers.TaskInformer,
	buildInformer buildinformers.BuildInformer,
	resourceInformer informers.PipelineResourceInformer,
) *controller.Impl {

	c := &Reconciler{
		Base:           reconciler.NewBase(opt, taskRunAgentName),
		taskRunLister:  taskRunInformer.Lister(),
		taskLister:     taskInformer.Lister(),
		buildLister:    buildInformer.Lister(),
		resourceLister: resourceInformer.Lister(),
	}
	impl := controller.NewImpl(c, c.Logger, taskRunControllerName)

	c.Logger.Info("Setting up event handlers")
	taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
	})

	// TODO(aaron-prindle) what to do if a task is deleted?
	// taskInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 	AddFunc:    impl.Enqueue,
	// 	UpdateFunc: controller.PassNew(impl.Enqueue),
	// 	DeleteFunc: impl.Enqueue,
	// })

	c.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())
	buildInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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

	// Get the Task Run resource with this namespace/name
	original, err := c.taskRunLister.TaskRuns(namespace).Get(name)
	if errors.IsNotFound(err) {
		// The resource no longer exists, in which case we stop processing.
		c.Logger.Errorf("task run %q in work queue no longer exists", key)
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
		c.Logger.Warn("Failed to update taskRun status", zap.Error(err))
		return err
	}
	return err
}

func (c *Reconciler) reconcile(ctx context.Context, tr *v1alpha1.TaskRun) error {
	// get build the same as the taskrun, this is the value we use for 1:1 mapping and retrieval
	b, err := c.getBuild(tr.Namespace, tr.Name)
	if errors.IsNotFound(err) {
		if b, err = c.makeBuild(tr, c.Logger); err != nil {
			return fmt.Errorf("Failed to create a build for taskrun: %v", err)
		}
	} else if err != nil {
		return fmt.Errorf("Failed retrieving build %s for taskRun %s: %v", tr.Name, tr.Name, err)
	}
	// handle cases where build with name exists but handled by another controller

	// switch ownerref := metav1.GetControllerOf(b); {
	// case ownerref == nil, ownerref.APIVersion != groupVersionKind.GroupVersion().String(), ownerref.Kind != groupVersionKind.Kind:
	// 	logger.Infof("build %s not controlled by taskrun controller", b.Name)
	// 	return nil
	// }

	// taskrun has finished (as child build has finished and status is synced)
	if len(tr.Status.Conditions) > 0 && tr.Status.Conditions[0].Status != corev1.ConditionUnknown {
		c.Logger.Infof("Finished %s", tr.Name)
		return nil
	}

	// sync build status with taskrun status
	if len(b.Status.Conditions) > 0 {
		c.Logger.Infof("Syncing taskrun conditions with build conditions %s", b.Status.Conditions[0])
	} else {
		c.Logger.Infof("Syncing taskrun conditions with build conditions []")
	}
	tr.Status.Conditions = b.Status.Conditions
	return nil
}

func (c *Reconciler) updateStatus(taskrun *v1alpha1.TaskRun) (*v1alpha1.TaskRun, error) {
	newtaskrun, err := c.taskRunLister.TaskRuns(taskrun.Namespace).Get(taskrun.Name)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(taskrun.Status, newtaskrun.Status) {
		newtaskrun.Status = taskrun.Status
		return c.PipelineClientSet.PipelineV1alpha1().TaskRuns(taskrun.Namespace).Update(newtaskrun)
	}
	return newtaskrun, nil
}

func (c *Reconciler) getBuild(namespace, name string) (*buildv1alpha1.Build, error) {
	return c.buildLister.Builds(namespace).Get(name)
}

func (c *Reconciler) deleteBuild(namespace, name string) error {
	return c.BuildClientSet.BuildV1alpha1().Builds(namespace).Delete(name, &metav1.DeleteOptions{})
}

func (c *Reconciler) deleteTaskRun(namespace, name string) error {
	return c.PipelineClientSet.PipelineV1alpha1().TaskRuns(namespace).Delete(name, &metav1.DeleteOptions{})
}

// makeBuild creates a build from the task, using the task's buildspec.
func (c *Reconciler) makeBuild(tr *v1alpha1.TaskRun, logger *zap.SugaredLogger) (*buildv1alpha1.Build, error) {
	// Get related task for taskrun
	t, err := c.taskLister.Tasks(tr.Namespace).Get(tr.Spec.TaskRef.Name)
	if err != nil {
		return nil, err
	}

	if t.Spec.BuildSpec == nil {
		return nil, fmt.Errorf("BuildSpec for task %s is nil", t.Name)
	}
	b := &buildv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tr.Name,
			Namespace: tr.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(tr, groupVersionKind),
			},
		},
		Spec: *t.Spec.BuildSpec,
	}

	build, err := resources.AddInputResource(b, t, tr, c.resourceLister, logger)
	if err != nil {
		logger.Errorf("Failed to create a build for taskrun: %s due to input resource error %v", tr.Name, err)
		return nil, err
	}

	createdBuild, err := c.BuildClientSet.BuildV1alpha1().Builds(tr.Namespace).Create(build)
	if err != nil {
		logger.Errorf("Failed to create build for taskrun %s, %v", tr.Name, err)
		return nil, err
	}

	buildRef := corev1.ObjectReference{
		APIVersion: "build.knative.dev/v1alpha1",
		Kind:       "Build",
		Namespace:  tr.Namespace,
		Name:       tr.Name,
	}
	if err := c.tracker.Track(buildRef, tr); err != nil {
		logger.Errorf("Failed to create tracker for build %s for taskrun %s: %v", buildRef, tr.Name, err)
		return nil, err
	}

	return createdBuild, nil
}
