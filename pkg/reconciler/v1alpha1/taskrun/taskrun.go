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
	"strings"
	"time"

	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	informers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1alpha1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
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

	// reasonExceededResourceQuota indicates that the TaskRun failed to create a pod due to
	// a ResourceQuota in the namespace
	reasonExceededResourceQuota = "ExceededResourceQuota"

	// reasonExceededNodeResources indicates that the TaskRun's pod has failed to start due
	// to resource constraints on the node
	reasonExceededNodeResources = "ExceededNodeResources"

	// taskRunAgentName defines logging agent name for TaskRun Controller
	taskRunAgentName = "taskrun-controller"
	// taskRunControllerName defines name for TaskRun Controller
	taskRunControllerName = "TaskRun"

	// imageDigestExporterContainerName defines the name of the container that will collect the
	// built images digest
	imageDigestExporterContainerName = "build-step-image-digest-exporter"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	taskRunLister     listers.TaskRunLister
	taskLister        listers.TaskLister
	clusterTaskLister listers.ClusterTaskLister
	resourceLister    listers.PipelineResourceLister
	tracker           tracker.Interface
	cache             *entrypoint.Cache
	timeoutHandler    *reconciler.TimeoutSet
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
	entrypointCache *entrypoint.Cache,
	timeoutHandler *reconciler.TimeoutSet,
) *controller.Impl {

	c := &Reconciler{
		Base:              reconciler.NewBase(opt, taskRunAgentName),
		taskRunLister:     taskRunInformer.Lister(),
		taskLister:        taskInformer.Lister(),
		clusterTaskLister: clusterTaskInformer.Lister(),
		resourceLister:    resourceInformer.Lister(),
		timeoutHandler:    timeoutHandler,
	}
	impl := controller.NewImpl(c, c.Logger, taskRunControllerName, reconciler.MustNewStatsReporter(taskRunControllerName, c.Logger))

	c.Logger.Info("Setting up event handlers")
	taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
	})

	c.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.EnqueueControllerOf,
		UpdateFunc: controller.PassNew(impl.EnqueueControllerOf),
		DeleteFunc: impl.EnqueueControllerOf,
	})

	c.Logger.Info("Setting up Entrypoint cache")
	c.cache = entrypointCache
	if c.cache == nil {
		c.cache, _ = entrypoint.NewCache()
	}

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

	// If the TaskRun is just starting, this will also set the starttime,
	// from which the timeout will immediately begin counting down.
	tr.Status.InitializeConditions()
	// In case node time was not synchronized, when controller has been scheduled to other nodes.
	if tr.Status.StartTime.Sub(tr.CreationTimestamp.Time) < 0 {
		c.Logger.Warnf("TaskRun %s createTimestamp %s is after the taskRun started %s", tr.GetRunKey(), tr.CreationTimestamp, tr.Status.StartTime)
		tr.Status.StartTime = &tr.CreationTimestamp
	}

	if tr.IsDone() {
		c.timeoutHandler.Release(tr)
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
	// the status and labels/annotations simultaneously.
	if !reflect.DeepEqual(original.ObjectMeta.Labels, tr.ObjectMeta.Labels) {
		if _, err := c.updateLabelsAndAnnotations(tr); err != nil {
			c.Logger.Warn("Failed to update TaskRun labels/annotations", zap.Error(err))
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
	if tr.IsCancelled() {
		before := tr.Status.GetCondition(apis.ConditionSucceeded)
		err := cancelTaskRun(tr, c.KubeClientSet, c.Logger)
		after := tr.Status.GetCondition(apis.ConditionSucceeded)
		reconciler.EmitEvent(c.Recorder, before, after, tr)
		return err
	}

	getTaskFunc := c.getTaskFunc(tr)
	taskMeta, taskSpec, err := resources.GetTaskData(tr, getTaskFunc)
	if err != nil {
		c.Logger.Errorf("Failed to determine Task spec to use for taskrun %s: %v", tr.Name, err)
		tr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
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

	// Propagate annotations from Task to TaskRun.
	if tr.ObjectMeta.Annotations == nil {
		tr.ObjectMeta.Annotations = make(map[string]string, len(taskMeta.Annotations))
	}
	for key, value := range taskMeta.Annotations {
		tr.ObjectMeta.Annotations[key] = value
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
		tr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  reasonFailedResolution,
			Message: err.Error(),
		})
		return nil
	}

	if err := ValidateResolvedTaskResources(tr.Spec.Inputs.Params, rtr); err != nil {
		c.Logger.Errorf("Failed to validate taskrun %q: %v", tr.Name, err)
		tr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  reasonFailedValidation,
			Message: err.Error(),
		})
		return nil
	}

	// Get the TaskRun's Pod if it should have one. Otherwise, create the Pod.
	pod, err := resources.TryGetPod(tr.Status, c.KubeClientSet.CoreV1().Pods(tr.Namespace).Get)
	if err != nil {
		c.Logger.Errorf("Error getting pod %q: %v", tr.Status.PodName, err)
		return err
	}
	if pod == nil {
		// Pod is not present, create pod.
		pod, err = c.createPod(tr, rtr.TaskSpec, rtr.TaskName)
		if err != nil {
			// This Run has failed, so we need to mark it as failed and stop reconciling it
			var reason, msg string
			if isExceededResourceQuotaError(err) {
				reason = reasonExceededResourceQuota
				msg = getExceededResourcesMessage(tr)
			} else {
				reason = reasonCouldntGetTask
				if tr.Spec.TaskRef != nil {
					msg = fmt.Sprintf("Missing or invalid Task %s/%s", tr.Namespace, tr.Spec.TaskRef.Name)
				} else {
					msg = fmt.Sprintf("Invalid TaskSpec")
				}
			}
			tr.Status.SetCondition(&apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  reason,
				Message: fmt.Sprintf("%s: %v", msg, err),
			})
			c.Recorder.Eventf(tr, corev1.EventTypeWarning, "BuildCreationFailed", "Failed to create build pod %q: %v", tr.Name, err)
			c.Logger.Errorf("Failed to create build pod for task %q :%v", err, tr.Name)
			return nil
		}
		go c.timeoutHandler.WaitTaskRun(tr, tr.Status.StartTime)
	}
	if err := c.tracker.Track(tr.GetBuildPodRef(), tr); err != nil {
		c.Logger.Errorf("Failed to create tracker for build pod %q for taskrun %q: %v", tr.Name, tr.Name, err)
		return err
	}

	if isPodExceedingNodeResources(pod) {
		c.Recorder.Eventf(tr, corev1.EventTypeWarning, reasonExceededNodeResources, "Insufficient resources to schedule pod %q", pod.Name)
	}

	before := tr.Status.GetCondition(apis.ConditionSucceeded)

	updateStatusFromPod(tr, pod, c.resourceLister, c.KubeClientSet, c.Logger)

	after := tr.Status.GetCondition(apis.ConditionSucceeded)

	reconciler.EmitEvent(c.Recorder, before, after, tr)

	c.Logger.Infof("Successfully reconciled taskrun %s/%s with status: %#v", tr.Name, tr.Namespace, after)

	return nil
}

