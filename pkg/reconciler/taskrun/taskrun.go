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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	taskrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/taskrun"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	resourcelisters "github.com/tektoncd/pipeline/pkg/client/resource/listers/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/internal/affinityassistant"
	"github.com/tektoncd/pipeline/pkg/internal/computeresources"
	podconvert "github.com/tektoncd/pipeline/pkg/pod"
	tknreconciler "github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	"github.com/tektoncd/pipeline/pkg/remote"
	resolution "github.com/tektoncd/pipeline/pkg/resolution/resource"
	"github.com/tektoncd/pipeline/pkg/taskrunmetrics"
	_ "github.com/tektoncd/pipeline/pkg/taskrunmetrics/fake" // Make sure the taskrunmetrics are setup
	"github.com/tektoncd/pipeline/pkg/workspace"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1Listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/changeset"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmap"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"sigs.k8s.io/yaml"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	KubeClientSet     kubernetes.Interface
	PipelineClientSet clientset.Interface
	Images            pipeline.Images
	Clock             clock.PassiveClock

	// listers index properties about resources
	taskRunLister       listers.TaskRunLister
	resourceLister      resourcelisters.PipelineResourceLister
	limitrangeLister    corev1Listers.LimitRangeLister
	podLister           corev1Listers.PodLister
	cloudEventClient    cloudevent.CEClient
	entrypointCache     podconvert.EntrypointCache
	metrics             *taskrunmetrics.Recorder
	pvcHandler          volumeclaim.PvcHandler
	resolutionRequester resolution.Requester
}

// Check that our Reconciler implements taskrunreconciler.Interface
var (
	_ taskrunreconciler.Interface = (*Reconciler)(nil)
)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Task Run
// resource with the current status of the resource.
func (c *Reconciler) ReconcileKind(ctx context.Context, tr *v1beta1.TaskRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	ctx = cloudevent.ToContext(ctx, c.cloudEventClient)
	// By this time, params and workspaces should not be propagated for embedded tasks so we cannot
	// validate that all parameter variables and workspaces used in the TaskSpec are declared by the Task.
	ctx = config.SkipValidationDueToPropagatedParametersAndWorkspaces(ctx, true)

	// Read the initial condition
	before := tr.Status.GetCondition(apis.ConditionSucceeded)

	// If the TaskRun is just starting, this will also set the starttime,
	// from which the timeout will immediately begin counting down.
	if !tr.HasStarted() {
		tr.Status.InitializeConditions()
		// In case node time was not synchronized, when controller has been scheduled to other nodes.
		if tr.Status.StartTime.Sub(tr.CreationTimestamp.Time) < 0 {
			logger.Warnf("TaskRun %s createTimestamp %s is after the taskRun started %s", tr.GetNamespacedName().String(), tr.CreationTimestamp, tr.Status.StartTime)
			tr.Status.StartTime = &tr.CreationTimestamp
		}
		// Emit events. During the first reconcile the status of the TaskRun may change twice
		// from not Started to Started and then to Running, so we need to sent the event here
		// and at the end of 'Reconcile' again.
		// We also want to send the "Started" event as soon as possible for anyone who may be waiting
		// on the event to perform user facing initialisations, such has reset a CI check status
		afterCondition := tr.Status.GetCondition(apis.ConditionSucceeded)
		events.Emit(ctx, nil, afterCondition, tr)
	}

	// If the TaskRun is complete, run some post run fixtures when applicable
	if tr.IsDone() {
		logger.Infof("taskrun done : %s \n", tr.Name)

		// We may be reading a version of the object that was stored at an older version
		// and may not have had all of the assumed default specified.
		tr.SetDefaults(ctx)

		// Try to send cloud events first
		cloudEventErr := cloudevent.SendCloudEvents(tr, c.cloudEventClient, logger, c.Clock)
		// Regardless of `err`, we must write back any status update that may have
		// been generated by `sendCloudEvents`
		if cloudEventErr != nil {
			// Let's keep timeouts and sidecars running as long as we're trying to
			// send cloud events. So we stop here an return errors encountered this far.
			return cloudEventErr
		}

		if err := c.stopSidecars(ctx, tr); err != nil {
			return err
		}

		return c.finishReconcileUpdateEmitEvents(ctx, tr, before, nil)
	}

	// If the TaskRun is cancelled, kill resources and update status
	if tr.IsCancelled() {
		message := fmt.Sprintf("TaskRun %q was cancelled. %s", tr.Name, tr.Spec.StatusMessage)
		err := c.failTaskRun(ctx, tr, v1beta1.TaskRunReasonCancelled, message)
		return c.finishReconcileUpdateEmitEvents(ctx, tr, before, err)
	}

	// Check if the TaskRun has timed out; if it is, this will set its status
	// accordingly.
	if tr.HasTimedOut(ctx, c.Clock) {
		message := fmt.Sprintf("TaskRun %q failed to finish within %q", tr.Name, tr.GetTimeout(ctx))
		err := c.failTaskRun(ctx, tr, v1beta1.TaskRunReasonTimedOut, message)
		return c.finishReconcileUpdateEmitEvents(ctx, tr, before, err)
	}

	// Check for Pod Failures
	if failed, reason, message := c.checkPodFailed(tr); failed {
		err := c.failTaskRun(ctx, tr, reason, message)
		return c.finishReconcileUpdateEmitEvents(ctx, tr, before, err)
	}

	// prepare fetches all required resources, validates them together with the
	// taskrun, runs API conversions. In case of error we update, emit events and return.
	_, rtr, err := c.prepare(ctx, tr)
	if err != nil {
		logger.Errorf("TaskRun prepare error: %v", err.Error())
		// We only return an error if update failed, otherwise we don't want to
		// reconcile an invalid TaskRun anymore
		return c.finishReconcileUpdateEmitEvents(ctx, tr, nil, err)
	}

	// Store the condition before reconcile
	before = tr.Status.GetCondition(apis.ConditionSucceeded)

	// Reconcile this copy of the task run and then write back any status
	// updates regardless of whether the reconciliation errored out.
	if err = c.reconcile(ctx, tr, rtr); err != nil {
		logger.Errorf("Reconcile: %v", err.Error())
	}

	// Emit events (only when ConditionSucceeded was changed)
	if err = c.finishReconcileUpdateEmitEvents(ctx, tr, before, err); err != nil {
		return err
	}

	if tr.Status.StartTime != nil {
		// Compute the time since the task started.
		elapsed := c.Clock.Since(tr.Status.StartTime.Time)
		// Snooze this resource until the timeout has elapsed.
		return controller.NewRequeueAfter(tr.GetTimeout(ctx) - elapsed)
	}
	return nil
}

