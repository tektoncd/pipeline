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
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	apisconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/artifacts"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// pipelineRunAgentName defines logging agent name for PipelineRun Controller
	pipelineRunAgentName = "pipeline-controller"
	// pipelineRunControllerName defines name for PipelineRun Controller
	pipelineRunControllerName = "PipelineRun"

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
	resourceLister    listers.PipelineResourceLister
	conditionLister   listers.ConditionLister
	tracker           tracker.Interface
	configStore       configStore
	timeoutHandler    *reconciler.TimeoutSet
	metrics           *Recorder
}

var (
	// Check that our Reconciler implements controller.Reconciler
	_ controller.Reconciler = (*Reconciler)(nil)
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Pipeline Run
// resource with the current status of the resource.
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	c.Logger.Infof("Reconciling %v", time.Now())

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
	var merr error

	if pr.IsDone() {
		if err := artifacts.CleanupArtifactStorage(pr, c.KubeClientSet, c.Logger); err != nil {
			c.Logger.Errorf("Failed to delete PVC for PipelineRun %s: %v", pr.Name, err)
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
	} else {
		if err := c.tracker.Track(pr.GetTaskRunRef(), pr); err != nil {
			c.Logger.Errorf("Failed to create tracker for TaskRuns for PipelineRun %s: %v", pr.Name, err)
			c.Recorder.Event(pr, corev1.EventTypeWarning, eventReasonFailed, "Failed to create tracker for TaskRuns for PipelineRun")
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

	// Since we are using the status subresource, it is not possible to update
	// the status and labels/annotations simultaneously.
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

	return merr
}

func (c *Reconciler) getPipelineFunc(tr *v1alpha1.PipelineRun) resources.GetPipeline {
	var gtFunc resources.GetPipeline = func(name string) (v1alpha1.PipelineInterface, error) {
		p, err := c.pipelineLister.Pipelines(tr.Namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return p, nil
	}
	return gtFunc
}

func (c *Reconciler) reconcile(ctx context.Context, pr *v1alpha1.PipelineRun) error {
	// We may be reading a version of the object that was stored at an older version
	// and may not have had all of the assumed default specified.
	pr.SetDefaults(v1alpha1.WithUpgradeViaDefaulting(ctx))

	getPipelineFunc := c.getPipelineFunc(pr)
	pipelineMeta, pipelineSpec, err := resources.GetPipelineData(pr, getPipelineFunc)
	if err != nil {
		c.Logger.Errorf("Failed to determine Pipeline spec to use for pipelinerun %s: %v", pr.Name, err)
		pr.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: ReasonCouldntGetPipeline,
			Message: fmt.Sprintf("Error retrieving pipeline for pipelinerun %s: %s",
				fmt.Sprintf("%s/%s", pr.Namespace, pr.Name), err),
		})
		return nil
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

	d, err := v1alpha1.BuildDAG(pipelineSpec.Tasks)
	if err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: ReasonInvalidGraph,
			Message: fmt.Sprintf("PipelineRun %s's Pipeline DAG is invalid: %s",
				fmt.Sprintf("%s/%s", pr.Namespace, pr.Name), err),
		})
		return nil
	}

	if err := pipelineSpec.Validate(ctx); err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: ReasonFailedValidation,
			Message: fmt.Sprintf("Pipeline %s can't be Run; it has an invalid spec: %s",
				fmt.Sprintf("%s/%s", pipelineMeta.Namespace, pr.Name), err),
		})
		return nil
	}

	if err := resources.ValidateResourceBindings(pipelineSpec, pr); err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: ReasonInvalidBindings,
			Message: fmt.Sprintf("PipelineRun %s doesn't bind Pipeline %s's PipelineResources correctly: %s",
				fmt.Sprintf("%s/%s", pr.Namespace, pr.Name), fmt.Sprintf("%s/%s", pr.Namespace, pr.Spec.PipelineRef.Name), err),
		})
		return nil
	}
	providedResources, err := resources.GetResourcesFromBindings(pr, c.resourceLister.PipelineResources(pr.Namespace).Get)
	if err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: ReasonCouldntGetResource,
			Message: fmt.Sprintf("PipelineRun %s can't be Run; it tries to bind Resources that don't exist: %s",
				fmt.Sprintf("%s/%s", pipelineMeta.Namespace, pr.Name), err),
		})
		return nil
	}

	// Ensure that the parameters from the PipelineRun are overriding Pipeline parameters with the same type.
	// Weird substitution issues can occur if this is not validated (ApplyParameters() does not verify type).
	err = resources.ValidateParamTypesMatching(pipelineSpec, pr)
	if err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: ReasonParameterTypeMismatch,
			Message: fmt.Sprintf("PipelineRun %s parameters have mismatching types with Pipeline %s's parameters: %s",
				fmt.Sprintf("%s/%s", pr.Namespace, pr.Name), fmt.Sprintf("%s/%s", pr.Namespace, pr.Spec.PipelineRef.Name), err),
		})
		return nil
	}

	// Apply parameter substitution from the PipelineRun
	pipelineSpec = resources.ApplyParameters(pipelineSpec, pr)

	pipelineState, err := resources.ResolvePipelineRun(
		*pr,
		func(name string) (v1alpha1.TaskInterface, error) {
			return c.taskLister.Tasks(pr.Namespace).Get(name)
		},
		func(name string) (*v1alpha1.TaskRun, error) {
			return c.taskRunLister.TaskRuns(pr.Namespace).Get(name)
		},
		func(name string) (v1alpha1.TaskInterface, error) {
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
			pr.Status.SetCondition(&apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
				Reason: ReasonCouldntGetTask,
				Message: fmt.Sprintf("Pipeline %s can't be Run; it contains Tasks that don't exist: %s",
					fmt.Sprintf("%s/%s", pipelineMeta.Namespace, pipelineMeta.Name), err),
			})
		case *resources.ConditionNotFoundError:
			pr.Status.SetCondition(&apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
				Reason: ReasonCouldntGetCondition,
				Message: fmt.Sprintf("PipelineRun %s can't be Run; it contains Conditions that don't exist:  %s",
					fmt.Sprintf("%s/%s", pipelineMeta.Namespace, pr.Name), err),
			})
		default:
			pr.Status.SetCondition(&apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
				Reason: ReasonFailedValidation,
				Message: fmt.Sprintf("PipelineRun %s can't be Run; couldn't resolve all references: %s",
					fmt.Sprintf("%s/%s", pipelineMeta.Namespace, pr.Name), err),
			})
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
			pr.Status.SetCondition(&apis.Condition{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  ReasonFailedValidation,
				Message: err.Error(),
			})
			// update pr completed time
			return nil
		}
	}

	// If the pipelinerun is cancelled, cancel tasks and update status
	if pr.IsCancelled() {
		before := pr.Status.GetCondition(apis.ConditionSucceeded)
		err := cancelPipelineRun(pr, pipelineState, c.PipelineClientSet)
		after := pr.Status.GetCondition(apis.ConditionSucceeded)
		reconciler.EmitEvent(c.Recorder, before, after, pr)
		return err
	}

	candidateTasks, err := dag.GetSchedulable(d, pipelineState.SuccessfulPipelineTaskNames()...)
	if err != nil {
		c.Logger.Errorf("Error getting potential next tasks for valid pipelinerun %s: %v", pr.Name, err)
	}

	rprts := pipelineState.GetNextTasks(candidateTasks)

	var as artifacts.ArtifactStorageInterface

	if as, err = artifacts.InitializeArtifactStorage(c.Images, pr, pipelineSpec, c.KubeClientSet, c.Logger); err != nil {
		c.Logger.Infof("PipelineRun failed to initialize artifact storage %s", pr.Name)
		return err
	}

	for _, rprt := range rprts {
		if rprt == nil {
			continue
		}
		if rprt.ResolvedConditionChecks == nil || rprt.ResolvedConditionChecks.IsSuccess() {
			rprt.TaskRun, err = c.createTaskRun(rprt, pr, as.StorageBasePath(pr))
			if err != nil {
				c.Recorder.Eventf(pr, corev1.EventTypeWarning, "TaskRunCreationFailed", "Failed to create TaskRun %q: %v", rprt.TaskRunName, err)
				return xerrors.Errorf("error creating TaskRun called %s for PipelineTask %s from PipelineRun %s: %w", rprt.TaskRunName, rprt.PipelineTask.Name, pr.Name, err)
			}
		} else if !rprt.ResolvedConditionChecks.HasStarted() {
			for _, rcc := range rprt.ResolvedConditionChecks {
				rcc.ConditionCheck, err = c.makeConditionCheckContainer(rprt, rcc, pr)
				if err != nil {
					c.Recorder.Eventf(pr, corev1.EventTypeWarning, "ConditionCheckCreationFailed", "Failed to create TaskRun %q: %v", rcc.ConditionCheckName, err)
					return xerrors.Errorf("error creating ConditionCheck container called %s for PipelineTask %s from PipelineRun %s: %w", rcc.ConditionCheckName, rprt.PipelineTask.Name, pr.Name, err)
				}
			}
		}
	}
	before := pr.Status.GetCondition(apis.ConditionSucceeded)
	after := resources.GetPipelineConditionStatus(pr, pipelineState, c.Logger, d)
	pr.Status.SetCondition(after)
	reconciler.EmitEvent(c.Recorder, before, after, pr)

	pr.Status.TaskRuns = getTaskRunsStatus(pr, pipelineState)

	c.Logger.Infof("PipelineRun %s status is being set to %s", pr.Name, pr.Status.GetCondition(apis.ConditionSucceeded))
	return nil
}