func updateStatusFromPod(taskRun *v1alpha1.TaskRun, pod *corev1.Pod, resourceLister listers.PipelineResourceLister, kubeclient kubernetes.Interface, logger *zap.SugaredLogger) {
	if taskRun.Status.GetCondition(apis.ConditionSucceeded) == nil || taskRun.Status.GetCondition(apis.ConditionSucceeded).Status == corev1.ConditionUnknown {
		// If the taskRunStatus doesn't exist yet, it's because we just started running
		taskRun.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  reasonRunning,
			Message: reasonRunning,
		})
	}

	taskRun.Status.PodName = pod.Name

	taskRun.Status.Steps = []v1alpha1.StepState{}
	for _, s := range pod.Status.ContainerStatuses {
		taskRun.Status.Steps = append(taskRun.Status.Steps, v1alpha1.StepState{
			ContainerState: *s.State.DeepCopy(),
			Name:           resources.TrimContainerNamePrefix(s.Name),
		})
	}

	switch pod.Status.Phase {
	case corev1.PodRunning:
		taskRun.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: "Building",
		})
	case corev1.PodFailed:
		msg := getFailureMessage(pod)
		taskRun.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Message: msg,
		})
		// update tr completed time
		taskRun.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	case corev1.PodPending:
		var reason, msg string
		if isPodExceedingNodeResources(pod) {
			reason = reasonExceededNodeResources
			msg = getExceededResourcesMessage(taskRun)
		} else {
			reason = "Pending"
			msg = getWaitingMessage(pod)
		}
		taskRun.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  reason,
			Message: msg,
		})
	case corev1.PodSucceeded:
		taskRun.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		})
		// update tr completed time
		taskRun.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	}

	updateTaskRunResourceResult(taskRun, pod, resourceLister, kubeclient, logger)
}

