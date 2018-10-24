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

	"github.com/knative/build-pipeline/pkg/apis/pipeline"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	buildinformers "github.com/knative/build/pkg/client/informers/externalversions/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
)

const (
	// ReasonCouldntGetTask indicates that the reason for the failure status is that the
	// Task couldn't be found
	ReasonCouldntGetTask = "CouldntGetTask"

	// ReasonFailedValidation indicated that the reason for failure status is
	// that pipelinerun failed runtime validation
	ReasonFailedValidation = "TaskRunValidationFailed"

	// ReasonRunning indicates that the reason for the inprogress status is that the TaskRun
	// is just starting to be reconciled
	ReasonRunning = "Running"

	// taskRunAgentName defines logging agent name for TaskRun Controller
	taskRunAgentName = "taskrun-controller"
	// taskRunControllerName defines name for TaskRun Controller
	taskRunControllerName = "TaskRun"

	pvcSizeBytes = 5 * 1024 * 1024 * 1024 // 5 GBs
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
		resourceLister: resourceInformer.Lister(),
	}
	impl := controller.NewImpl(c, c.Logger, taskRunControllerName)

	c.Logger.Info("Setting up event handlers")
	taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
	})

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
		c.Logger.Infof("task run %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		c.Logger.Errorf("Error retreiving TaskRun %q: %s", name, err)
		return err
	}

	// Don't modify the informer's copy.
	tr := original.DeepCopy()
	tr.Status.InitializeConditions()

	// Reconcile this copy of the task run and then write back any status
	// updates regardless of whether the reconciliation errored out.
	err = c.reconcile(ctx, tr)
	if err != nil {
		c.Logger.Errorf("Reconcile error: %v", err.Error())
	}
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
	if err := validateTaskRun(c, tr); err != nil {
		c.Logger.Error("Failed to validate taskrun %s with error %v", tr.Name, err)
		tr.Status.SetCondition(&duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  ReasonFailedValidation,
			Message: err.Error(),
		})
		return nil
	}

	// get build the same as the taskrun, this is the value we use for 1:1 mapping and retrieval
	build, err := c.BuildClientSet.BuildV1alpha1().Builds(tr.Namespace).Get(tr.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		pvc, err := c.KubeClientSet.CoreV1().PersistentVolumeClaims(tr.Namespace).Get(tr.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			// Create a persistent volume claim to hold Build logs
			pvc, err = c.createPVC(tr)
			if err != nil {
				return fmt.Errorf("Failed to create persistent volume claim %s for task %q: %v", tr.Name, err, tr.Name)
			}
		} else if err != nil {
			c.Logger.Errorf("Failed to reconcile taskrun: %q, failed to get pvc %q: %v", tr.Name, tr.Name, err)
			return err
		}

		// Build is not present, create build
		build, err = c.createBuild(tr, pvc.Name)
		if err != nil {
			// This Run has failed, so we need to mark it as failed and stop reconciling it
			tr.Status.SetCondition(&duckv1alpha1.Condition{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionFalse,
				Reason: ReasonCouldntGetTask,
				Message: fmt.Sprintf("References a Task %s that doesn't exist: %v",
					fmt.Sprintf("%s/%s", tr.Namespace, tr.Spec.TaskRef.Name), err),
			})
			c.Recorder.Eventf(tr, corev1.EventTypeWarning, "BuildCreationFailed", "Failed to create build %q: %v", tr.Name, err)
			c.Logger.Errorf("Failed to create build for task %q :%v", err, tr.Name)
			return nil
		}
	} else if err != nil {
		c.Logger.Errorf("Failed to reconcile taskrun: %q, failed to get build %q; %v", tr.Name, tr.Name, err)
		return err
	}
	if err := c.tracker.Track(tr.GetBuildRef(), tr); err != nil {
		c.Logger.Errorf("Failed to create tracker for build %q for taskrun %q: %v", tr.Name, tr.Name, err)
		return err
	}

	// switch ownerref := metav1.GetControllerOf(b); {
	// case ownerref == nil, ownerref.APIVersion != groupVersionKind.GroupVersion().String(), ownerref.Kind != groupVersionKind.Kind:
	// 	logger.Infof("build %s not controlled by taskrun controller", b.Name)
	// 	return nil
	// }

	before := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
	// sync build status with taskrun status
	if build.Status.GetCondition(duckv1alpha1.ConditionSucceeded) != nil {
		tr.Status.SetCondition(build.Status.GetCondition(duckv1alpha1.ConditionSucceeded))
	} else {
		// If the Build status doesn't exist yet, it's because we just started running
		tr.Status.SetCondition(&duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  ReasonRunning,
			Message: ReasonRunning,
		})
	}

	after := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
	reconciler.EmitEvent(c.Recorder, before, after, tr)

	c.Logger.Infof("Successfully reconciled taskrun %s/%s with status: %#v", tr.Name, tr.Namespace,
		after)

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

