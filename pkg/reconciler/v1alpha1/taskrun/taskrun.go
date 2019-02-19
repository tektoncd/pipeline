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
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	informers "github.com/knative/build-pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	listers "github.com/knative/build-pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/config"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// reasonCouldntGetTask indicates that the reason for the failure status is that the
	// Task couldn't be found
	reasonCouldntGetTask = "CouldntGetTask"

	// reasonFailedResolution indicated that the reason for failure status is
	// that references within the TaskRun could not be resolved
	reasonFailedResolution = "TaskRunResolutionFailed"

	// reasonFailedValidation indicated that the reason for failure status is
	// that taskrun failed runtime validation
	reasonFailedValidation = "TaskRunValidationFailed"

	// reasonRunning indicates that the reason for the inprogress status is that the TaskRun
	// is just starting to be reconciled
	reasonRunning = "Running"

	// reasonTimedOut indicates that the TaskRun has taken longer than its configured timeout
	reasonTimedOut = "TaskRunTimeout"

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

type configStore interface {
	ToContext(ctx context.Context) context.Context
	WatchConfigs(w configmap.Watcher)
}

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	taskRunLister     listers.TaskRunLister
	taskLister        listers.TaskLister
	clusterTaskLister listers.ClusterTaskLister
	resourceLister    listers.PipelineResourceLister
	tracker           tracker.Interface
	configStore       configStore
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController creates a new Configuration controller
func NewController(
	opt reconciler.Options,
	taskRunInformer informers.TaskRunInformer,
	taskInformer informers.TaskInformer,
	clusterTaskInformer informers.ClusterTaskInformer,
	resourceInformer informers.PipelineResourceInformer,
	podInformer coreinformers.PodInformer,
) *controller.Impl {

	c := &Reconciler{
		Base:              reconciler.NewBase(opt, taskRunAgentName),
		taskRunLister:     taskRunInformer.Lister(),
		taskLister:        taskInformer.Lister(),
		clusterTaskLister: clusterTaskInformer.Lister(),
		resourceLister:    resourceInformer.Lister(),
	}
	impl := controller.NewImpl(c, c.Logger, taskRunControllerName, reconciler.MustNewStatsReporter(taskRunControllerName, c.Logger))

	c.Logger.Info("Setting up event handlers")
	taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
	})

	c.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.tracker.OnChanged,
		UpdateFunc: controller.PassNew(c.tracker.OnChanged),
	})

	c.Logger.Info("Setting up ConfigMap receivers")
	c.configStore = config.NewStore(c.Logger.Named("config-store"))
	c.configStore.WatchConfigs(opt.ConfigMapWatcher)
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

	ctx = c.configStore.ToContext(ctx)

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

	if isDone(&tr.Status) {
		return nil
	}

	// Reconcile this copy of the task run and then write back any status
	// updates regardless of whether the reconciliation errored out.
	if err := c.reconcile(ctx, tr); err != nil {
		c.Logger.Errorf("Reconcile error: %v", err.Error())
		return err
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
	// Since we are using the status subresource, it is not possible to update
	// the status and labels simultaneously.
	if !reflect.DeepEqual(original.ObjectMeta.Labels, tr.ObjectMeta.Labels) {
		if _, err := c.updateLabels(tr); err != nil {
			c.Logger.Warn("Failed to update TaskRun labels", zap.Error(err))
			return err
		}
	}

	return err
}

func (c *Reconciler) getTaskFunc(tr *v1alpha1.TaskRun) resources.GetTask {
	var gtFunc resources.GetTask
	if tr.Spec.TaskRef != nil && tr.Spec.TaskRef.Kind == v1alpha1.ClusterTaskKind {
		gtFunc = func(name string) (v1alpha1.TaskInterface, error) {
			t, err := c.clusterTaskLister.Get(name)
			if err != nil {
				return nil, err
			}
			return t, nil
		}
	} else {
		gtFunc = func(name string) (v1alpha1.TaskInterface, error) {
			t, err := c.taskLister.Tasks(tr.Namespace).Get(name)
			if err != nil {
				return nil, err
			}
			return t, nil
		}
	}
	return gtFunc
}