func updateTaskRunResourceResult(taskRun *v1alpha1.TaskRun, pod *corev1.Pod, resourceLister listers.PipelineResourceLister, kubeclient kubernetes.Interface, logger *zap.SugaredLogger) {
	if resources.TaskRunHasOutputImageResource(resourceLister.PipelineResources(taskRun.Namespace).Get, taskRun) && taskRun.IsSuccessful() {
		for _, container := range pod.Spec.Containers {
			if strings.HasPrefix(container.Name, imageDigestExporterContainerName) {
				req := kubeclient.CoreV1().Pods(taskRun.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name})
				logContent, err := req.Do().Raw()
				if err != nil {
					logger.Errorf("Error getting output from image-digest-exporter for %s/%s: %s", taskRun.Name, taskRun.Namespace, err)
				}
				err = resources.UpdateTaskRunStatusWithResourceResult(taskRun, logContent)
				if err != nil {
					logger.Errorf("Error getting output from image-digest-exporter for %s/%s: %s", taskRun.Name, taskRun.Namespace, err)
				}
			}
		}
	}
}

func getWaitingMessage(pod *corev1.Pod) string {
	// First, try to surface reason for pending/unknown about the actual build step.
	for _, status := range pod.Status.ContainerStatuses {
		wait := status.State.Waiting
		if wait != nil && wait.Message != "" {
			return fmt.Sprintf("build step %q is pending with reason %q",
				status.Name, wait.Message)
		}
	}
	// Try to surface underlying reason by inspecting pod's recent status if condition is not true
	for i, podStatus := range pod.Status.Conditions {
		if podStatus.Status != corev1.ConditionTrue {
			return fmt.Sprintf("pod status %q:%q; message: %q",
				pod.Status.Conditions[i].Type,
				pod.Status.Conditions[i].Status,
				pod.Status.Conditions[i].Message)
		}
	}
	// Next, return the Pod's status message if it has one.
	if pod.Status.Message != "" {
		return pod.Status.Message
	}

	// Lastly fall back on a generic pending message.
	return "Pending"
}

