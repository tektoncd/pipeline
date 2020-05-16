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

package pipelinerun

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	apisconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/artifacts"
	listersv1alpha1 "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	resourcelisters "github.com/tektoncd/pipeline/pkg/client/resource/listers/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/contexts"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/tracker"
)

const (
	// ReasonCouldntGetPipeline indicates that the reason for the failure status is that the
	// associated Pipeline couldn't be retrieved
	ReasonCouldntGetPipeline = "CouldntGetPipeline"
	// ReasonInvalidBindings indicates that the reason for the failure status is that the
	// PipelineResources bound in the PipelineRun didn't match those declared in the Pipeline
	ReasonInvalidBindings = "InvalidPipelineResourceBindings"
	// ReasonInvalidWorkspaceBinding indicates that a Pipeline expects a workspace but a
	// PipelineRun has provided an invalid binding.
	ReasonInvalidWorkspaceBinding = "InvalidWorkspaceBindings"
	// ReasonInvalidServiceAccountMapping indicates that PipelineRun.Spec.ServiceAccountNames defined with a wrong taskName
	ReasonInvalidServiceAccountMapping = "InvalidServiceAccountMappings"
	// ReasonParameterTypeMismatch indicates that the reason for the failure status is that
	// parameter(s) declared in the PipelineRun do not have the some declared type as the
	// parameters(s) declared in the Pipeline that they are supposed to override.
	ReasonParameterTypeMismatch = "ParameterTypeMismatch"
	// ReasonCouldntGetTask indicates that the reason for the failure status is that the
	// associated Pipeline's Tasks couldn't all be retrieved
	ReasonCouldntGetTask = "CouldntGetTask"
	// ReasonCouldntGetResource indicates that the reason for the failure status is that the
	// associated PipelineRun's bound PipelineResources couldn't all be retrieved
	ReasonCouldntGetResource = "CouldntGetResource"
	// ReasonCouldntGetCondition indicates that the reason for the failure status is that the
	// associated Pipeline's Conditions couldn't all be retrieved
	ReasonCouldntGetCondition = "CouldntGetCondition"
	// ReasonFailedValidation indicates that the reason for failure status is
	// that pipelinerun failed runtime validation
	ReasonFailedValidation = "PipelineValidationFailed"
	// ReasonInvalidGraph indicates that the reason for the failure status is that the
	// associated Pipeline is an invalid graph (a.k.a wrong order, cycle, …)
	ReasonInvalidGraph = "PipelineInvalidGraph"
	// ReasonCancelled indicates that a PipelineRun was cancelled.
	ReasonCancelled = "PipelineRunCancelled"
	// ReasonCouldntCancel indicates that a PipelineRun was cancelled but attempting to update
	// all of the running TaskRuns as cancelled failed.
	ReasonCouldntCancel = "PipelineRunCouldntCancel"

	// pipelineRunAgentName defines logging agent name for PipelineRun Controller
	pipelineRunAgentName = "pipeline-controller"

	// Event reasons
	eventReasonFailed    = "PipelineRunFailed"
	eventReasonSucceeded = "PipelineRunSucceeded"
)

type configStore interface {
	ToContext(ctx context.Context) context.Context
	WatchConfigs(w configmap.Watcher)
}

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	*reconciler.Base
	// listers index properties about resources
	pipelineRunLister listers.PipelineRunLister
	pipelineLister    listers.PipelineLister
	taskRunLister     listers.TaskRunLister
	taskLister        listers.TaskLister
	clusterTaskLister listers.ClusterTaskLister
	resourceLister    resourcelisters.PipelineResourceLister
	conditionLister   listersv1alpha1.ConditionLister
	tracker           tracker.Interface
	configStore       configStore
	timeoutHandler    *reconciler.TimeoutSet
	metrics           *Recorder
	pvcHandler        volumeclaim.PvcHandler
}