func (c *Reconciler) reconcile(ctx context.Context, tr *v1alpha1.TaskRun) error {
	// If the taskrun is cancelled, kill resources and update status
	if isCancelled(tr.Spec) {
		before := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		err := cancelTaskRun(tr, c.KubeClientSet, c.Logger)
		after := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
		reconciler.EmitEvent(c.Recorder, before, after, tr)
		return err
	}

	getTaskFunc := c.getTaskFunc(tr)
	taskMeta, taskSpec, err := resources.GetTaskData(tr, getTaskFunc)
	if err != nil {
		c.Logger.Errorf("Failed to determine Task spec to use for taskrun %s: %v", tr.Name, err)
		tr.Status.SetCondition(&duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  reasonFailedResolution,
			Message: err.Error(),
		})
		return nil
	}

	// Propagate labels from Task to TaskRun.
	if tr.ObjectMeta.Labels == nil {
		tr.ObjectMeta.Labels = make(map[string]string, len(taskMeta.Labels)+1)
	}
	for key, value := range taskMeta.Labels {
		tr.ObjectMeta.Labels[key] = value
	}
	if tr.Spec.TaskRef != nil {
		tr.ObjectMeta.Labels[pipeline.GroupName+pipeline.TaskLabelKey] = taskMeta.Name
	}

	// Check if the TaskRun has timed out; if it is, this will set its status
	// accordingly.
	if timedOut, err := c.checkTimeout(tr, taskSpec, c.KubeClientSet.CoreV1().Pods(tr.Namespace).Delete); err != nil {
		return err
	} else if timedOut {
		return nil
	}

	rtr, err := resources.ResolveTaskResources(taskSpec, taskMeta.Name, tr.Spec.Inputs.Resources, tr.Spec.Outputs.Resources, c.resourceLister.PipelineResources(tr.Namespace).Get)
	if err != nil {
		c.Logger.Errorf("Failed to resolve references for taskrun %s: %v", tr.Name, err)
		tr.Status.SetCondition(&duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  reasonFailedResolution,
			Message: err.Error(),
		})
		return nil
	}

	if err := ValidateResolvedTaskResources(tr.Spec.Inputs.Params, rtr); err != nil {
		c.Logger.Errorf("Failed to validate taskrun %q: %v", tr.Name, err)
		tr.Status.SetCondition(&duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  reasonFailedValidation,
			Message: err.Error(),
		})
		return nil
	}

	// Get the TaskRun's Pod if it should have one. Otherwise, create the Pod.
	var pod *corev1.Pod
	if tr.Status.PodName != "" {
		pod, err = c.KubeClientSet.CoreV1().Pods(tr.Namespace).Get(tr.Status.PodName, metav1.GetOptions{})
		if err != nil {
			c.Logger.Errorf("Error getting pod %q: %v", tr.Status.PodName, err)
			return err
		}
	} else {
		// Build pod is not present, create build pod.
		pod, err = c.createBuildPod(ctx, tr, rtr.TaskSpec, rtr.TaskName)
		if err != nil {
			// This Run has failed, so we need to mark it as failed and stop reconciling it
			var msg string
			if tr.Spec.TaskRef != nil {
				msg = fmt.Sprintf("References a Task %s that doesn't exist: ", fmt.Sprintf("%s/%s", tr.Namespace, tr.Spec.TaskRef.Name))
			} else {
				msg = fmt.Sprintf("References a TaskSpec with missing information: ")
			}
			tr.Status.SetCondition(&duckv1alpha1.Condition{
				Type:    duckv1alpha1.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  reasonCouldntGetTask,
				Message: fmt.Sprintf("%s %v", msg, err),
			})
			c.Recorder.Eventf(tr, corev1.EventTypeWarning, "BuildCreationFailed", "Failed to create build pod %q: %v", tr.Name, err)
			c.Logger.Errorf("Failed to create build pod for task %q :%v", err, tr.Name)
			return nil
		}
	}
	if err := c.tracker.Track(tr.GetBuildPodRef(), tr); err != nil {
		c.Logger.Errorf("Failed to create tracker for build pod %q for taskrun %q: %v", tr.Name, tr.Name, err)
		return err
	}

	before := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)

	// Translate Pod -> BuildStatus
	buildStatus := resources.BuildStatusFromPod(pod, buildv1alpha1.BuildSpec{})
	// Translate BuildStatus -> TaskRunStatus
	updateStatusFromBuildStatus(tr, buildStatus)

	after := tr.Status.GetCondition(duckv1alpha1.ConditionSucceeded)

	reconciler.EmitEvent(c.Recorder, before, after, tr)

	c.Logger.Infof("Successfully reconciled taskrun %s/%s with status: %#v", tr.Name, tr.Namespace, after)

	return nil
}