func (c *Reconciler) checkPodFailed(tr *v1beta1.TaskRun) (bool, v1beta1.TaskRunReason, string) {
	for _, step := range tr.Status.Steps {
		if step.Waiting != nil && step.Waiting.Reason == "ImagePullBackOff" {
			image := step.ImageID
			message := fmt.Sprintf(`The step %q in TaskRun %q failed to pull the image %q. The pod errored with the message: "%s."`, step.Name, tr.Name, image, step.Waiting.Message)
			return true, v1beta1.TaskRunReasonImagePullFailed, message
		}
	}
	for _, sidecar := range tr.Status.Sidecars {
		if sidecar.Waiting != nil && sidecar.Waiting.Reason == "ImagePullBackOff" {
			image := sidecar.ImageID
			message := fmt.Sprintf(`The sidecar %q in TaskRun %q failed to pull the image %q. The pod errored with the message: "%s."`, sidecar.Name, tr.Name, image, sidecar.Waiting.Message)
			return true, v1beta1.TaskRunReasonImagePullFailed, message
		}
	}
	return false, "", ""
}

func (c *Reconciler) durationAndCountMetrics(ctx context.Context, tr *v1beta1.TaskRun) {
	logger := logging.FromContext(ctx)
	if tr.IsDone() {
		newTr, err := c.taskRunLister.TaskRuns(tr.Namespace).Get(tr.Name)
		if err != nil && !k8serrors.IsNotFound(err) {
			logger.Errorf("Error getting TaskRun %s when updating metrics: %w", tr.Name, err)
			return
		} else if k8serrors.IsNotFound(err) || newTr == nil {
			logger.Debugf("TaskRun %s not found when updating metrics: %w", tr.Name, err)
			return
		}

		before := newTr.Status.GetCondition(apis.ConditionSucceeded)
		go func(metrics *taskrunmetrics.Recorder) {
			if err := metrics.DurationAndCount(ctx, tr, before); err != nil {
				logger.Warnf("Failed to log the metrics : %v", err)
			}
			if err := metrics.CloudEvents(ctx, tr); err != nil {
				logger.Warnf("Failed to log the metrics : %v", err)
			}
		}(c.metrics)
	}
}