func getTaskRunsStatus(pr *v1alpha1.PipelineRun, state []*resources.ResolvedPipelineRunTask) map[string]*v1alpha1.PipelineRunTaskRunStatus {
	status := make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
	for _, rprt := range state {
		if rprt.TaskRun == nil && rprt.ResolvedConditionChecks == nil {
			continue
		}

		var prtrs *v1alpha1.PipelineRunTaskRunStatus
		if rprt.TaskRun != nil {
			prtrs = pr.Status.TaskRuns[rprt.TaskRun.Name]
		}
		if prtrs == nil {
			prtrs = &v1alpha1.PipelineRunTaskRunStatus{
				PipelineTaskName: rprt.PipelineTask.Name,
			}
		}

		if rprt.TaskRun != nil {
			prtrs.Status = &rprt.TaskRun.Status
		}

		if len(rprt.ResolvedConditionChecks) > 0 {
			cStatus := make(map[string]*v1alpha1.PipelineRunConditionCheckStatus)
			for _, c := range rprt.ResolvedConditionChecks {
				cStatus[c.ConditionCheckName] = &v1alpha1.PipelineRunConditionCheckStatus{
					ConditionName: c.Condition.Name,
				}
				if c.ConditionCheck != nil {
					cStatus[c.ConditionCheckName].Status = c.NewConditionCheckStatus()
				}
			}
			prtrs.ConditionChecks = cStatus
			if rprt.ResolvedConditionChecks.IsDone() && !rprt.ResolvedConditionChecks.IsSuccess() {
				if prtrs.Status == nil {
					prtrs.Status = &v1alpha1.TaskRunStatus{}
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

func (c *Reconciler) updateTaskRunsStatusDirectly(pr *v1alpha1.PipelineRun) error {
	for taskRunName := range pr.Status.TaskRuns {
		// TODO(dibyom): Add conditionCheck statuses here
		prtrs := pr.Status.TaskRuns[taskRunName]
		tr, err := c.taskRunLister.TaskRuns(pr.Namespace).Get(taskRunName)
		if err != nil {
			// If the TaskRun isn't found, it just means it won't be run
			if !errors.IsNotFound(err) {
				return xerrors.Errorf("error retrieving TaskRun %s: %w", taskRunName, err)
			}
		} else {
			prtrs.Status = &tr.Status
		}
	}
	return nil
}

func (c *Reconciler) createTaskRun(rprt *resources.ResolvedPipelineRunTask, pr *v1alpha1.PipelineRun, storageBasePath string) (*v1alpha1.TaskRun, error) {
	tr, _ := c.taskRunLister.TaskRuns(pr.Namespace).Get(rprt.TaskRunName)
	if tr != nil {
		//is a retry
		addRetryHistory(tr)
		clearStatus(tr)
		tr.Status.SetCondition(&apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		})
		return c.PipelineClientSet.TektonV1alpha1().TaskRuns(pr.Namespace).UpdateStatus(tr)
	}

	tr = &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:            rprt.TaskRunName,
			Namespace:       pr.Namespace,
			OwnerReferences: pr.GetOwnerReference(),
			Labels:          getTaskrunLabels(pr, rprt.PipelineTask.Name),
			Annotations:     getTaskrunAnnotations(pr),
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name: rprt.ResolvedTaskResources.TaskName,
				Kind: rprt.ResolvedTaskResources.Kind,
			},
			Inputs: v1alpha1.TaskRunInputs{
				Params: rprt.PipelineTask.Params,
			},
			ServiceAccountName: pr.GetServiceAccountName(rprt.PipelineTask.Name),
			Timeout:            getTaskRunTimeout(pr),
			PodTemplate:        pr.Spec.PodTemplate,
		}}

	resources.WrapSteps(&tr.Spec, rprt.PipelineTask, rprt.ResolvedTaskResources.Inputs, rprt.ResolvedTaskResources.Outputs, storageBasePath)
	c.Logger.Infof("Creating a new TaskRun object %s", rprt.TaskRunName)
	return c.PipelineClientSet.TektonV1alpha1().TaskRuns(pr.Namespace).Create(tr)
}

func addRetryHistory(tr *v1alpha1.TaskRun) {
	newStatus := *tr.Status.DeepCopy()
	newStatus.RetriesStatus = nil
	tr.Status.RetriesStatus = append(tr.Status.RetriesStatus, newStatus)
}

func clearStatus(tr *v1alpha1.TaskRun) {
	tr.Status.StartTime = nil
	tr.Status.CompletionTime = nil
	tr.Status.PodName = ""
}

func getTaskrunAnnotations(pr *v1alpha1.PipelineRun) map[string]string {
	// Propagate annotations from PipelineRun to TaskRun.
	annotations := make(map[string]string, len(pr.ObjectMeta.Annotations)+1)
	for key, val := range pr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	return annotations
}

func getTaskrunLabels(pr *v1alpha1.PipelineRun, pipelineTaskName string) map[string]string {
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

func getTaskRunTimeout(pr *v1alpha1.PipelineRun) *metav1.Duration {
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
			taskRunTimeout = &metav1.Duration{Duration: timeout}
		}
	}
	return taskRunTimeout
}