var (
	// Check that our Reconciler implements controller.Reconciler
	_ controller.Reconciler = (*Reconciler)(nil)
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Pipeline Run
// resource with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	c.Logger.Infof("Reconciling key %s at %v", key, time.Now())

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	ctx = c.configStore.ToContext(ctx)

	// Get the Pipeline Run resource with this namespace/name
	original, err := c.pipelineRunLister.PipelineRuns(namespace).Get(name)
	if errors.IsNotFound(err) {
		// The resource no longer exists, in which case we stop processing.
		c.Logger.Errorf("pipeline run %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informer's copy.
	pr := original.DeepCopy()
	if !pr.HasStarted() {
		pr.Status.InitializeConditions()
		// In case node time was not synchronized, when controller has been scheduled to other nodes.
		if pr.Status.StartTime.Sub(pr.CreationTimestamp.Time) < 0 {
			c.Logger.Warnf("PipelineRun %s createTimestamp %s is after the pipelineRun started %s", pr.GetRunKey(), pr.CreationTimestamp, pr.Status.StartTime)
			pr.Status.StartTime = &pr.CreationTimestamp
		}
		// start goroutine to track pipelinerun timeout only startTime is not set
		go c.timeoutHandler.WaitPipelineRun(pr, pr.Status.StartTime)
	} else {
		pr.Status.InitializeConditions()
	}

	// In case of reconcile errors, we store the error in a multierror, attempt
	// to update, and return the original error combined with any update error
	var merr *multierror.Error

	switch {
	case pr.IsDone():
		// We may be reading a version of the object that was stored at an older version
		// and may not have had all of the assumed default specified.
		pr.SetDefaults(contexts.WithUpgradeViaDefaulting(ctx))

		c.updatePipelineResults(ctx, pr)
		if err := artifacts.CleanupArtifactStorage(pr, c.KubeClientSet, c.Logger); err != nil {
			c.Logger.Errorf("Failed to delete PVC for PipelineRun %s: %v", pr.Name, err)
			return err
		}
		if err := c.cleanupAffinityAssistants(pr); err != nil {
			c.Logger.Errorf("Failed to delete StatefulSet for PipelineRun %s: %v", pr.Name, err)
			return err
		}
		c.timeoutHandler.Release(pr)
		if err := c.updateTaskRunsStatusDirectly(pr); err != nil {
			c.Logger.Errorf("Failed to update TaskRun status for PipelineRun %s: %v", pr.Name, err)
			return err
		}
		go func(metrics *Recorder) {
			err := metrics.DurationAndCount(pr)
			if err != nil {
				c.Logger.Warnf("Failed to log the metrics : %v", err)
			}
		}(c.metrics)
	case pr.IsCancelled():
		// If the pipelinerun is cancelled, cancel tasks and update status
		before := pr.Status.GetCondition(apis.ConditionSucceeded)
		merr = multierror.Append(merr, cancelPipelineRun(c.Logger, pr, c.PipelineClientSet))
		after := pr.Status.GetCondition(apis.ConditionSucceeded)
		events.Emit(c.Recorder, before, after, pr)
	default:
		if err := c.tracker.Track(pr.GetTaskRunRef(), pr); err != nil {
			c.Logger.Errorf("Failed to create tracker for TaskRuns for PipelineRun %s: %v", pr.Name, err)
			c.Recorder.Event(pr, corev1.EventTypeWarning, eventReasonFailed, "Failed to create tracker for TaskRuns for PipelineRun")
			return err
		}

		// Make sure that the PipelineRun status is in sync with the actual TaskRuns
		err = c.updatePipelineRunStatusFromInformer(pr)
		if err != nil {
			// This should not fail. Return the error so we can re-try later.
			c.Logger.Errorf("Error while syncing the pipelinerun status: %v", err.Error())
			return err
		}

		// Reconcile this copy of the pipelinerun and then write back any status or label
		// updates regardless of whether the reconciliation errored out.
		if err = c.reconcile(ctx, pr); err != nil {
			c.Logger.Errorf("Reconcile error: %v", err.Error())
			merr = multierror.Append(merr, err)
		}
	}

	var updated bool
	if !equality.Semantic.DeepEqual(original.Status, pr.Status) {
		if _, err := c.updateStatus(pr); err != nil {
			c.Logger.Warn("Failed to update PipelineRun status", zap.Error(err))
			c.Recorder.Event(pr, corev1.EventTypeWarning, eventReasonFailed, "PipelineRun failed to update")
			return multierror.Append(merr, err)
		}
		updated = true
	}

	// When we update the status only, we use updateStatus to minimize the chances of
	// racing any clients updating other parts of the Run, e.g. the spec or the labels.
	// If we need to update the labels or annotations, we need to call Update with these
	// changes explicitly.
	if !reflect.DeepEqual(original.ObjectMeta.Labels, pr.ObjectMeta.Labels) || !reflect.DeepEqual(original.ObjectMeta.Annotations, pr.ObjectMeta.Annotations) {
		if _, err := c.updateLabelsAndAnnotations(pr); err != nil {
			c.Logger.Warn("Failed to update PipelineRun labels/annotations", zap.Error(err))
			c.Recorder.Event(pr, corev1.EventTypeWarning, eventReasonFailed, "PipelineRun failed to update labels/annotations")
			return multierror.Append(merr, err)
		}
		updated = true
	}

	if updated {
		go func(metrics *Recorder) {
			err := metrics.RunningPipelineRuns(c.pipelineRunLister)
			if err != nil {
				c.Logger.Warnf("Failed to log the metrics : %v", err)
			}
		}(c.metrics)
	}

	return merr.ErrorOrNil()
}

func (c *Reconciler) updatePipelineResults(ctx context.Context, pr *v1beta1.PipelineRun) {

	// TODO: Use factory func instead of hard-coding this once OCI images are supported.
	resolver := &resources.LocalPipelineRefResolver{
		Namespace:    pr.Namespace,
		Tektonclient: c.PipelineClientSet,
	}
	_, pipelineSpec, err := resources.GetPipelineData(ctx, pr, resolver.GetPipeline)
	if err != nil {
		if ce, ok := err.(*v1beta1.CannotConvertError); ok {
			pr.Status.MarkResourceNotConvertible(ce)
			return
		}
		c.Logger.Errorf("Failed to determine Pipeline spec to use for pipelinerun %s: %v", pr.Name, err)
		pr.Status.MarkFailed(ReasonCouldntGetPipeline,
			"Error retrieving pipeline for pipelinerun %s/%s: %s",
			pr.Namespace, pr.Name, err)
		return
	}
	resolvedResultRefs := resources.ResolvePipelineResultRefs(pr.Status, pipelineSpec.Results)
	pr.Status.PipelineResults = getPipelineRunResults(pipelineSpec, resolvedResultRefs)
}
func (c *Reconciler) reconcile(ctx context.Context, pr *v1beta1.PipelineRun) error {
	// We may be reading a version of the object that was stored at an older version
	// and may not have had all of the assumed default specified.
	pr.SetDefaults(contexts.WithUpgradeViaDefaulting(ctx))

	// TODO: Use factory func instead of hard-coding this once OCI images are supported.
	resolver := &resources.LocalPipelineRefResolver{
		Namespace:    pr.Namespace,
		Tektonclient: c.PipelineClientSet,
	}
	pipelineMeta, pipelineSpec, err := resources.GetPipelineData(ctx, pr, resolver.GetPipeline)
	if err != nil {
		if ce, ok := err.(*v1beta1.CannotConvertError); ok {
			pr.Status.MarkResourceNotConvertible(ce)
			return nil
		}
		c.Logger.Errorf("Failed to determine Pipeline spec to use for pipelinerun %s: %v", pr.Name, err)
		pr.Status.MarkFailed(ReasonCouldntGetPipeline,
			"Error retrieving pipeline for pipelinerun %s/%s: %s",
			pr.Namespace, pr.Name, err)
		return nil
	}

	// Store the fetched PipelineSpec on the PipelineRun for auditing
	if err := storePipelineSpec(ctx, pr, pipelineSpec); err != nil {
		c.Logger.Errorf("Failed to store PipelineSpec on PipelineRun.Status for pipelinerun %s: %v", pr.Name, err)
	}

	// Propagate labels from Pipeline to PipelineRun.
	if pr.ObjectMeta.Labels == nil {
		pr.ObjectMeta.Labels = make(map[string]string, len(pipelineMeta.Labels)+1)
	}
	for key, value := range pipelineMeta.Labels {
		pr.ObjectMeta.Labels[key] = value
	}
	pr.ObjectMeta.Labels[pipeline.GroupName+pipeline.PipelineLabelKey] = pipelineMeta.Name

	// Propagate annotations from Pipeline to PipelineRun.
	if pr.ObjectMeta.Annotations == nil {
		pr.ObjectMeta.Annotations = make(map[string]string, len(pipelineMeta.Annotations))
	}
	for key, value := range pipelineMeta.Annotations {
		pr.ObjectMeta.Annotations[key] = value
	}

	d, err := dag.Build(v1beta1.PipelineTaskList(pipelineSpec.Tasks))
	if err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(ReasonInvalidGraph,
			"PipelineRun %s/%s's Pipeline DAG is invalid: %s",
			pr.Namespace, pr.Name, err)
		return nil
	}

	if err := pipelineSpec.Validate(ctx); err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(ReasonFailedValidation,
			"Pipeline %s/%s can't be Run; it has an invalid spec: %s",
			pipelineMeta.Namespace, pipelineMeta.Name, err)
		return nil
	}

	if err := resources.ValidateResourceBindings(pipelineSpec, pr); err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(ReasonInvalidBindings,
			"PipelineRun %s/%s doesn't bind Pipeline %s/%s's PipelineResources correctly: %s",
			pr.Namespace, pr.Name, pr.Namespace, pipelineMeta.Name, err)
		return nil
	}
	providedResources, err := resources.GetResourcesFromBindings(pr, c.resourceLister.PipelineResources(pr.Namespace).Get)
	if err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(ReasonCouldntGetResource,
			"PipelineRun %s/%s can't be Run; it tries to bind Resources that don't exist: %s",
			pipelineMeta.Namespace, pr.Name, err)
		return nil
	}

	// Ensure that the parameters from the PipelineRun are overriding Pipeline parameters with the same type.
	// Weird substitution issues can occur if this is not validated (ApplyParameters() does not verify type).
	err = resources.ValidateParamTypesMatching(pipelineSpec, pr)
	if err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(ReasonParameterTypeMismatch,
			"PipelineRun %s/%s parameters have mismatching types with Pipeline %s/%s's parameters: %s",
			pr.Namespace, pr.Name, pr.Namespace, pipelineMeta.Name, err)
		return nil
	}

	// Ensure that the workspaces expected by the Pipeline are provided by the PipelineRun.
	if err := resources.ValidateWorkspaceBindings(pipelineSpec, pr); err != nil {
		pr.Status.MarkFailed(ReasonInvalidWorkspaceBinding,
			"PipelineRun %s/%s doesn't bind Pipeline %s/%s's Workspaces correctly: %s",
			pr.Namespace, pr.Name, pr.Namespace, pipelineMeta.Name, err)
		return nil
	}

	// Ensure that the ServiceAccountNames defined correct.
	if err := resources.ValidateServiceaccountMapping(pipelineSpec, pr); err != nil {
		pr.Status.MarkFailed(ReasonInvalidServiceAccountMapping,
			"PipelineRun %s/%s doesn't define ServiceAccountNames correctly: %s",
			pr.Namespace, pr.Name, err)
		return nil
	}

	// Apply parameter substitution from the PipelineRun
	pipelineSpec = resources.ApplyParameters(pipelineSpec, pr)

	pipelineState, err := resources.ResolvePipelineRun(ctx,
		*pr,
		func(name string) (v1beta1.TaskInterface, error) {
			return c.taskLister.Tasks(pr.Namespace).Get(name)
		},
		func(name string) (*v1beta1.TaskRun, error) {
			return c.taskRunLister.TaskRuns(pr.Namespace).Get(name)
		},
		func(name string) (v1beta1.TaskInterface, error) {
			return c.clusterTaskLister.Get(name)
		},
		func(name string) (*v1alpha1.Condition, error) {
			return c.conditionLister.Conditions(pr.Namespace).Get(name)
		},
		pipelineSpec.Tasks, providedResources,
	)

	if err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		switch err := err.(type) {
		case *resources.TaskNotFoundError:
			pr.Status.MarkFailed(ReasonCouldntGetTask,
				"Pipeline %s/%s can't be Run; it contains Tasks that don't exist: %s",
				pipelineMeta.Namespace, pipelineMeta.Name, err)
		case *resources.ConditionNotFoundError:
			pr.Status.MarkFailed(ReasonCouldntGetCondition,
				"PipelineRun %s/%s can't be Run; it contains Conditions that don't exist:  %s",
				pipelineMeta.Namespace, pr.Name, err)
		default:
			pr.Status.MarkFailed(ReasonFailedValidation,
				"PipelineRun %s/%s can't be Run; couldn't resolve all references: %s",
				pipelineMeta.Namespace, pr.Name, err)
		}
		return nil
	}

	if pipelineState.IsDone() && pr.IsDone() {
		c.timeoutHandler.Release(pr)
		c.Recorder.Event(pr, corev1.EventTypeNormal, eventReasonSucceeded, "PipelineRun completed successfully.")
		return nil
	}

	for _, rprt := range pipelineState {
		err := taskrun.ValidateResolvedTaskResources(rprt.PipelineTask.Params, rprt.ResolvedTaskResources)
		if err != nil {
			c.Logger.Errorf("Failed to validate pipelinerun %q with error %v", pr.Name, err)
			pr.Status.MarkFailed(ReasonFailedValidation, err.Error())
			return nil
		}
	}

	if pipelineState.IsBeforeFirstTaskRun() {
		if pr.HasVolumeClaimTemplate() {
			// create workspace PVC from template
			if err = c.pvcHandler.CreatePersistentVolumeClaimsForWorkspaces(pr.Spec.Workspaces, pr.GetOwnerReference(), pr.Namespace); err != nil {
				c.Logger.Errorf("Failed to create PVC for PipelineRun %s: %v", pr.Name, err)
				pr.Status.MarkFailed(volumeclaim.ReasonCouldntCreateWorkspacePVC,
					"Failed to create PVC for PipelineRun %s/%s Workspaces correctly: %s",
					pr.Namespace, pr.Name, err)
				return nil
			}
		}

		if !c.isAffinityAssistantDisabled(ctx) {
			// create Affinity Assistant (StatefulSet) so that taskRun pods that share workspace PVC achieve Node Affinity
			if err = c.createAffinityAssistants(pr.Spec.Workspaces, pr, pr.Namespace); err != nil {
				c.Logger.Errorf("Failed to create affinity assistant StatefulSet for PipelineRun %s: %v", pr.Name, err)
				pr.Status.MarkFailed(ReasonCouldntCreateAffinityAssistantStatefulSet,
					"Failed to create StatefulSet for PipelineRun %s/%s correctly: %s",
					pr.Namespace, pr.Name, err)
				return nil
			}
		}
	}

	candidateTasks, err := dag.GetSchedulable(d, pipelineState.SuccessfulPipelineTaskNames()...)
	if err != nil {
		c.Logger.Errorf("Error getting potential next tasks for valid pipelinerun %s: %v", pr.Name, err)
	}

	nextRprts := pipelineState.GetNextTasks(candidateTasks)
	resolvedResultRefs, err := resources.ResolveResultRefs(pipelineState, nextRprts)
	if err != nil {
		c.Logger.Infof("Failed to resolve all task params for %q with error %v", pr.Name, err)
		pr.Status.MarkFailed(ReasonFailedValidation, err.Error())
		return nil
	}
	resources.ApplyTaskResults(nextRprts, resolvedResultRefs)

	var as artifacts.ArtifactStorageInterface

	if as, err = artifacts.InitializeArtifactStorage(c.Images, pr, pipelineSpec, c.KubeClientSet, c.Logger); err != nil {
		c.Logger.Infof("PipelineRun failed to initialize artifact storage %s", pr.Name)
		return err
	}

	for _, rprt := range nextRprts {
		if rprt == nil {
			continue
		}
		if rprt.ResolvedConditionChecks == nil || rprt.ResolvedConditionChecks.IsSuccess() {
			rprt.TaskRun, err = c.createTaskRun(ctx, rprt, pr, as.StorageBasePath(pr))
			if err != nil {
				c.Recorder.Eventf(pr, corev1.EventTypeWarning, "TaskRunCreationFailed", "Failed to create TaskRun %q: %v", rprt.TaskRunName, err)
				return fmt.Errorf("error creating TaskRun called %s for PipelineTask %s from PipelineRun %s: %w", rprt.TaskRunName, rprt.PipelineTask.Name, pr.Name, err)
			}
		} else if !rprt.ResolvedConditionChecks.HasStarted() {
			for _, rcc := range rprt.ResolvedConditionChecks {
				rcc.ConditionCheck, err = c.makeConditionCheckContainer(rprt, rcc, pr)
				if err != nil {
					c.Recorder.Eventf(pr, corev1.EventTypeWarning, "ConditionCheckCreationFailed", "Failed to create TaskRun %q: %v", rcc.ConditionCheckName, err)
					return fmt.Errorf("error creating ConditionCheck container called %s for PipelineTask %s from PipelineRun %s: %w", rcc.ConditionCheckName, rprt.PipelineTask.Name, pr.Name, err)
				}
			}
		}
	}
	before := pr.Status.GetCondition(apis.ConditionSucceeded)
	after := resources.GetPipelineConditionStatus(pr, pipelineState, c.Logger, d)
	switch after.Status {
	case corev1.ConditionTrue:
		pr.Status.MarkSucceeded(after.Reason, after.Message)
	case corev1.ConditionFalse:
		pr.Status.MarkFailed(after.Reason, after.Message)
	case corev1.ConditionUnknown:
		pr.Status.MarkRunning(after.Reason, after.Message)
	}
	events.Emit(c.Recorder, before, after, pr)

	pr.Status.TaskRuns = getTaskRunsStatus(pr, pipelineState)
	c.Logger.Infof("PipelineRun %s status is being set to %s", pr.Name, after)
	return nil
}