func (c *Reconciler) stopSidecars(ctx context.Context, tr *v1beta1.TaskRun) error {
	logger := logging.FromContext(ctx)
	// do not continue without knowing the associated pod
	if tr.Status.PodName == "" {
		return nil
	}

	// do not continue if the TaskSpec had no sidecars
	if tr.Status.TaskSpec != nil && len(tr.Status.TaskSpec.Sidecars) == 0 {
		return nil
	}

	// do not continue if the TaskRun was canceled or timed out as this caused the pod to be deleted in failTaskRun
	condition := tr.Status.GetCondition(apis.ConditionSucceeded)
	if condition != nil {
		reason := v1beta1.TaskRunReason(condition.Reason)
		if reason == v1beta1.TaskRunReasonCancelled || reason == v1beta1.TaskRunReasonTimedOut {
			return nil
		}
	}

	// do not continue if there are no Running sidecars.
	if !podconvert.IsSidecarStatusRunning(tr) {
		return nil
	}

	pod, err := podconvert.StopSidecars(ctx, c.Images.NopImage, c.KubeClientSet, tr.Namespace, tr.Status.PodName)
	if err == nil {
		// Check if any SidecarStatuses are still shown as Running after stopping
		// Sidecars. If any Running, update SidecarStatuses based on Pod ContainerStatuses.
		if podconvert.IsSidecarStatusRunning(tr) {
			err = updateStoppedSidecarStatus(pod, tr)
		}
	}
	if k8serrors.IsNotFound(err) {
		// At this stage the TaskRun has been completed if the pod is not found, it won't come back,
		// it has probably evicted. We can return the error, but we consider it a permanent one.
		return controller.NewPermanentError(err)
	} else if err != nil {
		logger.Errorf("Error stopping sidecars for TaskRun %q: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedResolution, err)
	}
	return nil
}

func (c *Reconciler) finishReconcileUpdateEmitEvents(ctx context.Context, tr *v1beta1.TaskRun, beforeCondition *apis.Condition, previousError error) error {
	logger := logging.FromContext(ctx)

	afterCondition := tr.Status.GetCondition(apis.ConditionSucceeded)

	// Send k8s events and cloud events (when configured)
	events.Emit(ctx, beforeCondition, afterCondition, tr)

	_, err := c.updateLabelsAndAnnotations(ctx, tr)
	if err != nil {
		logger.Warn("Failed to update TaskRun labels/annotations", zap.Error(err))
		events.EmitError(controller.GetEventRecorder(ctx), err, tr)
	}

	merr := multierror.Append(previousError, err).ErrorOrNil()
	if controller.IsPermanentError(previousError) {
		return controller.NewPermanentError(merr)
	}
	return merr
}

// `prepare` fetches resources the taskrun depends on, runs validation and conversion
// It may report errors back to Reconcile, it updates the taskrun status in case of
// error but it does not sync updates back to etcd. It does not emit events.
// All errors returned by `prepare` are always handled by `Reconcile`, so they don't cause
// the key to be re-queued directly.
// `prepare` returns spec and resources. In future we might store
// them in the TaskRun.Status so we don't need to re-run `prepare` at every
// reconcile (see https://github.com/tektoncd/pipeline/issues/2473).
func (c *Reconciler) prepare(ctx context.Context, tr *v1beta1.TaskRun) (*v1beta1.TaskSpec, *resources.ResolvedTaskResources, error) {
	logger := logging.FromContext(ctx)
	tr.SetDefaults(ctx)

	getTaskfunc, err := resources.GetTaskFuncFromTaskRun(ctx, c.KubeClientSet, c.PipelineClientSet, c.resolutionRequester, tr)
	if err != nil {
		logger.Errorf("Failed to fetch task reference %s: %v", tr.Spec.TaskRef.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedResolution, err)
		return nil, nil, err
	}

	taskMeta, taskSpec, err := resources.GetTaskData(ctx, tr, getTaskfunc)
	switch {
	case errors.Is(err, remote.ErrorRequestInProgress):
		message := fmt.Sprintf("TaskRun %s/%s awaiting remote resource", tr.Namespace, tr.Name)
		tr.Status.MarkResourceOngoing(v1beta1.TaskRunReasonResolvingTaskRef, message)
		return nil, nil, err
	case err != nil:
		logger.Errorf("Failed to determine Task spec to use for taskrun %s: %v", tr.Name, err)
		if resources.IsGetTaskErrTransient(err) {
			return nil, nil, err
		}
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedResolution, err)
		return nil, nil, controller.NewPermanentError(err)
	default:
		// Store the fetched TaskSpec on the TaskRun for auditing
		if err := storeTaskSpecAndMergeMeta(tr, taskSpec, taskMeta); err != nil {
			logger.Errorf("Failed to store TaskSpec on TaskRun.Statusfor taskrun %s: %v", tr.Name, err)
		}
	}

	inputs := []v1beta1.TaskResourceBinding{}
	outputs := []v1beta1.TaskResourceBinding{}
	if tr.Spec.Resources != nil {
		inputs = tr.Spec.Resources.Inputs
		outputs = tr.Spec.Resources.Outputs
	}
	rtr, err := resources.ResolveTaskResources(taskSpec, taskMeta.Name, resources.GetTaskKind(tr), inputs, outputs, c.resourceLister.PipelineResources(tr.Namespace).Get)
	if err != nil {
		if k8serrors.IsNotFound(err) && tknreconciler.IsYoungResource(tr) {
			// For newly created resources, don't fail immediately.
			// Instead return an (non-permanent) error, which will prompt the
			// controller to requeue the key with backoff.
			logger.Warnf("References for taskrun %s not found: %v", tr.Name, err)
			tr.Status.MarkResourceOngoing(podconvert.ReasonFailedResolution,
				fmt.Sprintf("Unable to resolve dependencies for %q: %v", tr.Name, err))
			return nil, nil, err
		}
		logger.Errorf("Failed to resolve references for taskrun %s: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedResolution, err)
		return nil, nil, controller.NewPermanentError(err)
	}

	if err := validateTaskSpecRequestResources(taskSpec); err != nil {
		logger.Errorf("TaskRun %s taskSpec request resources are invalid: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
		return nil, nil, controller.NewPermanentError(err)
	}

	if err := ValidateResolvedTaskResources(ctx, tr.Spec.Params, &v1beta1.Matrix{}, rtr); err != nil {
		logger.Errorf("TaskRun %q resources are invalid: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
		return nil, nil, controller.NewPermanentError(err)
	}

	if err := validateParamArrayIndex(ctx, tr.Spec.Params, rtr.TaskSpec); err != nil {
		logger.Errorf("TaskRun %q Param references are invalid: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
		return nil, nil, controller.NewPermanentError(err)
	}

	if err := c.updateTaskRunWithDefaultWorkspaces(ctx, tr, taskSpec); err != nil {
		logger.Errorf("Failed to update taskrun %s with default workspace: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedResolution, err)
		return nil, nil, controller.NewPermanentError(err)
	}

	var workspaceDeclarations []v1beta1.WorkspaceDeclaration
	// Propagating workspaces allows users to skip declarations
	// In order to validate the workspace bindings we create declarations based on
	// the workspaces provided in the task run spec. This logic is hidden behind the
	// alpha feature gate since propagating workspaces is behind that feature gate.
	// In addition, we only allow this feature for embedded taskSpec.
	if config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == config.AlphaAPIFields && tr.Spec.TaskSpec != nil {
		for _, ws := range tr.Spec.Workspaces {
			wspaceDeclaration := v1beta1.WorkspaceDeclaration{Name: ws.Name}
			workspaceDeclarations = append(workspaceDeclarations, wspaceDeclaration)
		}
		workspaceDeclarations = append(workspaceDeclarations, taskSpec.Workspaces...)
	} else {
		workspaceDeclarations = taskSpec.Workspaces
	}
	if err := workspace.ValidateBindings(ctx, workspaceDeclarations, tr.Spec.Workspaces); err != nil {
		logger.Errorf("TaskRun %q workspaces are invalid: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
		return nil, nil, controller.NewPermanentError(err)
	}

	if _, usesAssistant := tr.Annotations[workspace.AnnotationAffinityAssistantName]; usesAssistant {
		if err := workspace.ValidateOnlyOnePVCIsUsed(tr.Spec.Workspaces); err != nil {
			logger.Errorf("TaskRun %q workspaces incompatible with Affinity Assistant: %v", tr.Name, err)
			tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
			return nil, nil, controller.NewPermanentError(err)
		}
	}

	if err := validateOverrides(taskSpec, &tr.Spec); err != nil {
		logger.Errorf("TaskRun %q step or sidecar overrides are invalid: %v", tr.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
		return nil, nil, controller.NewPermanentError(err)
	}

	// Initialize the cloud events if at least a CloudEventResource is defined
	// and they have not been initialized yet.
	// FIXME(afrittoli) This resource specific logic will have to be replaced
	// once we have a custom PipelineResource framework in place.
	logger.Debugf("Cloud Events: %s", tr.Status.CloudEvents)
	cloudevent.InitializeCloudEvents(tr, rtr.Outputs)

	return taskSpec, rtr, nil
}

// `reconcile` creates the Pod associated to the TaskRun, and it pulls back status
// updates from the Pod to the TaskRun.
// It reports errors back to Reconcile, it updates the taskrun status in case of
// error but it does not sync updates back to etcd. It does not emit events.
// `reconcile` consumes spec and resources returned by `prepare`
func (c *Reconciler) reconcile(ctx context.Context, tr *v1beta1.TaskRun, rtr *resources.ResolvedTaskResources) error {
	defer c.durationAndCountMetrics(ctx, tr)
	logger := logging.FromContext(ctx)
	recorder := controller.GetEventRecorder(ctx)
	var err error

	// Get the TaskRun's Pod if it should have one. Otherwise, create the Pod.
	var pod *corev1.Pod

	if tr.Status.PodName != "" {
		pod, err = c.podLister.Pods(tr.Namespace).Get(tr.Status.PodName)
		if k8serrors.IsNotFound(err) {
			// Keep going, this will result in the Pod being created below.
		} else if err != nil {
			// This is considered a transient error, so we return error, do not update
			// the task run condition, and return an error which will cause this key to
			// be requeued for reconcile.
			logger.Errorf("Error getting pod %q: %v", tr.Status.PodName, err)
			return err
		}
	} else {
		// List pods that have a label with this TaskRun name.  Do not include other labels from the
		// TaskRun in this selector.  The user could change them during the lifetime of the TaskRun so the
		// current labels may not be set on a previously created Pod.
		labelSelector := labels.Set{pipeline.TaskRunLabelKey: tr.Name}
		pos, err := c.podLister.Pods(tr.Namespace).List(labelSelector.AsSelector())

		if err != nil {
			logger.Errorf("Error listing pods: %v", err)
			return err
		}
		for index := range pos {
			po := pos[index]
			if metav1.IsControlledBy(po, tr) && !podconvert.DidTaskRunFail(po) {
				pod = po
			}
		}
	}

	// Please note that this block is required to run before `applyParamsContextsResultsAndWorkspaces` is called the first time,
	// and that `applyParamsContextsResultsAndWorkspaces` _must_ be called on every reconcile.
	if pod == nil && tr.HasVolumeClaimTemplate() {
		if err := c.pvcHandler.CreatePersistentVolumeClaimsForWorkspaces(ctx, tr.Spec.Workspaces, *kmeta.NewControllerRef(tr), tr.Namespace); err != nil {
			logger.Errorf("Failed to create PVC for TaskRun %s: %v", tr.Name, err)
			tr.Status.MarkResourceFailed(volumeclaim.ReasonCouldntCreateWorkspacePVC,
				fmt.Errorf("Failed to create PVC for TaskRun %s workspaces correctly: %s",
					fmt.Sprintf("%s/%s", tr.Namespace, tr.Name), err))
			return controller.NewPermanentError(err)
		}

		taskRunWorkspaces := applyVolumeClaimTemplates(tr.Spec.Workspaces, *kmeta.NewControllerRef(tr))
		// This is used by createPod below. Changes to the Spec are not updated.
		tr.Spec.Workspaces = taskRunWorkspaces
	}

	// Get the randomized volume names assigned to workspace bindings
	workspaceVolumes := workspace.CreateVolumes(tr.Spec.Workspaces)

	ts, err := applyParamsContextsResultsAndWorkspaces(ctx, tr, rtr, workspaceVolumes)
	if err != nil {
		logger.Errorf("Error updating task spec parameters, contexts, results and workspaces: %s", err)
		return err
	}
	tr.Status.TaskSpec = ts

	if len(tr.Status.TaskSpec.Steps) > 0 {
		logger.Debugf("set taskspec for %s/%s - script: %s", tr.Namespace, tr.Name, tr.Status.TaskSpec.Steps[0].Script)
	}

	if pod == nil {
		pod, err = c.createPod(ctx, ts, tr, rtr, workspaceVolumes)
		if err != nil {
			newErr := c.handlePodCreationError(tr, err)
			logger.Errorf("Failed to create task run pod for taskrun %q: %v", tr.Name, newErr)
			return newErr
		}
	}

	if podconvert.IsPodExceedingNodeResources(pod) {
		recorder.Eventf(tr, corev1.EventTypeWarning, podconvert.ReasonExceededNodeResources, "Insufficient resources to schedule pod %q", pod.Name)
	}

	if podconvert.SidecarsReady(pod.Status) {
		if err := podconvert.UpdateReady(ctx, c.KubeClientSet, *pod); err != nil {
			return err
		}
		if err := c.metrics.RecordPodLatency(ctx, pod, tr); err != nil {
			logger.Warnf("Failed to log the metrics : %v", err)
		}
	}

	// Convert the Pod's status to the equivalent TaskRun Status.
	tr.Status, err = podconvert.MakeTaskRunStatus(logger, *tr, pod)
	if err != nil {
		return err
	}

	if err := validateTaskRunResults(tr, rtr.TaskSpec); err != nil {
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
		return err
	}

	logger.Infof("Successfully reconciled taskrun %s/%s with status: %#v", tr.Name, tr.Namespace, tr.Status.GetCondition(apis.ConditionSucceeded))
	return nil
}

func (c *Reconciler) updateTaskRunWithDefaultWorkspaces(ctx context.Context, tr *v1beta1.TaskRun, taskSpec *v1beta1.TaskSpec) error {
	configMap := config.FromContextOrDefaults(ctx)
	defaults := configMap.Defaults
	if defaults.DefaultTaskRunWorkspaceBinding != "" {
		var defaultWS v1beta1.WorkspaceBinding
		if err := yaml.Unmarshal([]byte(defaults.DefaultTaskRunWorkspaceBinding), &defaultWS); err != nil {
			return fmt.Errorf("failed to unmarshal %v", defaults.DefaultTaskRunWorkspaceBinding)
		}
		workspaceBindings := map[string]v1beta1.WorkspaceBinding{}
		for _, tsWorkspace := range taskSpec.Workspaces {
			if !tsWorkspace.Optional {
				workspaceBindings[tsWorkspace.Name] = v1beta1.WorkspaceBinding{
					Name:                  tsWorkspace.Name,
					SubPath:               defaultWS.SubPath,
					VolumeClaimTemplate:   defaultWS.VolumeClaimTemplate,
					PersistentVolumeClaim: defaultWS.PersistentVolumeClaim,
					EmptyDir:              defaultWS.EmptyDir,
					ConfigMap:             defaultWS.ConfigMap,
					Secret:                defaultWS.Secret,
				}
			}
		}

		for _, trWorkspace := range tr.Spec.Workspaces {
			workspaceBindings[trWorkspace.Name] = trWorkspace
		}

		tr.Spec.Workspaces = []v1beta1.WorkspaceBinding{}
		for _, wsBinding := range workspaceBindings {
			tr.Spec.Workspaces = append(tr.Spec.Workspaces, wsBinding)
		}
	}
	return nil
}

func (c *Reconciler) updateLabelsAndAnnotations(ctx context.Context, tr *v1beta1.TaskRun) (*v1beta1.TaskRun, error) {
	// Ensure the TaskRun is properly decorated with the version of the Tekton controller processing it.
	if tr.Annotations == nil {
		tr.Annotations = make(map[string]string, 1)
	}
	tr.Annotations[podconvert.ReleaseAnnotation] = changeset.Get()

	newTr, err := c.taskRunLister.TaskRuns(tr.Namespace).Get(tr.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting TaskRun %s when updating labels/annotations: %w", tr.Name, err)
	}
	if !reflect.DeepEqual(tr.ObjectMeta.Labels, newTr.ObjectMeta.Labels) || !reflect.DeepEqual(tr.ObjectMeta.Annotations, newTr.ObjectMeta.Annotations) {
		// Note that this uses Update vs. Patch because the former is significantly easier to test.
		// If we want to switch this to Patch, then we will need to teach the utilities in test/controller.go
		// to deal with Patch (setting resourceVersion, and optimistic concurrency checks).
		newTr = newTr.DeepCopy()
		newTr.Labels = kmap.Union(newTr.Labels, tr.Labels)
		newTr.Annotations = kmap.Union(newTr.Annotations, tr.Annotations)
		return c.PipelineClientSet.TektonV1beta1().TaskRuns(tr.Namespace).Update(ctx, newTr, metav1.UpdateOptions{})
	}
	return newTr, nil
}

func (c *Reconciler) handlePodCreationError(tr *v1beta1.TaskRun, err error) error {
	switch {
	case isResourceQuotaConflictError(err):
		// Requeue if it runs into ResourceQuotaConflictError Error i.e https://github.com/kubernetes/kubernetes/issues/67761
		tr.Status.StartTime = nil
		tr.Status.MarkResourceOngoing(podconvert.ReasonPending, fmt.Sprint("tried to create pod, but it failed with ResourceQuotaConflictError"))
		return controller.NewRequeueAfter(time.Second)
	case isExceededResourceQuotaError(err):
		// If we are struggling to create the pod, then it hasn't started.
		tr.Status.StartTime = nil
		tr.Status.MarkResourceOngoing(podconvert.ReasonExceededResourceQuota, fmt.Sprint("TaskRun Pod exceeded available resources: ", err))
		return controller.NewRequeueAfter(time.Minute)
	case isTaskRunValidationFailed(err):
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
	case k8serrors.IsAlreadyExists(err):
		tr.Status.MarkResourceOngoing(podconvert.ReasonPending, fmt.Sprint("tried to create pod, but it already exists"))
	default:
		// The pod creation failed with unknown reason. The most likely
		// reason is that something is wrong with the spec of the Task, that we could
		// not check with validation before - i.e. pod template fields
		msg := fmt.Sprintf("failed to create task run pod %q: %v. Maybe ", tr.Name, err)
		if tr.Spec.TaskRef != nil {
			msg += fmt.Sprintf("missing or invalid Task %s/%s", tr.Namespace, tr.Spec.TaskRef.Name)
		} else {
			msg += "invalid TaskSpec"
		}
		err = controller.NewPermanentError(errors.New(msg))
		tr.Status.MarkResourceFailed(podconvert.ReasonCouldntGetTask, err)
	}
	return err
}

// failTaskRun stops a TaskRun with the provided Reason
// If a pod is associated to the TaskRun, it stops it
// failTaskRun function may return an error in case the pod could not be deleted
// failTaskRun may update the local TaskRun status, but it won't push the updates to etcd
func (c *Reconciler) failTaskRun(ctx context.Context, tr *v1beta1.TaskRun, reason v1beta1.TaskRunReason, message string) error {
	logger := logging.FromContext(ctx)

	logger.Warnf("stopping task run %q because of %q", tr.Name, reason)
	tr.Status.MarkResourceFailed(reason, errors.New(message))

	completionTime := metav1.Time{Time: c.Clock.Now()}
	// update tr completed time
	tr.Status.CompletionTime = &completionTime

	if tr.Status.PodName == "" {
		logger.Warnf("task run %q has no pod running yet", tr.Name)
		return nil
	}

	// tr.Status.PodName will be empty if the pod was never successfully created. This condition
	// can be reached, for example, by the pod never being schedulable due to limits imposed by
	// a namespace's ResourceQuota.
	err := c.KubeClientSet.CoreV1().Pods(tr.Namespace).Delete(ctx, tr.Status.PodName, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.Infof("Failed to terminate pod: %v", err)
		return err
	}

	// Update step states for TaskRun on TaskRun object since pod has been deleted for cancel or timeout
	for i, step := range tr.Status.Steps {
		// If running, include StartedAt for when step began running
		if step.Running != nil {
			step.Terminated = &corev1.ContainerStateTerminated{
				ExitCode:   1,
				StartedAt:  step.Running.StartedAt,
				FinishedAt: completionTime,
				Reason:     reason.String(),
			}
			step.Running = nil
			tr.Status.Steps[i] = step
		}

		if step.Waiting != nil {
			step.Terminated = &corev1.ContainerStateTerminated{
				ExitCode:   1,
				FinishedAt: completionTime,
				Reason:     reason.String(),
			}
			step.Waiting = nil
			tr.Status.Steps[i] = step
		}
	}

	return nil
}

// createPod creates a Pod based on the Task's configuration, with pvcName as a volumeMount
// TODO(dibyom): Refactor resource setup/substitution logic to its own function in the resources package
func (c *Reconciler) createPod(ctx context.Context, ts *v1beta1.TaskSpec, tr *v1beta1.TaskRun, rtr *resources.ResolvedTaskResources, workspaceVolumes map[string]corev1.Volume) (*corev1.Pod, error) {
	logger := logging.FromContext(ctx)
	inputResources, err := resourceImplBinding(rtr.Inputs, c.Images)
	if err != nil {
		logger.Errorf("Failed to initialize input resources: %v", err)
		return nil, err
	}
	outputResources, err := resourceImplBinding(rtr.Outputs, c.Images)
	if err != nil {
		logger.Errorf("Failed to initialize output resources: %v", err)
		return nil, err
	}

	// Get actual resource
	err = resources.AddOutputImageDigestExporter(c.Images.ImageDigestExporterImage, tr, ts, c.resourceLister.PipelineResources(tr.Namespace).Get)
	if err != nil {
		logger.Errorf("Failed to create a pod for taskrun: %s due to output image resource error %v", tr.Name, err)
		return nil, err
	}

	ts, err = resources.AddInputResource(ctx, c.KubeClientSet, c.Images, rtr.TaskName, ts, tr, inputResources)
	if err != nil {
		logger.Errorf("Failed to create a pod for taskrun: %s due to input resource error %v", tr.Name, err)
		return nil, err
	}

	ts, err = resources.AddOutputResources(ctx, c.KubeClientSet, c.Images, rtr.TaskName, ts, tr, outputResources)
	if err != nil {
		logger.Errorf("Failed to create a pod for taskrun: %s due to output resource error %v", tr.Name, err)
		return nil, err
	}

	// Apply bound resource substitution from the taskrun.
	ts = resources.ApplyResources(ts, inputResources, "inputs")
	ts = resources.ApplyResources(ts, outputResources, "outputs")

	// By this time, params and workspaces should be propagated down so we can
	// validate that all parameter variables and workspaces used in the TaskSpec are declared by the Task.
	ctx = config.SkipValidationDueToPropagatedParametersAndWorkspaces(ctx, false)
	if validateErr := ts.Validate(ctx); validateErr != nil {
		logger.Errorf("Failed to create a pod for taskrun: %s due to task validation error %v", tr.Name, validateErr)
		return nil, validateErr
	}

	ts, err = workspace.Apply(ctx, *ts, tr.Spec.Workspaces, workspaceVolumes)

	if err != nil {
		logger.Errorf("Failed to create a pod for taskrun: %s due to workspace error %v", tr.Name, err)
		return nil, err
	}

	// Apply path substitutions for the legacy credentials helper (aka "creds-init")
	ts = resources.ApplyCredentialsPath(ts, pipeline.CredsDir)

	podbuilder := podconvert.Builder{
		Images:          c.Images,
		KubeClient:      c.KubeClientSet,
		EntrypointCache: c.entrypointCache,
	}
	pod, err := podbuilder.Build(ctx, tr, *ts,
		computeresources.NewTransformer(ctx, tr.Namespace, c.limitrangeLister),
		affinityassistant.NewTransformer(ctx, tr.Annotations),
	)
	if err != nil {
		return nil, fmt.Errorf("translating TaskSpec to Pod: %w", err)
	}

	// Stash the podname in case there's create conflict so that we can try
	// to fetch it.
	podName := pod.Name
	pod, err = c.KubeClientSet.CoreV1().Pods(tr.Namespace).Create(ctx, pod, metav1.CreateOptions{})

	if err == nil && willOverwritePodSetAffinity(tr) {
		if recorder := controller.GetEventRecorder(ctx); recorder != nil {
			recorder.Eventf(tr, corev1.EventTypeWarning, "PodAffinityOverwrite", "Pod template affinity is overwritten by affinity assistant for pod %q", pod.Name)
		}
	}
	// If the pod failed to be created because it already exists, try to fetch
	// from the informer and return if successful. Otherwise, return the
	// original error.
	if err != nil && k8serrors.IsAlreadyExists(err) {
		if p, getErr := c.podLister.Pods(tr.Namespace).Get(podName); getErr == nil {
			return p, nil
		}
	}
	return pod, err
}

// applyParamsContextsResultsAndWorkspaces applies paramater, context, results and workspace substitutions to the TaskSpec.
func applyParamsContextsResultsAndWorkspaces(ctx context.Context, tr *v1beta1.TaskRun, rtr *resources.ResolvedTaskResources, workspaceVolumes map[string]corev1.Volume) (*v1beta1.TaskSpec, error) {
	ts := rtr.TaskSpec.DeepCopy()
	var defaults []v1beta1.ParamSpec
	if len(ts.Params) > 0 {
		defaults = append(defaults, ts.Params...)
	}
	// Apply parameter substitution from the taskrun.
	ts = resources.ApplyParameters(ctx, ts, tr, defaults...)

	// Apply context substitution from the taskrun
	ts = resources.ApplyContexts(ts, rtr.TaskName, tr)

	// Apply task result substitution
	ts = resources.ApplyTaskResults(ts)

	// Apply step exitCode path substitution
	ts = resources.ApplyStepExitCodePath(ts)

	// Apply workspace resource substitution
	if config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == config.AlphaAPIFields {
		// propagate workspaces from taskrun to task.
		twn := []string{}
		for _, tw := range ts.Workspaces {
			twn = append(twn, tw.Name)
		}

		for _, trw := range tr.Spec.Workspaces {
			skip := false
			for _, tw := range twn {
				if tw == trw.Name {
					skip = true
					break
				}
			}
			if !skip {
				ts.Workspaces = append(ts.Workspaces, v1beta1.WorkspaceDeclaration{Name: trw.Name})
			}
		}
	}
	ts = resources.ApplyWorkspaces(ctx, ts, ts.Workspaces, tr.Spec.Workspaces, workspaceVolumes)

	return ts, nil
}

func isExceededResourceQuotaError(err error) bool {
	return err != nil && k8serrors.IsForbidden(err) && strings.Contains(err.Error(), "exceeded quota")
}

func isTaskRunValidationFailed(err error) bool {
	return err != nil && strings.Contains(err.Error(), "TaskRun validation failed")
}

// resourceImplBinding maps pipeline resource names to the actual resource type implementations
func resourceImplBinding(resources map[string]*resourcev1alpha1.PipelineResource, images pipeline.Images) (map[string]v1beta1.PipelineResourceInterface, error) {
	p := make(map[string]v1beta1.PipelineResourceInterface)
	for rName, r := range resources {
		i, err := resource.FromType(rName, r, images)
		if err != nil {
			return nil, fmt.Errorf("failed to create resource %s : %v with error: %w", rName, r, err)
		}
		p[rName] = i
	}
	return p, nil
}

// updateStoppedSidecarStatus updates SidecarStatus for sidecars that were
// terminated by nop image
func updateStoppedSidecarStatus(pod *corev1.Pod, tr *v1beta1.TaskRun) error {
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
	return nil
}

// applyVolumeClaimTemplates and return WorkspaceBindings were templates is translated to PersistentVolumeClaims
func applyVolumeClaimTemplates(workspaceBindings []v1beta1.WorkspaceBinding, owner metav1.OwnerReference) []v1beta1.WorkspaceBinding {
	taskRunWorkspaceBindings := make([]v1beta1.WorkspaceBinding, 0, len(workspaceBindings))
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

func storeTaskSpecAndMergeMeta(tr *v1beta1.TaskRun, ts *v1beta1.TaskSpec, meta *metav1.ObjectMeta) error {
	// Only store the TaskSpec once, if it has never been set before.
	if tr.Status.TaskSpec == nil {
		tr.Status.TaskSpec = ts
		// Propagate annotations from Task to TaskRun. TaskRun annotations take precedences over Task.
		tr.ObjectMeta.Annotations = kmap.Union(meta.Annotations, tr.ObjectMeta.Annotations)
		// Propagate labels from Task to TaskRun. TaskRun labels take precedences over Task.
		tr.ObjectMeta.Labels = kmap.Union(meta.Labels, tr.ObjectMeta.Labels)
		if tr.Spec.TaskRef != nil {
			if tr.Spec.TaskRef.Kind == "ClusterTask" {
				tr.ObjectMeta.Labels[pipeline.ClusterTaskLabelKey] = meta.Name
			} else {
				tr.ObjectMeta.Labels[pipeline.TaskLabelKey] = meta.Name
			}
		}
	}
	return nil
}

// willOverwritePodSetAffinity returns a bool indicating whether the
// affinity for pods will be overwritten with affinity assistant.
func willOverwritePodSetAffinity(taskRun *v1beta1.TaskRun) bool {
	var podTemplate pod.Template
	if taskRun.Spec.PodTemplate != nil {
		podTemplate = *taskRun.Spec.PodTemplate
	}
	return taskRun.Annotations[workspace.AnnotationAffinityAssistantName] != "" && podTemplate.Affinity != nil
}

// isResourceQuotaConflictError returns a bool indicating whether the
// k8 error is of kind resourcequotas or not
func isResourceQuotaConflictError(err error) bool {
	k8Err, ok := err.(k8serrors.APIStatus)
	if !ok {
		return false
	}
	k8ErrStatus := k8Err.Status()
	if k8ErrStatus.Reason != metav1.StatusReasonConflict {
		return false
	}
	return k8ErrStatus.Details != nil && k8ErrStatus.Details.Kind == "resourcequotas"
}