func updateStatusFromBuildStatus(taskRun *v1alpha1.TaskRun, buildStatus buildv1alpha1.BuildStatus) {
	if buildStatus.GetCondition(duckv1alpha1.ConditionSucceeded) != nil {
		taskRun.Status.SetCondition(buildStatus.GetCondition(duckv1alpha1.ConditionSucceeded))
	} else {
		// If the buildStatus doesn't exist yet, it's because we just started running
		taskRun.Status.SetCondition(&duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  reasonRunning,
			Message: reasonRunning,
		})
	}
	taskRun.Status.StartTime = buildStatus.StartTime
	if buildStatus.Cluster != nil {
		taskRun.Status.PodName = buildStatus.Cluster.PodName
	}
	taskRun.Status.CompletionTime = buildStatus.CompletionTime

	taskRun.Status.Steps = []v1alpha1.StepState{}
	for i := 0; i < len(buildStatus.StepStates); i++ {
		state := buildStatus.StepStates[i]
		taskRun.Status.Steps = append(taskRun.Status.Steps, v1alpha1.StepState{
			ContainerState: *state.DeepCopy(),
		})
	}
}

func (c *Reconciler) updateStatus(taskrun *v1alpha1.TaskRun) (*v1alpha1.TaskRun, error) {
	newtaskrun, err := c.taskRunLister.TaskRuns(taskrun.Namespace).Get(taskrun.Name)
	if err != nil {
		return nil, fmt.Errorf("Error getting TaskRun %s when updating status: %s", taskrun.Name, err)
	}
	if !reflect.DeepEqual(taskrun.Status, newtaskrun.Status) {
		newtaskrun.Status = taskrun.Status
		return c.PipelineClientSet.TektonV1alpha1().TaskRuns(taskrun.Namespace).UpdateStatus(newtaskrun)
	}
	return newtaskrun, nil
}

func (c *Reconciler) updateLabels(tr *v1alpha1.TaskRun) (*v1alpha1.TaskRun, error) {
	newTr, err := c.taskRunLister.TaskRuns(tr.Namespace).Get(tr.Name)
	if err != nil {
		return nil, fmt.Errorf("Error getting TaskRun %s when updating labels: %s", tr.Name, err)
	}
	if !reflect.DeepEqual(tr.ObjectMeta.Labels, newTr.ObjectMeta.Labels) {
		newTr.ObjectMeta.Labels = tr.ObjectMeta.Labels
		return c.PipelineClientSet.TektonV1alpha1().TaskRuns(tr.Namespace).Update(newTr)
	}
	return newTr, nil
}

// createPod creates a Pod based on the Task's configuration, with pvcName as a
// volumeMount
func (c *Reconciler) createBuildPod(ctx context.Context, tr *v1alpha1.TaskRun, ts *v1alpha1.TaskSpec, taskName string) (*corev1.Pod, error) {
	// TODO: Preferably use Validate on task.spec to catch validation error
	bs := ts.GetBuildSpec()
	if bs == nil {
		return nil, fmt.Errorf("task %s has nil BuildSpec", taskName)
	}

	// For each step with no entrypoint set, try to populate it with the info
	// from the remote registry
	entrypointCache, err := entrypoint.NewCache()
	if err != nil {
		return nil, fmt.Errorf("couldn't create new entrypoint cache: %v", err)
	}
	digestCache, err := entrypoint.NewCache()
	if err != nil {
		return nil, fmt.Errorf("couldn't create new digest cache: %v", err)
	}
	bSpec := bs.DeepCopy()
	for i := range bSpec.Steps {
		step := &bSpec.Steps[i]
		if len(step.Command) == 0 {
			digest, err := entrypoint.GetImageDigest(digestCache, step.Image)
			if err != nil {
				return nil, fmt.Errorf("could not get digest for %s: %v", step.Image, err)
			}
			ep, err := entrypoint.GetRemoteEntrypoint(entrypointCache, digest)
			if err != nil {
				return nil, fmt.Errorf("could not get entrypoint from registry for %s: %v", step.Image, err)
			}
			step.Command = ep
		}
	}
	bSpec.Timeout = tr.Spec.Timeout
	bSpec.Affinity = tr.Spec.Affinity
	bSpec.NodeSelector = tr.Spec.NodeSelector

	build, err := createRedirectedBuild(ctx, bSpec, tr)
	if err != nil {
		return nil, fmt.Errorf("couldn't create redirected Build: %v", err)
	}

	build, err = resources.AddInputResource(c.KubeClientSet, build, taskName, ts, tr, c.resourceLister, c.Logger)
	if err != nil {
		c.Logger.Errorf("Failed to create a build for taskrun: %s due to input resource error %v", tr.Name, err)
		return nil, err
	}

	err = resources.AddOutputResources(c.KubeClientSet, build, taskName, ts, tr, c.resourceLister, c.Logger)
	if err != nil {
		c.Logger.Errorf("Failed to create a build for taskrun: %s due to output resource error %v", tr.Name, err)
		return nil, err
	}

	var defaults []v1alpha1.TaskParam
	if ts.Inputs != nil {
		defaults = append(defaults, ts.Inputs.Params...)
	}
	// Apply parameter templating from the taskrun.
	build = resources.ApplyParameters(build, tr, defaults...)

	// Apply bound resource templating from the taskrun.
	build, err = resources.ApplyResources(build, tr.Spec.Inputs.Resources, c.resourceLister.PipelineResources(tr.Namespace).Get, "inputs")
	if err != nil {
		return nil, fmt.Errorf("couldnt apply input resource templating: %s", err)
	}
	build, err = resources.ApplyResources(build, tr.Spec.Outputs.Resources, c.resourceLister.PipelineResources(tr.Namespace).Get, "outputs")
	if err != nil {
		return nil, fmt.Errorf("couldnt apply output resource templating: %s", err)
	}

	pod, err := resources.MakePod(build, c.KubeClientSet)
	if err != nil {
		return nil, fmt.Errorf("translating Build to Pod: %v", err)
	}

	return c.KubeClientSet.CoreV1().Pods(tr.Namespace).Create(pod)
}