func getPipelineRunResults(pipelineSpec *v1beta1.PipelineSpec, resolvedResultRefs resources.ResolvedResultRefs) []v1beta1.PipelineRunResult {
	var results []v1beta1.PipelineRunResult
	stringReplacements := map[string]string{}

	for _, resolvedResultRef := range resolvedResultRefs {
		replaceTarget := fmt.Sprintf("%s.%s.%s.%s", v1beta1.ResultTaskPart, resolvedResultRef.ResultReference.PipelineTask, v1beta1.ResultResultPart, resolvedResultRef.ResultReference.Result)
		stringReplacements[replaceTarget] = resolvedResultRef.Value.StringVal
	}
	for _, result := range pipelineSpec.Results {
		in := result.Value
		for k, v := range stringReplacements {
			in = strings.Replace(in, fmt.Sprintf("$(%s)", k), v, -1)
		}
		results = append(results, v1beta1.PipelineRunResult{
			Name:  result.Name,
			Value: in,
		})
	}
	return results
}

func getTaskRunsStatus(pr *v1beta1.PipelineRun, state []*resources.ResolvedPipelineRunTask) map[string]*v1beta1.PipelineRunTaskRunStatus {
	status := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	for _, rprt := range state {
		if rprt.TaskRun == nil && rprt.ResolvedConditionChecks == nil {
			continue
		}

		var prtrs *v1beta1.PipelineRunTaskRunStatus
		if rprt.TaskRun != nil {
			prtrs = pr.Status.TaskRuns[rprt.TaskRun.Name]
		}
		if prtrs == nil {
			prtrs = &v1beta1.PipelineRunTaskRunStatus{
				PipelineTaskName: rprt.PipelineTask.Name,
			}
		}

		if rprt.TaskRun != nil {
			prtrs.Status = &rprt.TaskRun.Status
		}

		if len(rprt.ResolvedConditionChecks) > 0 {
			cStatus := make(map[string]*v1beta1.PipelineRunConditionCheckStatus)
			for _, c := range rprt.ResolvedConditionChecks {
				cStatus[c.ConditionCheckName] = &v1beta1.PipelineRunConditionCheckStatus{
					ConditionName: c.ConditionRegisterName,
				}
				if c.ConditionCheck != nil {
					cStatus[c.ConditionCheckName].Status = c.NewConditionCheckStatus()
				}
			}
			prtrs.ConditionChecks = cStatus
			if rprt.ResolvedConditionChecks.IsDone() && !rprt.ResolvedConditionChecks.IsSuccess() {
				if prtrs.Status == nil {
					prtrs.Status = &v1beta1.TaskRunStatus{}
				}
				prtrs.Status.SetCondition(&apis.Condition{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Reason:  resources.ReasonConditionCheckFailed,
					Message: fmt.Sprintf("ConditionChecks failed for Task %s in PipelineRun %s", rprt.TaskRunName, pr.Name),
				})
			}
		}
		status[rprt.TaskRunName] = prtrs
	}
	return status
}

