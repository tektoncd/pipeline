/*
Copyright 2019 The Tekton Authors

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
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/sidecars"
	"github.com/tektoncd/pipeline/pkg/status"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	// taskRunAgentName defines logging agent name for TaskRun Controller
	taskRunAgentName = "taskrun-controller"
	// taskRunControllerName defines name for TaskRun Controller
	taskRunControllerName = "TaskRun"

	// imageDigestExporterContainerName defines the name of the container that will collect the
	// built images digest
	imageDigestExporterContainerName = "step-image-digest-exporter"
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
	impl := controller.NewImpl(c, c.Logger, taskRunControllerName)

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
		pod, err := c.KubeClientSet.CoreV1().Pods(tr.Namespace).Get(tr.Status.PodName, metav1.GetOptions{})
		if err == nil {
			err = sidecars.Stop(pod, c.KubeClientSet.CoreV1().Pods(tr.Namespace).Update)
		} else if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			c.Logger.Errorf("Error stopping sidecars for TaskRun %q: %v", name, err)
		}
		return err
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

func (c *Reconciler) getTaskFunc(tr *v1alpha1.TaskRun) (resources.GetTask, v1alpha1.TaskKind) {
	var gtFunc resources.GetTask
	kind := v1alpha1.NamespacedTaskKind
	if tr.Spec.TaskRef != nil && tr.Spec.TaskRef.Kind == v1alpha1.ClusterTaskKind {
		gtFunc = func(name string) (v1alpha1.TaskInterface, error) {
			t, err := c.clusterTaskLister.Get(name)
			if err != nil {
				return nil, err
			}
			return t, nil
		}
		kind = v1alpha1.ClusterTaskKind
	} else {
		gtFunc = func(name string) (v1alpha1.TaskInterface, error) {
			t, err := c.taskLister.Tasks(tr.Namespace).Get(name)
			if err != nil {
				return nil, err
			}
			return t, nil
		}
	}
	return gtFunc, kind
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

	getTaskFunc, kind := c.getTaskFunc(tr)
	taskMeta, taskSpec, err := resources.GetTaskData(tr, getTaskFunc)
	if err != nil {
		c.Logger.Errorf("Failed to determine Task spec to use for taskrun %s: %v", tr.Name, err)
		tr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  status.ReasonFailedResolution,
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

	rtr, err := resources.ResolveTaskResources(taskSpec, taskMeta.Name, kind, tr.Spec.Inputs.Resources, tr.Spec.Outputs.Resources, c.resourceLister.PipelineResources(tr.Namespace).Get)
	if err != nil {
		c.Logger.Errorf("Failed to resolve references for taskrun %s: %v", tr.Name, err)
		tr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  status.ReasonFailedResolution,
			Message: err.Error(),
		})
		return nil
	}

	if err := ValidateResolvedTaskResources(tr.Spec.Inputs.Params, rtr); err != nil {
		c.Logger.Errorf("Failed to validate taskrun %q: %v", tr.Name, err)
		tr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  status.ReasonFailedValidation,
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
		pod, err = c.createPod(tr, rtr)
		if err != nil {
			c.handlePodCreationError(tr, err)
			return nil
		}
		go c.timeoutHandler.WaitTaskRun(tr, tr.Status.StartTime)
	}
	if err := c.tracker.Track(tr.GetBuildPodRef(), tr); err != nil {
		c.Logger.Errorf("Failed to create tracker for build pod %q for taskrun %q: %v", tr.Name, tr.Name, err)
		return err
	}

	if status.IsPodExceedingNodeResources(pod) {
		c.Recorder.Eventf(tr, corev1.EventTypeWarning, status.ReasonExceededNodeResources, "Insufficient resources to schedule pod %q", pod.Name)
	}

	before := tr.Status.GetCondition(apis.ConditionSucceeded)

	addReady := status.UpdateStatusFromPod(tr, pod, c.resourceLister, c.KubeClientSet, c.Logger)

	status.SortTaskRunStepOrder(tr.Status.Steps, taskSpec.Steps)

	updateTaskRunResourceResult(tr, pod, c.resourceLister, c.KubeClientSet, c.Logger)

	after := tr.Status.GetCondition(apis.ConditionSucceeded)

	if addReady {
		if err := c.updateReady(pod); err != nil {
			return err
		}
	}

	reconciler.EmitEvent(c.Recorder, before, after, tr)

	c.Logger.Infof("Successfully reconciled taskrun %s/%s with status: %#v", tr.Name, tr.Namespace, after)

	return nil
}

func (c *Reconciler) handlePodCreationError(tr *v1alpha1.TaskRun, err error) {
	var reason, msg string
	var succeededStatus corev1.ConditionStatus
	if isExceededResourceQuotaError(err) {
		succeededStatus = corev1.ConditionUnknown
		reason = status.ReasonExceededResourceQuota
		backoff, currentlyBackingOff := c.timeoutHandler.GetBackoff(tr)
		if !currentlyBackingOff {
			go c.timeoutHandler.SetTaskRunTimer(tr, time.Until(backoff.NextAttempt))
		}
		msg = fmt.Sprintf("%s, reattempted %d times", status.GetExceededResourcesMessage(tr), backoff.NumAttempts)
	} else {
		succeededStatus = corev1.ConditionFalse
		reason = status.ReasonCouldntGetTask
		if tr.Spec.TaskRef != nil {
			msg = fmt.Sprintf("Missing or invalid Task %s/%s", tr.Namespace, tr.Spec.TaskRef.Name)
		} else {
			msg = fmt.Sprintf("Invalid TaskSpec")
		}
	}
	tr.Status.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  succeededStatus,
		Reason:  reason,
		Message: fmt.Sprintf("%s: %v", msg, err),
	})
	c.Recorder.Eventf(tr, corev1.EventTypeWarning, "BuildCreationFailed", "Failed to create build pod %q: %v", tr.Name, err)
	c.Logger.Errorf("Failed to create build pod for task %q: %v", tr.Name, err)
}

func updateTaskRunResourceResult(taskRun *v1alpha1.TaskRun, pod *corev1.Pod, resourceLister listers.PipelineResourceLister, kubeclient kubernetes.Interface, logger *zap.SugaredLogger) {
	if resources.TaskRunHasOutputImageResource(resourceLister.PipelineResources(taskRun.Namespace).Get, taskRun) && taskRun.IsSuccessful() {
		for _, cs := range pod.Status.ContainerStatuses {
			if strings.HasPrefix(cs.Name, imageDigestExporterContainerName) {
				err := resources.UpdateTaskRunStatusWithResourceResult(taskRun, []byte(cs.State.Terminated.Message))
				if err != nil {
					logger.Errorf("Error getting output from image-digest-exporter for %s/%s: %s", taskRun.Name, taskRun.Namespace, err)
				}
			}
		}
	}
}

func (c *Reconciler) updateStatus(taskrun *v1alpha1.TaskRun) (*v1alpha1.TaskRun, error) {
	newtaskrun, err := c.taskRunLister.TaskRuns(taskrun.Namespace).Get(taskrun.Name)
	if err != nil {
		return nil, xerrors.Errorf("Error getting TaskRun %s when updating status: %w", taskrun.Name, err)
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
		return nil, xerrors.Errorf("Error getting TaskRun %s when updating labels/annotations: %w", tr.Name, err)
	}
	if !reflect.DeepEqual(tr.ObjectMeta.Labels, newTr.ObjectMeta.Labels) || !reflect.DeepEqual(tr.ObjectMeta.Annotations, newTr.ObjectMeta.Annotations) {
		newTr.ObjectMeta.Labels = tr.ObjectMeta.Labels
		newTr.ObjectMeta.Annotations = tr.ObjectMeta.Annotations
		return c.PipelineClientSet.TektonV1alpha1().TaskRuns(tr.Namespace).Update(newTr)
	}
	return newTr, nil
}

// updateReady updates a Pod to include the "ready" annotation, which will be projected by
// the Downward API into a volume mounted by the entrypoint container. This will signal to
// the entrypoint that the TaskRun can proceed.
func (c *Reconciler) updateReady(pod *corev1.Pod) error {
	newPod, err := c.KubeClientSet.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		return xerrors.Errorf("Error getting Pod %q when updating ready annotation: %w", pod.Name, err)
	}
	updatePod := c.KubeClientSet.CoreV1().Pods(newPod.Namespace).Update
	if err := resources.AddReadyAnnotation(newPod, updatePod); err != nil {
		c.Logger.Errorf("Failed to update ready annotation for pod %q for taskrun %q: %v", pod.Name, pod.Name, err)
		return xerrors.Errorf("Error adding ready annotation to Pod %q: %w", pod.Name, err)
	}

	return nil
}

// createPod creates a Pod based on the Task's configuration, with pvcName as a volumeMount
// TODO(dibyom): Refactor resource setup/templating logic to its own function in the resources package
func (c *Reconciler) createPod(tr *v1alpha1.TaskRun, rtr *resources.ResolvedTaskResources) (*corev1.Pod, error) {
	ts := rtr.TaskSpec.DeepCopy()
	inputResources, err := resourceImplBinding(rtr.Inputs)
	if err != nil {
		c.Logger.Errorf("Failed to initialize input resources: %v", err)
		return nil, err
	}
	outputResources, err := resourceImplBinding(rtr.Outputs)
	if err != nil {
		c.Logger.Errorf("Failed to initialize output resources: %v", err)
		return nil, err
	}

	// Get actual resource

	err = resources.AddOutputImageDigestExporter(tr, ts, c.resourceLister.PipelineResources(tr.Namespace).Get)
	if err != nil {
		c.Logger.Errorf("Failed to create a build for taskrun: %s due to output image resource error %v", tr.Name, err)
		return nil, err
	}

	ts, err = resources.AddInputResource(c.KubeClientSet, rtr.TaskName, ts, tr, inputResources, c.Logger)
	if err != nil {
		c.Logger.Errorf("Failed to create a build for taskrun: %s due to input resource error %v", tr.Name, err)
		return nil, err
	}

	ts, err = resources.AddOutputResources(c.KubeClientSet, rtr.TaskName, ts, tr, outputResources, c.Logger)
	if err != nil {
		c.Logger.Errorf("Failed to create a build for taskrun: %s due to output resource error %v", tr.Name, err)
		return nil, err
	}

	ts, err = createRedirectedTaskSpec(c.KubeClientSet, ts, tr, c.cache, c.Logger)
	if err != nil {
		return nil, xerrors.Errorf("couldn't create redirected TaskSpec: %w", err)
	}

	var defaults []v1alpha1.ParamSpec
	if ts.Inputs != nil {
		defaults = append(defaults, ts.Inputs.Params...)
	}
	// Apply parameter templating from the taskrun.
	ts = resources.ApplyParameters(ts, tr, defaults...)

	// Apply bound resource templating from the taskrun.
	ts = resources.ApplyResources(ts, inputResources, "inputs")
	ts = resources.ApplyResources(ts, outputResources, "outputs")

	pod, err := resources.MakePod(tr, *ts, c.KubeClientSet, c.cache, c.Logger)
	if err != nil {
		return nil, xerrors.Errorf("translating Build to Pod: %w", err)
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
		return nil, xerrors.Errorf("failed to add entrypoint to steps of TaskRun %s: %w", tr.Name, err)
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

	ts.Volumes = append(ts.Volumes, corev1.Volume{
		Name: entrypoint.DownwardMountName,
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					corev1.DownwardAPIVolumeFile{
						Path: entrypoint.DownwardMountReadyFile,
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: fmt.Sprintf("metadata.annotations['%s']", resources.ReadyAnnotation),
						},
					},
				},
			},
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

	timeout := tr.Spec.Timeout.Duration
	runtime := time.Since(tr.Status.StartTime.Time)
	c.Logger.Infof("Checking timeout for TaskRun %q (startTime %s, timeout %s, runtime %s)", tr.Name, tr.Status.StartTime, timeout, runtime)

	if runtime > timeout {
		c.Logger.Infof("TaskRun %q is timeout (runtime %s over %s), deleting pod", tr.Name, runtime, timeout)
		// tr.Status.PodName will be empty if the pod was never successfully created. This condition
		// can be reached, for example, by the pod never being schedulable due to limits imposed by
		// a namespace's ResourceQuota.
		if tr.Status.PodName != "" {
			if err := dp(tr.Status.PodName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				c.Logger.Errorf("Failed to terminate pod: %v", err)
				return true, err
			}
		}

		timeoutMsg := fmt.Sprintf("TaskRun %q failed to finish within %q", tr.Name, timeout.String())
		tr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  status.ReasonTimedOut,
			Message: timeoutMsg,
		})
		// update tr completed time
		tr.Status.CompletionTime = &metav1.Time{Time: time.Now()}

		return true, nil
	}
	return false, nil
}

func isExceededResourceQuotaError(err error) bool {
	return err != nil && errors.IsForbidden(err) && strings.Contains(err.Error(), "exceeded quota")
}

// resourceImplBinding maps pipeline resource names to the actual resource type implementations
func resourceImplBinding(resources map[string]*v1alpha1.PipelineResource) (map[string]v1alpha1.PipelineResourceInterface, error) {
	p := make(map[string]v1alpha1.PipelineResourceInterface)
	for rName, r := range resources {
		i, err := v1alpha1.ResourceFromType(r)
		if err != nil {
			return nil, xerrors.Errorf("failed to create resource %s : %v with error: %w", rName, r, err)
		}
		p[rName] = i
	}
	return p, nil
}