func (c *Reconciler) updateStatus(pr *v1alpha1.PipelineRun) (*v1alpha1.PipelineRun, error) {
	newPr, err := c.pipelineRunLister.PipelineRuns(pr.Namespace).Get(pr.Name)
	if err != nil {
		return nil, xerrors.Errorf("Error getting PipelineRun %s when updating status: %w", pr.Name, err)
	}
	succeeded := pr.Status.GetCondition(apis.ConditionSucceeded)
	if succeeded.Status == corev1.ConditionFalse || succeeded.Status == corev1.ConditionTrue {
		// update pr completed time
		pr.Status.CompletionTime = &metav1.Time{Time: time.Now()}

	}
	if !reflect.DeepEqual(pr.Status, newPr.Status) {
		newPr.Status = pr.Status
		return c.PipelineClientSet.TektonV1alpha1().PipelineRuns(pr.Namespace).UpdateStatus(newPr)
	}
	return newPr, nil
}

func (c *Reconciler) updateLabelsAndAnnotations(pr *v1alpha1.PipelineRun) (*v1alpha1.PipelineRun, error) {
	newPr, err := c.pipelineRunLister.PipelineRuns(pr.Namespace).Get(pr.Name)
	if err != nil {
		return nil, xerrors.Errorf("Error getting PipelineRun %s when updating labels/annotations: %w", pr.Name, err)
	}
	if !reflect.DeepEqual(pr.ObjectMeta.Labels, newPr.ObjectMeta.Labels) || !reflect.DeepEqual(pr.ObjectMeta.Annotations, newPr.ObjectMeta.Annotations) {
		newPr.ObjectMeta.Labels = pr.ObjectMeta.Labels
		newPr.ObjectMeta.Annotations = pr.ObjectMeta.Annotations
		return c.PipelineClientSet.TektonV1alpha1().PipelineRuns(pr.Namespace).Update(newPr)
	}
	return newPr, nil
}