func (c *Reconciler) updateTaskRunsStatusDirectly(pr *v1beta1.PipelineRun) error {
	for taskRunName := range pr.Status.TaskRuns {
		// TODO(dibyom): Add conditionCheck statuses here
		prtrs := pr.Status.TaskRuns[taskRunName]
		tr, err := c.taskRunLister.TaskRuns(pr.Namespace).Get(taskRunName)
		if err != nil {
			// If the TaskRun isn't found, it just means it won't be run
			if !errors.IsNotFound(err) {
				return fmt.Errorf("error retrieving TaskRun %s: %w", taskRunName, err)
			}
		} else {
			prtrs.Status = &tr.Status
		}
	}
	return nil
}

func (c *Reconciler) createTaskRun(ctx context.Context, rprt *resources.ResolvedPipelineRunTask, pr *v1beta1.PipelineRun, storageBasePath string) (*v1beta1.TaskRun, error) {
	tr, _ := c.taskRunLister.TaskRuns(pr.Namespace).Get(rprt.TaskRunName)
	if tr != nil {
		//is a retry
		addRetryHistory(tr)
		clearStatus(tr)
		tr.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		})
		return c.PipelineClientSet.TektonV1beta1().TaskRuns(pr.Namespace).UpdateStatus(tr)
	}

	serviceAccountName, podTemplate := pr.GetTaskRunSpecs(rprt.PipelineTask.Name)
	tr = &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:            rprt.TaskRunName,
			Namespace:       pr.Namespace,
			OwnerReferences: []metav1.OwnerReference{pr.GetOwnerReference()},
			Labels:          getTaskrunLabels(pr, rprt.PipelineTask.Name),
			Annotations:     getTaskrunAnnotations(pr),
		},
		Spec: v1beta1.TaskRunSpec{
			Params:             rprt.PipelineTask.Params,
			ServiceAccountName: serviceAccountName,
			Timeout:            getTaskRunTimeout(pr, rprt),
			PodTemplate:        podTemplate,
		}}

	if rprt.ResolvedTaskResources.TaskName != "" {
		tr.Spec.TaskRef = &v1beta1.TaskRef{
			Name: rprt.ResolvedTaskResources.TaskName,
			Kind: rprt.ResolvedTaskResources.Kind,
		}
	} else if rprt.ResolvedTaskResources.TaskSpec != nil {
		tr.Spec.TaskSpec = rprt.ResolvedTaskResources.TaskSpec
	}

	var pipelinePVCWorkspaceName string
	pipelineRunWorkspaces := make(map[string]v1beta1.WorkspaceBinding)
	for _, binding := range pr.Spec.Workspaces {
		pipelineRunWorkspaces[binding.Name] = binding
	}
	for _, ws := range rprt.PipelineTask.Workspaces {
		taskWorkspaceName, pipelineTaskSubPath, pipelineWorkspaceName := ws.Name, ws.SubPath, ws.Workspace
		if b, hasBinding := pipelineRunWorkspaces[pipelineWorkspaceName]; hasBinding {
			if b.PersistentVolumeClaim != nil || b.VolumeClaimTemplate != nil {
				pipelinePVCWorkspaceName = pipelineWorkspaceName
			}
			tr.Spec.Workspaces = append(tr.Spec.Workspaces, taskWorkspaceByWorkspaceVolumeSource(b, taskWorkspaceName, pipelineTaskSubPath, pr.GetOwnerReference()))
		} else {
			return nil, fmt.Errorf("expected workspace %q to be provided by pipelinerun for pipeline task %q", pipelineWorkspaceName, rprt.PipelineTask.Name)
		}
	}

	if !c.isAffinityAssistantDisabled(ctx) && pipelinePVCWorkspaceName != "" {
		tr.Annotations[workspace.AnnotationAffinityAssistantName] = getAffinityAssistantName(pipelinePVCWorkspaceName, pr.GetOwnerReference())
	}

	resources.WrapSteps(&tr.Spec, rprt.PipelineTask, rprt.ResolvedTaskResources.Inputs, rprt.ResolvedTaskResources.Outputs, storageBasePath)
	c.Logger.Infof("Creating a new TaskRun object %s", rprt.TaskRunName)
	return c.PipelineClientSet.TektonV1beta1().TaskRuns(pr.Namespace).Create(tr)
}