// createVolume will create a persistent volume mount for tr which
// will be used to gather logs using the entrypoint wrapper
func (c *Reconciler) createPVC(tr *v1alpha1.TaskRun) (*corev1.PersistentVolumeClaim, error) {
	v, err := c.KubeClientSet.CoreV1().PersistentVolumeClaims(tr.Namespace).Create(
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: tr.Namespace,
				// This pvc is specific to this TaskRun, so we'll use the same name
				Name: tr.Name,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(tr, groupVersionKind),
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: *resource.NewQuantity(pvcSizeBytes, resource.BinarySI),
					},
				},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to claim Persistent Volume %q due to error: %s", tr.Name, err)
	}
	return v, nil
}

// createBuild creates a build from the task, using the task's buildspec
// with pvcName as a volumeMount
func (c *Reconciler) createBuild(tr *v1alpha1.TaskRun, pvcName string) (*buildv1alpha1.Build, error) {
	// Get related task for taskrun
	t, err := c.taskLister.Tasks(tr.Namespace).Get(tr.Spec.TaskRef.Name)
	if err != nil {
		return nil, fmt.Errorf("error when listing tasks %v", err)
	}

	// TODO: Preferably use Validate on task.spec to catch validation error
	if t.Spec.BuildSpec == nil {
		return nil, fmt.Errorf("task %s has nil BuildSpec", t.Name)
	}

	// For each step with no entrypoint set, try to populate it with the info
	// from the remote registry
	cache := entrypoint.NewCache()
	bSpec := t.Spec.BuildSpec.DeepCopy()
	for i := range bSpec.Steps {
		step := &bSpec.Steps[i]
		if len(step.Command) == 0 {
			ep, err := entrypoint.GetRemoteEntrypoint(cache, step.Image)
			if err != nil {
				return nil, fmt.Errorf("could not get entrypoint from registry for %s: %v", step.Image, err)
			}
			step.Command = ep
		}
	}

	b, err := CreateRedirectedBuild(bSpec, pvcName, tr)
	if err != nil {
		return nil, fmt.Errorf("couldn't create redirected Build: %v", err)
	}

	build, err := resources.AddInputResource(b, t, tr, c.resourceLister, c.Logger)
	if err != nil {
		c.Logger.Errorf("Failed to create a build for taskrun: %s due to input resource error %v", tr.Name, err)
		return nil, err
	}

	var defaults []v1alpha1.TaskParam
	if t.Spec.Inputs != nil {
		defaults = append(defaults, t.Spec.Inputs.Params...)
	}
	// Apply parameter templating from the taskrun.
	build = resources.ApplyParameters(build, tr, defaults...)

	// Apply bound resource templating from the taskrun.
	build, err = resources.ApplyResources(build, tr.Spec.Inputs.Resources, c.resourceLister.PipelineResources(t.Namespace), "inputs")
	if err != nil {
		return nil, fmt.Errorf("couldnt apply input resource templating: %s", err)
	}
	build, err = resources.ApplyResources(build, tr.Spec.Outputs.Resources, c.resourceLister.PipelineResources(t.Namespace), "outputs")
	if err != nil {
		return nil, fmt.Errorf("couldnt apply output resource templating: %s", err)
	}

	return c.BuildClientSet.BuildV1alpha1().Builds(tr.Namespace).Create(build)
}

// CreateRedirectedBuild takes a build, a persistent volume claim name, a taskrun and
// an entrypoint cache creates a build where all entrypoints are switched to
// be the entrypoint redirector binary. This function assumes that it receives
// its own copy of the BuildSpec and modifies it freely
func CreateRedirectedBuild(bs *buildv1alpha1.BuildSpec, pvcName string, tr *v1alpha1.TaskRun) (*buildv1alpha1.Build, error) {
	bs.ServiceAccountName = tr.Spec.ServiceAccount
	// RedirectSteps the entrypoint in each container so that we can use our custom
	// entrypoint which copies logs to the volume
	err := entrypoint.RedirectSteps(bs.Steps)
	if err != nil {
		return nil, fmt.Errorf("failed to add entrypoint to steps of TaskRun %s: %v", tr.Name, err)
	}
	// Add the step which will copy the entrypoint into the volume
	// we are going to be using, so that all of the steps will have
	// access to it.
	entrypoint.AddCopyStep(bs)
	b := &buildv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tr.Name,
			Namespace: tr.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(tr, groupVersionKind),
			},
			// Attach new label and pass taskrun labels to build
			Labels: makeLabels(tr),
		},
		Spec: *bs,
	}
	// Add the volume used for storing the binary and logs
	b.Spec.Volumes = append(b.Spec.Volumes, corev1.Volume{
		Name: entrypoint.MountName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	})
	// Pass service account name from taskrun to build
	// if task specifies service account name override with taskrun SA
	b.Spec.ServiceAccountName = tr.Spec.ServiceAccount

	return b, nil
}

// makeLabels constructs the labels we will apply to TaskRun resources.
func makeLabels(s *v1alpha1.TaskRun) map[string]string {
	labels := make(map[string]string, len(s.ObjectMeta.Labels)+1)
	labels[pipeline.GroupName+pipeline.TaskRunLabelKey] = s.Name
	for k, v := range s.ObjectMeta.Labels {
		labels[k] = v
	}
	return labels

}