func getFailureMessage(pod *corev1.Pod) string {
	// First, try to surface an error about the actual build step that failed.
	for _, status := range pod.Status.ContainerStatuses {
		term := status.State.Terminated
		if term != nil && term.ExitCode != 0 {
			return fmt.Sprintf("%q exited with code %d (image: %q); for logs run: kubectl -n %s logs %s -c %s",
				status.Name, term.ExitCode, status.ImageID,
				pod.Namespace, pod.Name, status.Name)
		}
	}
	// Next, return the Pod's status message if it has one.
	if pod.Status.Message != "" {
		return pod.Status.Message
	}
	// Lastly fall back on a generic error message.
	return "build failed for unspecified reasons."
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

func (c *Reconciler) updateLabelsAndAnnotations(tr *v1alpha1.TaskRun) (*v1alpha1.TaskRun, error) {
	newTr, err := c.taskRunLister.TaskRuns(tr.Namespace).Get(tr.Name)
	if err != nil {
		return nil, fmt.Errorf("Error getting TaskRun %s when updating labels/annotations: %s", tr.Name, err)
	}
	if !reflect.DeepEqual(tr.ObjectMeta.Labels, newTr.ObjectMeta.Labels) || !reflect.DeepEqual(tr.ObjectMeta.Annotations, newTr.ObjectMeta.Annotations) {
		newTr.ObjectMeta.Labels = tr.ObjectMeta.Labels
		newTr.ObjectMeta.Annotations = tr.ObjectMeta.Annotations
		return c.PipelineClientSet.TektonV1alpha1().TaskRuns(tr.Namespace).Update(newTr)
	}
	return newTr, nil
}

// createPod creates a Pod based on the Task's configuration, with pvcName as a
// volumeMount
func (c *Reconciler) createPod(tr *v1alpha1.TaskRun, ts *v1alpha1.TaskSpec, taskName string) (*corev1.Pod, error) {
	ts = ts.DeepCopy()

	err := resources.AddOutputImageDigestExporter(tr, ts, c.resourceLister.PipelineResources(tr.Namespace).Get)
	if err != nil {
		c.Logger.Errorf("Failed to create a build for taskrun: %s due to output image resource error %v", tr.Name, err)
		return nil, err
	}

	ts, err = resources.AddInputResource(c.KubeClientSet, taskName, ts, tr, c.resourceLister, c.Logger)
	if err != nil {
		c.Logger.Errorf("Failed to create a build for taskrun: %s due to input resource error %v", tr.Name, err)
		return nil, err
	}

	err = resources.AddOutputResources(c.KubeClientSet, taskName, ts, tr, c.resourceLister, c.Logger)
	if err != nil {
		c.Logger.Errorf("Failed to create a build for taskrun: %s due to output resource error %v", tr.Name, err)
		return nil, err
	}

	ts, err = createRedirectedTaskSpec(c.KubeClientSet, ts, tr, c.cache, c.Logger)
	if err != nil {
		return nil, fmt.Errorf("couldn't create redirected TaskSpec: %v", err)
	}

	var defaults []v1alpha1.TaskParam
	if ts.Inputs != nil {
		defaults = append(defaults, ts.Inputs.Params...)
	}
	// Apply parameter templating from the taskrun.
	ts = resources.ApplyParameters(ts, tr, defaults...)

	// Apply bound resource templating from the taskrun.
	ts, err = resources.ApplyResources(ts, tr.Spec.Inputs.Resources, c.resourceLister.PipelineResources(tr.Namespace).Get, "inputs")
	if err != nil {
		return nil, fmt.Errorf("couldnt apply input resource templating: %s", err)
	}
	ts, err = resources.ApplyResources(ts, tr.Spec.Outputs.Resources, c.resourceLister.PipelineResources(tr.Namespace).Get, "outputs")
	if err != nil {
		return nil, fmt.Errorf("couldnt apply output resource templating: %s", err)
	}

	pod, err := resources.MakePod(tr, *ts, c.KubeClientSet, c.cache, c.Logger)
	if err != nil {
		return nil, fmt.Errorf("translating Build to Pod: %v", err)
	}

	return c.KubeClientSet.CoreV1().Pods(tr.Namespace).Create(pod)
}

// CreateRedirectedTaskSpec takes a TaskSpec, a persistent volume claim name, a taskrun and
// an entrypoint cache creates a build where all entrypoints are switched to
// be the entrypoint redirector binary. This function assumes that it receives
// its own copy of the TaskSpec and modifies it freely
func createRedirectedTaskSpec(kubeclient kubernetes.Interface, ts *v1alpha1.TaskSpec, tr *v1alpha1.TaskRun, cache *entrypoint.Cache, logger *zap.SugaredLogger) (*v1alpha1.TaskSpec, error) {
	// RedirectSteps the entrypoint in each container so that we can use our custom
	// entrypoint which copies logs to the volume
	err := entrypoint.RedirectSteps(cache, ts.Steps, kubeclient, tr, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to add entrypoint to steps of TaskRun %s: %v", tr.Name, err)
	}
	// Add the step which will copy the entrypoint into the volume
	// we are going to be using, so that all of the steps will have
	// access to it.
	entrypoint.AddCopyStep(ts)

	// Add the volume used for storing the binary and logs
	ts.Volumes = append(ts.Volumes, corev1.Volume{
		Name: entrypoint.MountName,
		VolumeSource: corev1.VolumeSource{
			// TODO(#107) we need to actually stream these logs somewhere, probably via sidecar.
			// Currently these logs will be lost when the pod is unscheduled.
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	return ts, nil
}

type DeletePod func(podName string, options *metav1.DeleteOptions) error

func (c *Reconciler) checkTimeout(tr *v1alpha1.TaskRun, ts *v1alpha1.TaskSpec, dp DeletePod) (bool, error) {
	// If tr has not started, startTime should be zero.
	if tr.Status.StartTime.IsZero() {
		return false, nil
	}

	timeout := reconciler.GetTimeout(tr.Spec.Timeout)
	runtime := time.Since(tr.Status.StartTime.Time)
	c.Logger.Infof("Checking timeout for TaskRun %q (startTime %s, timeout %s, runtime %s)", tr.Name, tr.Status.StartTime, timeout, runtime)
	if runtime > timeout {
		c.Logger.Infof("TaskRun %q is timeout (runtime %s over %s), deleting pod", tr.Name, runtime, timeout)
		if err := dp(tr.Status.PodName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			c.Logger.Errorf("Failed to terminate pod: %v", err)
			return true, err
		}

		timeoutMsg := fmt.Sprintf("TaskRun %q failed to finish within %q", tr.Name, timeout.String())
		tr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  reasonTimedOut,
			Message: timeoutMsg,
		})
		// update tr completed time
		tr.Status.CompletionTime = &metav1.Time{Time: time.Now()}

		return true, nil
	}
	return false, nil
}

func isPodExceedingNodeResources(pod *corev1.Pod) bool {
	for _, podStatus := range pod.Status.Conditions {
		if podStatus.Reason == corev1.PodReasonUnschedulable && strings.Contains(podStatus.Message, "Insufficient") {
			return true
		}
	}
	return false
}

func isExceededResourceQuotaError(err error) bool {
	return err != nil && errors.IsForbidden(err) && strings.Contains(err.Error(), "exceeded quota")
}

func getExceededResourcesMessage(tr *v1alpha1.TaskRun) string {
	return fmt.Sprintf("TaskRun pod %q exceeded available resources", tr.Name)
}