// taskWorkspaceByWorkspaceVolumeSource is returning the WorkspaceBinding with the TaskRun specified name.
// If the volume source is a volumeClaimTemplate, the template is applied and passed to TaskRun as a persistentVolumeClaim
func taskWorkspaceByWorkspaceVolumeSource(wb v1beta1.WorkspaceBinding, taskWorkspaceName string, pipelineTaskSubPath string, owner metav1.OwnerReference) v1beta1.WorkspaceBinding {
	if wb.VolumeClaimTemplate == nil {
		binding := *wb.DeepCopy()
		binding.Name = taskWorkspaceName
		binding.SubPath = combinedSubPath(wb.SubPath, pipelineTaskSubPath)
		return binding
	}

	// apply template
	binding := v1beta1.WorkspaceBinding{
		SubPath: combinedSubPath(wb.SubPath, pipelineTaskSubPath),
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: volumeclaim.GetPersistentVolumeClaimName(wb.VolumeClaimTemplate, wb, owner),
		},
	}
	binding.Name = taskWorkspaceName
	return binding
}

// combinedSubPath returns the combined value of the optional subPath from workspaceBinding and the optional
// subPath from pipelineTask. If both is set, they are joined with a slash.
func combinedSubPath(workspaceSubPath string, pipelineTaskSubPath string) string {
	if workspaceSubPath == "" {
		return pipelineTaskSubPath
	} else if pipelineTaskSubPath == "" {
		return workspaceSubPath
	}
	return filepath.Join(workspaceSubPath, pipelineTaskSubPath)
}