func (c *Reconciler) makeConditionCheckContainer(rprt *resources.ResolvedPipelineRunTask, rcc *resources.ResolvedConditionCheck, pr *v1alpha1.PipelineRun) (*v1alpha1.ConditionCheck, error) {
	labels := getTaskrunLabels(pr, rprt.PipelineTask.Name)
	labels[pipeline.GroupName+pipeline.ConditionCheckKey] = rcc.ConditionCheckName

	taskSpec, err := rcc.ConditionToTaskSpec()
	if err != nil {
		return nil, xerrors.Errorf("Failed to get TaskSpec from Condition: %w", err)
	}

	tr := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:            rcc.ConditionCheckName,
			Namespace:       pr.Namespace,
			OwnerReferences: pr.GetOwnerReference(),
			Labels:          labels,
			Annotations:     getTaskrunAnnotations(pr), // Propagate annotations from PipelineRun to TaskRun.
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskSpec:           taskSpec,
			ServiceAccountName: pr.GetServiceAccountName(rprt.PipelineTask.Name),
			Inputs: v1alpha1.TaskRunInputs{
				Params:    rcc.PipelineTaskCondition.Params,
				Resources: rcc.ToTaskResourceBindings(),
			},
			Timeout:     getTaskRunTimeout(pr),
			PodTemplate: pr.Spec.PodTemplate,
		}}

	cctr, err := c.PipelineClientSet.TektonV1alpha1().TaskRuns(pr.Namespace).Create(tr)
	cc := v1alpha1.ConditionCheck(*cctr)
	return &cc, err
}
