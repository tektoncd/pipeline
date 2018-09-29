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
	"fmt"
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/knative/build-pipeline/pkg/controller"
	"github.com/knative/build/pkg/builder"
	"github.com/knative/pkg/logging/logkey"

	clientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned"
	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"

	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	buildclientset "github.com/knative/build/pkg/client/clientset/versioned"
	buildinformers "github.com/knative/build/pkg/client/informers/externalversions"
	buildlisters "github.com/knative/build/pkg/client/listers/build/v1alpha1"
)

const controllerAgentName = "pipeline-controller"

var (
	groupVersionKind = schema.GroupVersionKind{
		Group:   v1alpha1.SchemeGroupVersion.Group,
		Version: v1alpha1.SchemeGroupVersion.Version,
		Kind:    "Task", // TODO(aaron-prindle) verify Task is correct here
	}
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Build is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Build fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by TaskRun"
	// MessageResourceSynced is the message used for an Event fired when a Build
	// is synced successfully
	MessageResourceSynced = "Build synced successfully"
)

// Controller is the controller implementation for Build resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// pipelineclientset is a clientset for our own API group
	pipelineclientset clientset.Interface
	buildclientset    buildclientset.Interface

	// taskrunsLister listers.BuildLister
	// tasksLister    listers.BuildTemplateLister
	taskrunsLister listers.TaskRunLister
	tasksLister    listers.TaskLister
	buildsLister   buildlisters.BuildLister

	taskrunsSynced cache.InformerSynced

	// The builder through which work will be done.
	builder builder.Interface
	// taskrunner builder.Interface

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
	// Sugared logger is easier to use but is not as performant as the
	// raw logger. In performance critical paths, call logger.Desugar()
	// and use the returned raw logger instead. In addition to the
	// performance benefits, raw logger also preserves type-safety at
	// the expense of slightly greater verbosity.
	logger *zap.SugaredLogger
}

// NewController returns a new taskrun controller
func NewController(
	builder builder.Interface,
	kubeclientset kubernetes.Interface,
	pipelineclientset clientset.Interface,
	buildclientset buildclientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	pipelineInformerFactory informers.SharedInformerFactory,
	buildInformerFactory buildinformers.SharedInformerFactory,
	logger *zap.SugaredLogger,
) controller.Interface {

	// obtain a reference to a shared index informer for the Build type.
	// buildInformer := pipelineInformerFactory.Pipeline().V1alpha1().Builds()
	// buildTemplateInformer := pipelineInformerFactory.Pipeline().V1alpha1().BuildTemplates()
	taskrunInformer := pipelineInformerFactory.Pipeline().V1alpha1().TaskRuns()
	taskInformer := pipelineInformerFactory.Pipeline().V1alpha1().Tasks()
	buildInformer := buildInformerFactory.Build().V1alpha1().Builds()

	// Enrich the logs with controller name
	logger = logger.Named(controllerAgentName).With(zap.String(logkey.ControllerType, controllerAgentName))

	// Create event broadcaster
	logger.Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logger.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		builder:           builder,
		kubeclientset:     kubeclientset,
		pipelineclientset: pipelineclientset,
		taskrunsLister:    taskrunInformer.Lister(),
		tasksLister:       taskInformer.Lister(),
		buildsLister:      buildInformer.Lister(),
		taskrunsSynced:    taskrunInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Builds"),
		recorder:          recorder,
		logger:            logger,
	}

	logger.Info("Setting up event handlers")
	// Set up an event handler for when Build resources change
	taskrunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueBuild,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueBuild(new)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	c.logger.Info("Starting Build controller")

	// Wait for the caches to be synced before starting workers
	c.logger.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.taskrunsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	c.logger.Info("Starting workers")
	// Launch two workers to process Build resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	c.logger.Info("Started workers")
	<-stopCh
	c.logger.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	if err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		key, ok := obj.(string)
		if !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Build resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		c.logger.Infof("Successfully synced '%s'", key)
		return nil
	}(obj); err != nil {
		runtime.HandleError(err)
	}

	return true
}

// enqueueBuild takes a Build resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Build.
func (c *Controller) enqueueBuild(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Build resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	taskrun, err := c.taskrunsLister.TaskRuns(namespace).Get(name)
	if err != nil {
		// The Build resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("build '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}
	// Don't mutate the informer's copy of our object.
	taskrun = taskrun.DeepCopy()

	// Get related task for taskrun
	task, err := c.pipelineclientset.PipelineV1alpha1().Tasks("tasknamespace").Get(
		"name", metav1.GetOptions{})

	build, err := c.getBuild(namespace, name)
	if errors.IsNotFound(err) {
		// taken from https://github.com/kubernetes/test-infra/blob/8faa040989cf8552683dcdd4400505ca62a3c408/prow/cmd/build/controller.go#L255
		if build, err = makeBuild(*task); err != nil {
			return fmt.Errorf("make build: %v", err)
		}
		fmt.Printf("Create builds/%s", key)
		if build, err = c.createBuild(namespace, build); err != nil {
			return fmt.Errorf("create build: %v", err)
		}
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the build is not controlled by this taskrun resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(build, taskrun) { // TODO(aaron-prindle) taskrun OR task?
		msg := fmt.Sprintf(MessageResourceExists, build.Name)
		c.recorder.Event(taskrun, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// TODO(aaron-prindle) TaskRun status monitoring?

	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	// err = c.updateTaskRunStatus(taskrun, build)
	// if err != nil {
	// 	return err
	// }

	c.recorder.Event(taskrun, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) getBuild(namespace, name string) (*buildv1alpha1.Build, error) {
	return c.buildsLister.Builds(namespace).Get(name)
}

func (c *Controller) createBuild(namespace string, b *buildv1alpha1.Build) (*buildv1alpha1.Build, error) {
	return c.buildclientset.BuildV1alpha1().Builds(namespace).Create(b)
}

func (c *Controller) waitForOperationAsync(taskrun *v1alpha1.TaskRun, op builder.Operation) error {
	go func() {
		status, err := op.Wait()
		if err != nil {
			c.logger.Errorf("Error while waiting for operation: %v", err)
			return
		}
		taskrun.Status = *status
		if _, err := c.updateStatus(taskrun); err != nil {
			c.logger.Errorf("Error updating taskrun status: %v", err)
		}
	}()
	return nil
}

func (c *Controller) updateStatus(u *v1alpha1.TaskRun) (*v1alpha1.TaskRun, error) {
	taskrunClient := c.pipelineclientset.PipelineV1alpha1().TaskRuns(u.Namespace)
	newu, err := taskrunClient.Get(u.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	newu.Status = u.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Build resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	return taskrunClient.Update(newu)
}

// makeBuild creates a build from a task, using the tasks's buildspec.
func makeBuild(task v1alpha1.Task) (*buildv1alpha1.Build, error) {
	if task.Spec.BuildSpec == nil {
		return nil, fmt.Errorf("nil BuildSpec")
	}
	return &buildv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      task.Name,
			Namespace: task.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&task, groupVersionKind),
			},
		},
		Spec: *task.Spec.BuildSpec,
	}, nil
}