func addRetryHistory(tr *v1beta1.TaskRun) {
	newStatus := *tr.Status.DeepCopy()
	newStatus.RetriesStatus = nil
	tr.Status.RetriesStatus = append(tr.Status.RetriesStatus, newStatus)
}

func clearStatus(tr *v1beta1.TaskRun) {
	tr.Status.StartTime = nil
	tr.Status.CompletionTime = nil
	tr.Status.PodName = ""
}

func getTaskrunAnnotations(pr *v1beta1.PipelineRun) map[string]string {
	// Propagate annotations from PipelineRun to TaskRun.
	annotations := make(map[string]string, len(pr.ObjectMeta.Annotations)+1)
	for key, val := range pr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	return annotations
}

func getTaskrunLabels(pr *v1beta1.PipelineRun, pipelineTaskName string) map[string]string {
	// Propagate labels from PipelineRun to TaskRun.
	labels := make(map[string]string, len(pr.ObjectMeta.Labels)+1)
	for key, val := range pr.ObjectMeta.Labels {
		labels[key] = val
	}
	labels[pipeline.GroupName+pipeline.PipelineRunLabelKey] = pr.Name
	if pipelineTaskName != "" {
		labels[pipeline.GroupName+pipeline.PipelineTaskLabelKey] = pipelineTaskName
	}
	return labels
}

