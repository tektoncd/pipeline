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

package build

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/knative/build/pkg/builder"
	"github.com/knative/build/pkg/controller"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"

	clientset "github.com/knative/build/pkg/client/clientset/versioned"
	informers "github.com/knative/build/pkg/client/informers/externalversions"
	listers "github.com/knative/build/pkg/client/listers/build/v1alpha1"
)

const controllerAgentName = "build-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Build is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Build fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceSynced is the message used for an Event fired when a Build
	// is synced successfully
	MessageResourceSynced = "Build synced successfully"
)

// Controller is the controller implementation for Build resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// buildclientset is a clientset for our own API group
	buildclientset clientset.Interface

	buildsLister         listers.BuildLister
	buildTemplatesLister listers.BuildTemplateLister
	buildsSynced         cache.InformerSynced

	// The builder through which work will be done.
	builder builder.Interface

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new build controller
func NewController(
	builder builder.Interface,
	kubeclientset kubernetes.Interface,
	buildclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	buildInformerFactory informers.SharedInformerFactory) controller.Interface {

	// obtain a reference to a shared index informer for the Build type.
	buildInformer := buildInformerFactory.Build().V1alpha1().Builds()
	buildTemplateInformer := buildInformerFactory.Build().V1alpha1().BuildTemplates()

	// Create event broadcaster
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		builder:              builder,
		kubeclientset:        kubeclientset,
		buildclientset:       buildclientset,
		buildsLister:         buildInformer.Lister(),
		buildTemplatesLister: buildTemplateInformer.Lister(),
		buildsSynced:         buildInformer.Informer().HasSynced,
		workqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Builds"),
		recorder:             recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Build resources change
	buildInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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
	glog.Info("Starting Build controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.buildsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Build resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

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
		glog.Infof("Successfully synced '%s'", key)
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

	// Get the Build resource with this namespace/name
	build, err := c.buildsLister.Builds(namespace).Get(name)
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
	build = build.DeepCopy()

	// If the build's done, then ignore it.
	if !builder.IsDone(&build.Status) {
		// If the build is not done, but is in progress (has an operation), then asynchronously wait for it.
		// TODO(mattmoor): Check whether the Builder matches the kind of our c.builder.
		if build.Status.Builder != "" {
			op, err := c.builder.OperationFromStatus(&build.Status)
			if err != nil {
				return err
			}
			if err := c.waitForOperationAsync(build, op); err != nil {
				return err
			}
		} else {
			build.Status.Builder = c.builder.Builder()
			// If the build hasn't even started, then start it and record the operation in our status.
			// Note that by recording our status, we will trigger a reconciliation, so the wait above
			// will kick in.
			var tmpl *v1alpha1.BuildTemplate
			if build.Spec.Template != nil {
				tmpl, err = c.buildTemplatesLister.BuildTemplates(namespace).Get(build.Spec.Template.Name)
				if err != nil {
					// The BuildTemplate resource may not exist.
					if errors.IsNotFound(err) {
						runtime.HandleError(fmt.Errorf("build template %q in namespace %q does not exist", key, namespace))
					}
					return err
				}
			}
			build, err = builder.ApplyTemplate(build, tmpl)
			if err != nil {
				return err
			}
			// TODO: Validate build except steps+template
			b, err := c.builder.BuildFromSpec(build)
			if err != nil {
				return err
			}
			op, err := b.Execute()
			if err != nil {
				build.Status.SetCondition(&v1alpha1.BuildCondition{
					Type:    v1alpha1.BuildSucceeded,
					Status:  corev1.ConditionFalse,
					Reason:  "BuildExecuteFailed",
					Message: err.Error(),
				})
				if _, err := c.updateStatus(build); err != nil {
					return err
				}
				return err
			}
			if err := op.Checkpoint(&build.Status); err != nil {
				return err
			}
			build, err = c.updateStatus(build)
			if err != nil {
				return err
			}
		}
	}

	c.recorder.Event(build, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) waitForOperationAsync(build *v1alpha1.Build, op builder.Operation) error {
	go func() {
		status, err := op.Wait()
		if err != nil {
			glog.Errorf("Error while waiting for operation: %v", err)
			return
		}
		build.Status = *status
		if _, err := c.updateStatus(build); err != nil {
			glog.Errorf("Error updating build status: %v", err)
		}
	}()
	return nil
}

func (c *Controller) updateStatus(u *v1alpha1.Build) (*v1alpha1.Build, error) {
	buildClient := c.buildclientset.BuildV1alpha1().Builds(u.Namespace)
	newu, err := buildClient.Get(u.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	newu.Status = u.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Build resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	return buildClient.Update(newu)
}