// CreateRedirectedBuild takes a build, a persistent volume claim name, a taskrun and
// an entrypoint cache creates a build where all entrypoints are switched to
// be the entrypoint redirector binary. This function assumes that it receives
// its own copy of the BuildSpec and modifies it freely
func createRedirectedBuild(ctx context.Context, bs *buildv1alpha1.BuildSpec, tr *v1alpha1.TaskRun) (*buildv1alpha1.Build, error) {
	// Pass service account name from taskrun to build
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
	entrypoint.AddCopyStep(ctx, bs)
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
			// TODO(#107) we need to actually stream these logs somewhere, probably via sidecar.
			// Currently these logs will be lost when the pod is unscheduled.
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	return b, nil
}

// makeLabels constructs the labels we will propagate from TaskRuns to Pods.
func makeLabels(s *v1alpha1.TaskRun) map[string]string {
	labels := make(map[string]string, len(s.ObjectMeta.Labels)+1)
	for k, v := range s.ObjectMeta.Labels {
		labels[k] = v
	}
	labels[pipeline.GroupName+pipeline.TaskRunLabelKey] = s.Name
	return labels
}

// isDone returns true if the TaskRun's status indicates that it is done.
func isDone(status *v1alpha1.TaskRunStatus) bool {
	return !status.GetCondition(duckv1alpha1.ConditionSucceeded).IsUnknown()
}

type DeletePod func(podName string, options *metav1.DeleteOptions) error

func (c *Reconciler) checkTimeout(tr *v1alpha1.TaskRun, ts *v1alpha1.TaskSpec, dp DeletePod) (bool, error) {
	// If tr has not started, startTime should be zero.
	if tr.Status.StartTime.IsZero() {
		return false, nil
	}

	if tr.Spec.Timeout != nil {
		timeout := tr.Spec.Timeout.Duration
		runtime := time.Since(tr.Status.StartTime.Time)
		if runtime > timeout {
			c.Logger.Infof("TaskRun %q is timeout (runtime %s over %s), deleting pod", tr.Name, runtime, timeout)
			if err := dp(tr.Status.PodName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				c.Logger.Errorf("Failed to terminate pod: %v", err)
				return true, err
			}

			timeoutMsg := fmt.Sprintf("TaskRun %q failed to finish within %q", tr.Name, timeout.String())
			tr.Status.SetCondition(&duckv1alpha1.Condition{
				Type:    duckv1alpha1.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  reasonTimedOut,
				Message: timeoutMsg,
			})
			// update tr completed time
			tr.Status.CompletionTime = &metav1.Time{Time: time.Now()}

			return true, nil
		}
	}
	return false, nil
}