func getTaskRunTimeout(pr *v1beta1.PipelineRun, rprt *resources.ResolvedPipelineRunTask) *metav1.Duration {
	var taskRunTimeout = &metav1.Duration{Duration: apisconfig.NoTimeoutDuration}

	var timeout time.Duration
	if pr.Spec.Timeout == nil {
		timeout = config.DefaultTimeoutMinutes * time.Minute
	} else {
		timeout = pr.Spec.Timeout.Duration
	}

	// If the value of the timeout is 0 for any resource, there is no timeout.
	// It is impossible for pr.Spec.Timeout to be nil, since SetDefault always assigns it with a value.
	if timeout != apisconfig.NoTimeoutDuration {
		pTimeoutTime := pr.Status.StartTime.Add(timeout)
		if time.Now().After(pTimeoutTime) {
			// Just in case something goes awry and we're creating the TaskRun after it should have already timed out,
			// set the timeout to 1 second.
			taskRunTimeout = &metav1.Duration{Duration: time.Until(pTimeoutTime)}
			if taskRunTimeout.Duration < 0 {
				taskRunTimeout = &metav1.Duration{Duration: 1 * time.Second}
			}
		} else {
			// check if PipelineTask has a timeout specified
			if rprt.PipelineTask.Timeout != nil {
				taskRunTimeout = &metav1.Duration{Duration: rprt.PipelineTask.Timeout.Duration}
			} else {
				taskRunTimeout = &metav1.Duration{Duration: timeout}
			}
		}
	}

	// check if PipelineTask has a timeout specified even if PipelineRun doesn't have a timeout
	if timeout == apisconfig.NoTimeoutDuration && rprt.PipelineTask.Timeout != nil {
		taskRunTimeout = &metav1.Duration{Duration: rprt.PipelineTask.Timeout.Duration}
	}

	return taskRunTimeout
}

func (c *Reconciler) updateStatus(pr *v1beta1.PipelineRun) (*v1beta1.PipelineRun, error) {
	newPr, err := c.pipelineRunLister.PipelineRuns(pr.Namespace).Get(pr.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting PipelineRun %s when updating status: %w", pr.Name, err)
	}
	if !reflect.DeepEqual(pr.Status, newPr.Status) {
		newPr = newPr.DeepCopy() // Don't modify the informer's copy
		newPr.Status = pr.Status
		return c.PipelineClientSet.TektonV1beta1().PipelineRuns(pr.Namespace).UpdateStatus(newPr)
	}
	return newPr, nil
}

func (c *Reconciler) updateLabelsAndAnnotations(pr *v1beta1.PipelineRun) (*v1beta1.PipelineRun, error) {
	newPr, err := c.pipelineRunLister.PipelineRuns(pr.Namespace).Get(pr.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting PipelineRun %s when updating labels/annotations: %w", pr.Name, err)
	}
	if !reflect.DeepEqual(pr.ObjectMeta.Labels, newPr.ObjectMeta.Labels) || !reflect.DeepEqual(pr.ObjectMeta.Annotations, newPr.ObjectMeta.Annotations) {
		mergePatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels":      pr.ObjectMeta.Labels,
				"annotations": pr.ObjectMeta.Annotations,
			},
		}
		patch, err := json.Marshal(mergePatch)
		if err != nil {
			return nil, err
		}
		return c.PipelineClientSet.TektonV1beta1().PipelineRuns(pr.Namespace).Patch(pr.Name, types.MergePatchType, patch)
	}
	return newPr, nil
}

func (c *Reconciler) makeConditionCheckContainer(rprt *resources.ResolvedPipelineRunTask, rcc *resources.ResolvedConditionCheck, pr *v1beta1.PipelineRun) (*v1beta1.ConditionCheck, error) {
	labels := getTaskrunLabels(pr, rprt.PipelineTask.Name)
	labels[pipeline.GroupName+pipeline.ConditionCheckKey] = rcc.ConditionCheckName
	labels[pipeline.GroupName+pipeline.ConditionNameKey] = rcc.Condition.Name

	for key, value := range rcc.Condition.ObjectMeta.Labels {
		labels[key] = value
	}

	// Propagate annotations from PipelineRun to TaskRun.
	annotations := getTaskrunAnnotations(pr)

	for key, value := range rcc.Condition.ObjectMeta.Annotations {
		annotations[key] = value
	}

	taskSpec, err := rcc.ConditionToTaskSpec()
	if err != nil {
		return nil, fmt.Errorf("failed to get TaskSpec from Condition: %w", err)
	}
	serviceAccountName, podTemplate := pr.GetTaskRunSpecs(rprt.PipelineTask.Name)
	tr := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:            rcc.ConditionCheckName,
			Namespace:       pr.Namespace,
			OwnerReferences: []metav1.OwnerReference{pr.GetOwnerReference()},
			Labels:          labels,
			Annotations:     annotations,
		},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec:           taskSpec,
			ServiceAccountName: serviceAccountName,
			Params:             rcc.PipelineTaskCondition.Params,
			Resources: &v1beta1.TaskRunResources{
				Inputs: rcc.ToTaskResourceBindings(),
			},
			Timeout:     getTaskRunTimeout(pr, rprt),
			PodTemplate: podTemplate,
		}}

	cctr, err := c.PipelineClientSet.TektonV1beta1().TaskRuns(pr.Namespace).Create(tr)
	cc := v1beta1.ConditionCheck(*cctr)
	return &cc, err
}

