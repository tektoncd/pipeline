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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	resourcelisters "github.com/tektoncd/pipeline/pkg/client/resource/listers/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/contexts"
	podconvert "github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	"github.com/tektoncd/pipeline/pkg/termination"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/tracker"
)

const (
	// taskRunAgentName defines logging agent name for TaskRun Controller
	taskRunAgentName = "taskrun-controller"
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
	resourceLister    resourcelisters.PipelineResourceLister
	cloudEventClient  cloudevent.CEClient
	tracker           tracker.Interface
	entrypointCache   podconvert.EntrypointCache
	timeoutHandler    *reconciler.TimeoutSet
	metrics           *Recorder
	pvcHandler        volumeclaim.PvcHandler
	configStore       configStore
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

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
	if k8serrors.IsNotFound(err) {
		// The resource no longer exists, in which case we stop processing.
		c.Logger.Infof("task run %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		c.Logger.Errorf("Error retrieving TaskRun %q: %s", name, err)
		return err
	}

	// Don't modify the informer's copy.
	tr := original.DeepCopy()

	// If the TaskRun is just starting, this will also set the starttime,
	// from which the timeout will immediately begin counting down.
	if !tr.HasStarted() {
		tr.Status.InitializeConditions()
		// In case node time was not synchronized, when controller has been scheduled to other nodes.
		if tr.Status.StartTime.Sub(tr.CreationTimestamp.Time) < 0 {
			c.Logger.Warnf("TaskRun %s createTimestamp %s is after the taskRun started %s", tr.GetRunKey(), tr.CreationTimestamp, tr.Status.StartTime)
			tr.Status.StartTime = &tr.CreationTimestamp
		}
		// Emit events. During the first reconcile the status of the TaskRun may change twice
		// from not Started to Started and then to Running, so we need to sent the event here
		// and at the end of 'Reconcile' again.
		// We also want to send the "Started" event as soon as possible for anyone who may be waiting
		// on the event to perform user facing initialisations, such has reset a CI check status
		afterCondition := tr.Status.GetCondition(apis.ConditionSucceeded)
		events.Emit(c.Recorder, nil, afterCondition, tr)
	}

	// If the TaskRun is complete, run some post run fixtures when applicable
	if tr.IsDone() {
		c.Logger.Infof("taskrun done : %s \n", tr.Name)
		var merr *multierror.Error
		// Try to send cloud events first
		cloudEventErr := cloudevent.SendCloudEvents(tr, c.cloudEventClient, c.Logger)
		// Regardless of `err`, we must write back any status update that may have
		// been generated by `sendCloudEvents`
		updateErr := c.updateStatusLabelsAndAnnotations(tr, original)
		merr = multierror.Append(cloudEventErr, updateErr)
		if cloudEventErr != nil {
			// Let's keep timeouts and sidecars running as long as we're trying to
			// send cloud events. So we stop here an return errors encountered this far.
			return merr.ErrorOrNil()
		}
		c.timeoutHandler.Release(tr)
		pod, err := c.KubeClientSet.CoreV1().Pods(tr.Namespace).Get(tr.Status.PodName, metav1.GetOptions{})
		if err == nil {
			err = podconvert.StopSidecars(c.Images.NopImage, c.KubeClientSet, *pod)
			if err == nil {
				// Check if any SidecarStatuses are still shown as Running after stopping
				// Sidecars. If any Running, update SidecarStatuses based on Pod ContainerStatuses.
				if podconvert.IsSidecarStatusRunning(tr) {
					err = updateStoppedSidecarStatus(pod, tr, c)
				}
			}
		} else if k8serrors.IsNotFound(err) {
			return merr.ErrorOrNil()
		}
		if err != nil {
			c.Logger.Errorf("Error stopping sidecars for TaskRun %q: %v", name, err)
			merr = multierror.Append(merr, err)
		}

		go func(metrics *Recorder) {
			err := metrics.DurationAndCount(tr)
			if err != nil {
				c.Logger.Warnf("Failed to log the metrics : %v", err)
			}
			err = metrics.RecordPodLatency(pod, tr)
			if err != nil {
				c.Logger.Warnf("Failed to log the metrics : %v", err)
			}
		}(c.metrics)

		return merr.ErrorOrNil()
	}

	// If the TaskRun is cancelled, kill resources and update status
	if tr.IsCancelled() {
		before := tr.Status.GetCondition(apis.ConditionSucceeded)
		message := fmt.Sprintf("TaskRun %q was cancelled", tr.Name)
		err := c.failTaskRun(tr, v1beta1.TaskRunReasonCancelled, message)
		return c.finishReconcileUpdateEmitEvents(tr, original, before, err)
	}

	// Check if the TaskRun has timed out; if it is, this will set its status
	// accordingly.
	if tr.HasTimedOut() {
		before := tr.Status.GetCondition(apis.ConditionSucceeded)
		message := fmt.Sprintf("TaskRun %q failed to finish within %q", tr.Name, tr.GetTimeout())
		err := c.failTaskRun(tr, podconvert.ReasonTimedOut, message)
		return c.finishReconcileUpdateEmitEvents(tr, original, before, err)
	}

	// prepare fetches all required resources, validates them together with the
	// taskrun, runs API convertions. Errors that come out of prepare are
	// permanent one, so in case of error we update, emit events and return
	taskSpec, rtr, err := c.prepare(ctx, tr)
	if err != nil {
		c.Logger.Errorf("TaskRun prepare error: %v", err.Error())
		// We only return an error if update failed, otherwise we don't want to
		// reconcile an invalid TaskRun anymore
		return c.finishReconcileUpdateEmitEvents(tr, original, nil, nil)
	}

	// Store the condition before reconcile
	before := tr.Status.GetCondition(apis.ConditionSucceeded)

	// Reconcile this copy of the task run and then write back any status
	// updates regardless of whether the reconciliation errored out.
	if err = c.reconcile(ctx, tr, taskSpec, rtr); err != nil {
		c.Logger.Errorf("Reconcile error: %v", err.Error())
	}
	// Emit events (only when ConditionSucceeded was changed)
	return c.finishReconcileUpdateEmitEvents(tr, original, before, err)
}

func (c *Reconciler) finishReconcileUpdateEmitEvents(tr, original *v1beta1.TaskRun, beforeCondition *apis.Condition, previousError error) error {
	afterCondition := tr.Status.GetCondition(apis.ConditionSucceeded)
	events.Emit(c.Recorder, beforeCondition, afterCondition, tr)
	err := c.updateStatusLabelsAndAnnotations(tr, original)
	events.EmitError(c.Recorder, err, tr)
	return multierror.Append(previousError, err).ErrorOrNil()
}

func (c *Reconciler) getTaskResolver(tr *v1beta1.TaskRun) (*resources.LocalTaskRefResolver, v1beta1.TaskKind) {
	resolver := &resources.LocalTaskRefResolver{
		Namespace:    tr.Namespace,
		Tektonclient: c.PipelineClientSet,
	}
	kind := v1beta1.NamespacedTaskKind
	if tr.Spec.TaskRef != nil && tr.Spec.TaskRef.Kind == v1beta1.ClusterTaskKind {
		kind = v1beta1.ClusterTaskKind
	}
	resolver.Kind = kind
	return resolver, kind
}

// `prepare` fetches resources the taskrun depends on, runs validation and conversion
// It may report errors back to Reconcile, it updates the taskrun status in case of
// error but it does not sync updates back to etcd. It does not emit events.
// All errors returned by `prepare` are always handled by `Reconcile`, so they don't cause
// the key to be re-queued directly. Once we start using `PermanentErrors` code in
// `prepare` will be able to control which error is handled by `Reconcile` and which is not
// See https://github.com/tektoncd/pipeline/issues/2474 for details.
// `prepare` returns spec and resources. In future we might store
// them in the TaskRun.Status so we don't need to re-run `prepare` at every
// reconcile (see https://github.com/tektoncd/pipeline/issues/2473).
func (c *Reconciler) prepare(ctx context.Context, tr *v1beta1.TaskRun) (*v1beta1.TaskSpec, *resources.ResolvedTaskResources, error) {
	// We may be reading a version of the object that was stored at an older version
	// and may not have had all of the assumed default specified.
	tr.SetDefaults(contexts.WithUpgradeViaDefaulting(ctx))

	resolver, kind := c.getTaskResolver(tr)
	taskMeta, taskSpec, err := resources.GetTaskData(ctx, tr, resolver.GetTask)
	if err != nil {
		if ce, ok := err.(*v1beta1.CannotConvertError); ok {
			tr.Status.MarkResourceNotConvertible(ce)
		}
		c.Logger.Errorf("Failed to determine Task spec to use for taskrun %s: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedResolution, err)
		return nil, nil, err
	}

	// Store the fetched TaskSpec on the TaskRun for auditing
	if err := storeTaskSpec(ctx, tr, taskSpec); err != nil {
		c.Logger.Errorf("Failed to store TaskSpec on TaskRun.Statusfor taskrun %s: %v", tr.Name, err)
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
		if tr.Spec.TaskRef.Kind == "ClusterTask" {
			tr.ObjectMeta.Labels[pipeline.GroupName+pipeline.ClusterTaskLabelKey] = taskMeta.Name
		}
	}

	// Propagate annotations from Task to TaskRun.
	if tr.ObjectMeta.Annotations == nil {
		tr.ObjectMeta.Annotations = make(map[string]string, len(taskMeta.Annotations))
	}
	for key, value := range taskMeta.Annotations {
		tr.ObjectMeta.Annotations[key] = value
	}

	inputs := []v1beta1.TaskResourceBinding{}
	outputs := []v1beta1.TaskResourceBinding{}
	if tr.Spec.Resources != nil {
		inputs = tr.Spec.Resources.Inputs
		outputs = tr.Spec.Resources.Outputs
	}
	rtr, err := resources.ResolveTaskResources(taskSpec, taskMeta.Name, kind, inputs, outputs, c.resourceLister.PipelineResources(tr.Namespace).Get)
	if err != nil {
		c.Logger.Errorf("Failed to resolve references for taskrun %s: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedResolution, err)
		return nil, nil, err
	}

	if err := ValidateResolvedTaskResources(tr.Spec.Params, rtr); err != nil {
		c.Logger.Errorf("TaskRun %q resources are invalid: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
		return nil, nil, err
	}

	if err := workspace.ValidateBindings(taskSpec.Workspaces, tr.Spec.Workspaces); err != nil {
		c.Logger.Errorf("TaskRun %q workspaces are invalid: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
		return nil, nil, err
	}

	// Initialize the cloud events if at least a CloudEventResource is defined
	// and they have not been initialized yet.
	// FIXME(afrittoli) This resource specific logic will have to be replaced
	// once we have a custom PipelineResource framework in place.
	c.Logger.Infof("Cloud Events: %s", tr.Status.CloudEvents)
	prs := make([]*resourcev1alpha1.PipelineResource, 0, len(rtr.Outputs))
	for _, pr := range rtr.Outputs {
		prs = append(prs, pr)
	}
	cloudevent.InitializeCloudEvents(tr, prs)

	return taskSpec, rtr, nil
}

// `reconcile` creates the Pod associated to the TaskRun, and it pulls back status
// updates from the Pod to the TaskRun.
// It reports errors back to Reconcile, it updates the taskrun status in case of
// error but it does not sync updates back to etcd. It does not emit events.
// `reconcile` consumes spec and resources returned by `prepare`
func (c *Reconciler) reconcile(ctx context.Context, tr *v1beta1.TaskRun,
	taskSpec *v1beta1.TaskSpec, rtr *resources.ResolvedTaskResources) error {
	// Get the TaskRun's Pod if it should have one. Otherwise, create the Pod.
	var pod *corev1.Pod
	var err error

	if tr.Status.PodName != "" {
		pod, err = c.KubeClientSet.CoreV1().Pods(tr.Namespace).Get(tr.Status.PodName, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			// Keep going, this will result in the Pod being created below.
		} else if err != nil {
			// This is considered a transient error, so we return error, do not update
			// the task run condition, and return an error which will cause this key to
			// be requeued for reconcile.
			c.Logger.Errorf("Error getting pod %q: %v", tr.Status.PodName, err)
			return err
		}
	} else {
		pos, err := c.KubeClientSet.CoreV1().Pods(tr.Namespace).List(metav1.ListOptions{
			LabelSelector: getLabelSelector(tr),
		})
		if err != nil {
			c.Logger.Errorf("Error listing pods: %v", err)
			return err
		}
		for index := range pos.Items {
			po := pos.Items[index]
			if metav1.IsControlledBy(&po, tr) && !podconvert.DidTaskRunFail(&po) {
				pod = &po
			}
		}
	}

	if pod == nil {
		if tr.HasVolumeClaimTemplate() {
			if err := c.pvcHandler.CreatePersistentVolumeClaimsForWorkspaces(tr.Spec.Workspaces, tr.GetOwnerReference(), tr.Namespace); err != nil {
				c.Logger.Errorf("Failed to create PVC for TaskRun %s: %v", tr.Name, err)
				tr.Status.MarkResourceFailed(volumeclaim.ReasonCouldntCreateWorkspacePVC,
					fmt.Errorf("Failed to create PVC for TaskRun %s workspaces correctly: %s",
						fmt.Sprintf("%s/%s", tr.Namespace, tr.Name), err))
				return nil
			}

			taskRunWorkspaces := applyVolumeClaimTemplates(tr.Spec.Workspaces, tr.GetOwnerReference())
			// This is used by createPod below. Changes to the Spec are not updated.
			tr.Spec.Workspaces = taskRunWorkspaces
		}

		pod, err = c.createPod(ctx, tr, rtr)
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

	if podconvert.IsPodExceedingNodeResources(pod) {
		c.Recorder.Eventf(tr, corev1.EventTypeWarning, podconvert.ReasonExceededNodeResources, "Insufficient resources to schedule pod %q", pod.Name)
	}

	if podconvert.SidecarsReady(pod.Status) {
		if err := podconvert.UpdateReady(c.KubeClientSet, *pod); err != nil {
			return err
		}
	}

	// Convert the Pod's status to the equivalent TaskRun Status.
	tr.Status = podconvert.MakeTaskRunStatus(c.Logger, *tr, pod, *taskSpec)

	if err := updateTaskRunResourceResult(tr, *pod); err != nil {
		return err
	}

	c.Logger.Infof("Successfully reconciled taskrun %s/%s with status: %#v", tr.Name, tr.Namespace, tr.Status.GetCondition(apis.ConditionSucceeded))
	return nil
}

// Push changes (if any) to the TaskRun status, labels and annotations to
// TaskRun definition in ectd
func (c *Reconciler) updateStatusLabelsAndAnnotations(tr, original *v1beta1.TaskRun) error {
	var updated bool

	if !equality.Semantic.DeepEqual(original.Status, tr.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
		if _, err := c.updateStatus(tr); err != nil {
			c.Logger.Warn("Failed to update taskRun status", zap.Error(err))
			return err
		}
		updated = true
	}

	// When we update the status only, we use updateStatus to minimize the chances of
	// racing any clients updating other parts of the Run, e.g. the spec or the labels.
	// If we need to update the labels or annotations, we need to call Update with these
	// changes explicitly.
	if !reflect.DeepEqual(original.ObjectMeta.Labels, tr.ObjectMeta.Labels) || !reflect.DeepEqual(original.ObjectMeta.Annotations, tr.ObjectMeta.Annotations) {
		if _, err := c.updateLabelsAndAnnotations(tr); err != nil {
			c.Logger.Warn("Failed to update TaskRun labels/annotations", zap.Error(err))
			return err
		}
		updated = true
	}

	if updated {
		go func(metrics *Recorder) {
			err := metrics.RunningTaskRuns(c.taskRunLister)
			if err != nil {
				c.Logger.Warnf("Failed to log the metrics : %v", err)
			}
		}(c.metrics)
	}

	return nil
}

func (c *Reconciler) updateStatus(taskrun *v1beta1.TaskRun) (*v1beta1.TaskRun, error) {
	newtaskrun, err := c.taskRunLister.TaskRuns(taskrun.Namespace).Get(taskrun.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting TaskRun %s when updating status: %w", taskrun.Name, err)
	}
	if !reflect.DeepEqual(taskrun.Status, newtaskrun.Status) {
		newtaskrun = newtaskrun.DeepCopy()
		newtaskrun.Status = taskrun.Status
		return c.PipelineClientSet.TektonV1beta1().TaskRuns(taskrun.Namespace).UpdateStatus(newtaskrun)
	}
	return newtaskrun, nil
}

func (c *Reconciler) updateLabelsAndAnnotations(tr *v1beta1.TaskRun) (*v1beta1.TaskRun, error) {
	newTr, err := c.taskRunLister.TaskRuns(tr.Namespace).Get(tr.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting TaskRun %s when updating labels/annotations: %w", tr.Name, err)
	}
	if !reflect.DeepEqual(tr.ObjectMeta.Labels, newTr.ObjectMeta.Labels) || !reflect.DeepEqual(tr.ObjectMeta.Annotations, newTr.ObjectMeta.Annotations) {
		mergePatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels":      tr.ObjectMeta.Labels,
				"annotations": tr.ObjectMeta.Annotations,
			},
		}
		patch, err := json.Marshal(mergePatch)
		if err != nil {
			return nil, err
		}
		return c.PipelineClientSet.TektonV1beta1().TaskRuns(tr.Namespace).Patch(tr.Name, types.MergePatchType, patch)
	}
	return newTr, nil
}

func (c *Reconciler) handlePodCreationError(tr *v1beta1.TaskRun, err error) {
	var msg string
	if isExceededResourceQuotaError(err) {
		backoff, currentlyBackingOff := c.timeoutHandler.GetBackoff(tr)
		if !currentlyBackingOff {
			go c.timeoutHandler.SetTaskRunTimer(tr, time.Until(backoff.NextAttempt))
		}
		msg = fmt.Sprintf("TaskRun Pod exceeded available resources, reattempted %d times", backoff.NumAttempts)
		tr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  podconvert.ReasonExceededResourceQuota,
			Message: fmt.Sprintf("%s: %v", msg, err),
		})
	} else {
		// The pod creation failed, not because of quota issues. The most likely
		// reason is that something is wrong with the spec of the Task, that we could
		// not check with validation before - i.e. pod template fields
		msg = fmt.Sprintf("failed to create task run pod %q: %v. Maybe ", tr.Name, err)
		if tr.Spec.TaskRef != nil {
			msg += fmt.Sprintf("missing or invalid Task %s/%s", tr.Namespace, tr.Spec.TaskRef.Name)
		} else {
			msg += "invalid TaskSpec"
		}
		tr.Status.MarkResourceFailed(podconvert.ReasonCouldntGetTask, errors.New(msg))
	}
	c.Logger.Error("Failed to create task run pod for task %q: %v", tr.Name, err)
}

// failTaskRun stops a TaskRun with the provided Reason
// If a pod is associated to the TaskRun, it stops it
// failTaskRun function may return an error in case the pod could not be deleted
// failTaskRun may update the local TaskRun status, but it won't push the updates to etcd
func (c *Reconciler) failTaskRun(tr *v1beta1.TaskRun, reason, message string) error {

	c.Logger.Warn("stopping task run %q because of %q", tr.Name, reason)
	tr.Status.MarkResourceFailed(reason, errors.New(message))

	// update tr completed time
	tr.Status.CompletionTime = &metav1.Time{Time: time.Now()}

	if tr.Status.PodName == "" {
		c.Logger.Warnf("task run %q has no pod running yet", tr.Name)
		return nil
	}

	// tr.Status.PodName will be empty if the pod was never successfully created. This condition
	// can be reached, for example, by the pod never being schedulable due to limits imposed by
	// a namespace's ResourceQuota.
	err := c.KubeClientSet.CoreV1().Pods(tr.Namespace).Delete(tr.Status.PodName, &metav1.DeleteOptions{})

	if err != nil && !k8serrors.IsNotFound(err) {
		c.Logger.Infof("Failed to terminate pod: %v", err)
		return err
	}
	return nil
}

// createPod creates a Pod based on the Task's configuration, with pvcName as a volumeMount
// TODO(dibyom): Refactor resource setup/substitution logic to its own function in the resources package
func (c *Reconciler) createPod(ctx context.Context, tr *v1beta1.TaskRun, rtr *resources.ResolvedTaskResources) (*corev1.Pod, error) {
	ts := rtr.TaskSpec.DeepCopy()
	inputResources, err := resourceImplBinding(rtr.Inputs, c.Images)
	if err != nil {
		c.Logger.Errorf("Failed to initialize input resources: %v", err)
		return nil, err
	}
	outputResources, err := resourceImplBinding(rtr.Outputs, c.Images)
	if err != nil {
		c.Logger.Errorf("Failed to initialize output resources: %v", err)
		return nil, err
	}

	// Get actual resource
	err = resources.AddOutputImageDigestExporter(c.Images.ImageDigestExporterImage, tr, ts, c.resourceLister.PipelineResources(tr.Namespace).Get)
	if err != nil {
		c.Logger.Errorf("Failed to create a pod for taskrun: %s due to output image resource error %v", tr.Name, err)
		return nil, err
	}

	ts, err = resources.AddInputResource(c.KubeClientSet, c.Images, rtr.TaskName, ts, tr, inputResources, c.Logger)
	if err != nil {
		c.Logger.Errorf("Failed to create a pod for taskrun: %s due to input resource error %v", tr.Name, err)
		return nil, err
	}

	ts, err = resources.AddOutputResources(c.KubeClientSet, c.Images, rtr.TaskName, ts, tr, outputResources, c.Logger)
	if err != nil {
		c.Logger.Errorf("Failed to create a pod for taskrun: %s due to output resource error %v", tr.Name, err)
		return nil, err
	}

	var defaults []v1beta1.ParamSpec
	if len(ts.Params) > 0 {
		defaults = append(defaults, ts.Params...)
	}
	// Apply parameter substitution from the taskrun.
	ts = resources.ApplyParameters(ts, tr, defaults...)

	// Apply bound resource substitution from the taskrun.
	ts = resources.ApplyResources(ts, inputResources, "inputs")
	ts = resources.ApplyResources(ts, outputResources, "outputs")

	// Apply workspace resource substitution
	ts = resources.ApplyWorkspaces(ts, ts.Workspaces, tr.Spec.Workspaces)

	// Apply task result substitution
	ts = resources.ApplyTaskResults(ts)

	ts, err = workspace.Apply(*ts, tr.Spec.Workspaces)
	if err != nil {
		c.Logger.Errorf("Failed to create a pod for taskrun: %s due to workspace error %v", tr.Name, err)
		return nil, err
	}

	// Check if the HOME env var of every Step should be set to /tekton/home.
	shouldOverrideHomeEnv := podconvert.ShouldOverrideHomeEnv(ctx)

	// Apply creds-init path substitutions.
	ts = resources.ApplyCredentialsPath(ts, podconvert.CredentialsPath(shouldOverrideHomeEnv))

	pod, err := podconvert.MakePod(ctx, c.Images, tr, *ts, c.KubeClientSet, c.entrypointCache, shouldOverrideHomeEnv)
	if err != nil {
		return nil, fmt.Errorf("translating TaskSpec to Pod: %w", err)
	}

	return c.KubeClientSet.CoreV1().Pods(tr.Namespace).Create(pod)
}

type DeletePod func(podName string, options *metav1.DeleteOptions) error

func updateTaskRunResourceResult(taskRun *v1beta1.TaskRun, pod corev1.Pod) error {
	podconvert.SortContainerStatuses(&pod)

	if taskRun.IsSuccessful() {
		for idx, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil {
				msg := cs.State.Terminated.Message
				r, err := termination.ParseMessage(msg)
				if err != nil {
					return fmt.Errorf("parsing message for container status %d: %v", idx, err)
				}
				taskResults, pipelineResourceResults := getResults(r)
				taskRun.Status.TaskRunResults = append(taskRun.Status.TaskRunResults, taskResults...)
				taskRun.Status.ResourcesResult = append(taskRun.Status.ResourcesResult, pipelineResourceResults...)
			}
		}
		taskRun.Status.TaskRunResults = removeDuplicateResults(taskRun.Status.TaskRunResults)
	}
	return nil
}

func getResults(results []v1beta1.PipelineResourceResult) ([]v1beta1.TaskRunResult, []v1beta1.PipelineResourceResult) {
	var taskResults []v1beta1.TaskRunResult
	var pipelineResourceResults []v1beta1.PipelineResourceResult
	for _, r := range results {
		switch r.ResultType {
		case v1beta1.TaskRunResultType:
			taskRunResult := v1beta1.TaskRunResult{
				Name:  r.Key,
				Value: r.Value,
			}
			taskResults = append(taskResults, taskRunResult)
		case v1beta1.PipelineResourceResultType:
			fallthrough
		default:
			pipelineResourceResults = append(pipelineResourceResults, r)
		}
	}
	return taskResults, pipelineResourceResults
}

func removeDuplicateResults(taskRunResult []v1beta1.TaskRunResult) []v1beta1.TaskRunResult {
	uniq := make([]v1beta1.TaskRunResult, 0)
	latest := make(map[string]v1beta1.TaskRunResult, 0)
	for _, res := range taskRunResult {
		if _, seen := latest[res.Name]; !seen {
			uniq = append(uniq, res)
		}
		latest[res.Name] = res
	}
	for i, res := range uniq {
		uniq[i] = latest[res.Name]
	}
	return uniq
}

func isExceededResourceQuotaError(err error) bool {
	return err != nil && k8serrors.IsForbidden(err) && strings.Contains(err.Error(), "exceeded quota")
}

// resourceImplBinding maps pipeline resource names to the actual resource type implementations
func resourceImplBinding(resources map[string]*resourcev1alpha1.PipelineResource, images pipeline.Images) (map[string]v1beta1.PipelineResourceInterface, error) {
	p := make(map[string]v1beta1.PipelineResourceInterface)
	for rName, r := range resources {
		i, err := resource.FromType(r, images)
		if err != nil {
			return nil, fmt.Errorf("failed to create resource %s : %v with error: %w", rName, r, err)
		}
		p[rName] = i
	}
	return p, nil
}

// getLabelSelector get label of centain taskrun
func getLabelSelector(tr *v1beta1.TaskRun) string {
	labels := []string{}
	labelsMap := podconvert.MakeLabels(tr)
	for key, value := range labelsMap {
		labels = append(labels, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(labels, ",")
}

// updateStoppedSidecarStatus updates SidecarStatus for sidecars that were
// terminated by nop image
func updateStoppedSidecarStatus(pod *corev1.Pod, tr *v1beta1.TaskRun, c *Reconciler) error {
	tr.Status.Sidecars = []v1beta1.SidecarState{}
	for _, s := range pod.Status.ContainerStatuses {
		if !podconvert.IsContainerStep(s.Name) {
			var sidecarState corev1.ContainerState
			if s.LastTerminationState.Terminated != nil {
				// Sidecar has successfully by terminated by nop image
				lastTerminatedState := s.LastTerminationState.Terminated
				sidecarState = corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode:    lastTerminatedState.ExitCode,
						Reason:      "Completed",
						Message:     "Sidecar container successfully stopped by nop image",
						StartedAt:   lastTerminatedState.StartedAt,
						FinishedAt:  lastTerminatedState.FinishedAt,
						ContainerID: lastTerminatedState.ContainerID,
					},
				}
			} else {
				// Sidecar has not been terminated
				sidecarState = s.State
			}

			tr.Status.Sidecars = append(tr.Status.Sidecars, v1beta1.SidecarState{
				ContainerState: *sidecarState.DeepCopy(),
				Name:           podconvert.TrimSidecarPrefix(s.Name),
				ContainerName:  s.Name,
				ImageID:        s.ImageID,
			})
		}
	}
	_, err := c.updateStatus(tr)
	return err
}

// apply VolumeClaimTemplates and return WorkspaceBindings were templates is translated to PersistentVolumeClaims
func applyVolumeClaimTemplates(workspaceBindings []v1beta1.WorkspaceBinding, owner metav1.OwnerReference) []v1beta1.WorkspaceBinding {
	taskRunWorkspaceBindings := make([]v1beta1.WorkspaceBinding, 0)
	for _, wb := range workspaceBindings {
		if wb.VolumeClaimTemplate == nil {
			taskRunWorkspaceBindings = append(taskRunWorkspaceBindings, wb)
			continue
		}

		// apply template
		b := v1beta1.WorkspaceBinding{
			Name:    wb.Name,
			SubPath: wb.SubPath,
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: volumeclaim.GetPersistentVolumeClaimName(wb.VolumeClaimTemplate, wb, owner),
			},
		}
		taskRunWorkspaceBindings = append(taskRunWorkspaceBindings, b)
	}
	return taskRunWorkspaceBindings
}

func storeTaskSpec(ctx context.Context, tr *v1beta1.TaskRun, ts *v1beta1.TaskSpec) error {
	// Only store the TaskSpec once, if it has never been set before.
	if tr.Status.TaskSpec == nil {
		tr.Status.TaskSpec = ts
	}
	return nil
}