func storePipelineSpec(ctx context.Context, pr *v1beta1.PipelineRun, ps *v1beta1.PipelineSpec) error {
	// Only store the PipelineSpec once, if it has never been set before.
	if pr.Status.PipelineSpec == nil {
		pr.Status.PipelineSpec = ps
	}
	return nil
}

func (c *Reconciler) updatePipelineRunStatusFromInformer(pr *v1beta1.PipelineRun) error {
	pipelineRunLabels := getTaskrunLabels(pr, "")
	taskRuns, err := c.taskRunLister.TaskRuns(pr.Namespace).List(labels.SelectorFromSet(pipelineRunLabels))
	if err != nil {
		c.Logger.Errorf("could not list TaskRuns %#v", err)
		return err
	}
	pr.Status = updatePipelineRunStatusFromTaskRuns(c.Logger, pr.Name, pr.Status, taskRuns)
	return nil
}

func updatePipelineRunStatusFromTaskRuns(logger *zap.SugaredLogger, prName string, prStatus v1beta1.PipelineRunStatus, trs []*v1beta1.TaskRun) v1beta1.PipelineRunStatus {
	// If no TaskRun was found, nothing to be done. We never remove taskruns from the status
	if trs == nil || len(trs) == 0 {
		return prStatus
	}
	// Store a list of Condition TaskRuns for each PipelineTask (by name)
	conditionTaskRuns := make(map[string][]*v1beta1.TaskRun)
	// Map PipelineTask names to TaskRun names that were already in the status
	taskRunByPipelineTask := make(map[string]string)
	if prStatus.TaskRuns != nil {
		for taskRunName, pipelineRunTaskRunStatus := range prStatus.TaskRuns {
			taskRunByPipelineTask[pipelineRunTaskRunStatus.PipelineTaskName] = taskRunName
		}
	}
	// Loop over all the TaskRuns associated to Tasks
	for _, taskrun := range trs {
		lbls := taskrun.GetLabels()
		pipelineTaskName := lbls[pipeline.GroupName+pipeline.PipelineTaskLabelKey]
		if _, ok := lbls[pipeline.GroupName+pipeline.ConditionCheckKey]; ok {
			// Save condition for looping over them after this
			if _, ok := conditionTaskRuns[pipelineTaskName]; !ok {
				// If it's the first condition taskrun, initialise the slice
				conditionTaskRuns[pipelineTaskName] = []*v1beta1.TaskRun{}
			}
			conditionTaskRuns[pipelineTaskName] = append(conditionTaskRuns[pipelineTaskName], taskrun)
			continue
		}
		if _, ok := prStatus.TaskRuns[taskrun.Name]; !ok {
			// This taskrun was missing from the status.
			// Add it without conditions, which are handled in the next loop
			prStatus.TaskRuns[taskrun.Name] = &v1beta1.PipelineRunTaskRunStatus{
				PipelineTaskName: pipelineTaskName,
				Status:           &taskrun.Status,
				ConditionChecks:  nil,
			}
			// Since this was recovered now, add it to the map, or it might be overwritten
			taskRunByPipelineTask[pipelineTaskName] = taskrun.Name
		}
	}
	// Then loop by pipelinetask name over all the TaskRuns associated to Conditions
	for pipelineTaskName, actualConditionTaskRuns := range conditionTaskRuns {
		taskRunName, ok := taskRunByPipelineTask[pipelineTaskName]
		if !ok {
			// The pipelineTask associated to the conditions was not found in the pipelinerun
			// status. This means that the conditions were orphaned, and never added to the
			// status. In this case we need to generate a new TaskRun name, that will be used
			// to run the TaskRun if the conditions are passed.
			taskRunName = resources.GetTaskRunName(prStatus.TaskRuns, pipelineTaskName, prName)
			prStatus.TaskRuns[taskRunName] = &v1beta1.PipelineRunTaskRunStatus{
				PipelineTaskName: pipelineTaskName,
				Status:           nil,
				ConditionChecks:  nil,
			}
		}
		// Build the map of condition checks for the taskrun
		// If there were no other condition, initialise the map
		conditionChecks := prStatus.TaskRuns[taskRunName].ConditionChecks
		if conditionChecks == nil {
			conditionChecks = make(map[string]*v1beta1.PipelineRunConditionCheckStatus)
		}
		for i, foundTaskRun := range actualConditionTaskRuns {
			lbls := foundTaskRun.GetLabels()
			if _, ok := conditionChecks[foundTaskRun.Name]; !ok {
				// The condition check was not found, so we need to add it
				// We only add the condition name, the status can now be gathered by the
				// normal reconcile process
				if conditionName, ok := lbls[pipeline.GroupName+pipeline.ConditionNameKey]; ok {
					conditionChecks[foundTaskRun.Name] = &v1beta1.PipelineRunConditionCheckStatus{
						ConditionName: fmt.Sprintf("%s-%s", conditionName, strconv.Itoa(i)),
					}
				} else {
					// The condition name label is missing, so we cannot recover this
					logger.Warnf("found an orphaned condition taskrun %#v with missing %s label",
						foundTaskRun, pipeline.ConditionNameKey)
				}
			}
		}
		prStatus.TaskRuns[taskRunName].ConditionChecks = conditionChecks
	}
	return prStatus
}
