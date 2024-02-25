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
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pipelineErrors "github.com/tektoncd/pipeline/pkg/apis/pipeline/errors"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	pipelinerunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1/pipelinerun"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	alpha1listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	beta1listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/internal/affinityassistant"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	"github.com/tektoncd/pipeline/pkg/pipelinerunmetrics"
	tknreconciler "github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/apiserver"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipeline/dag"
	rprp "github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/pipelinespec"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
	tresources "github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	"github.com/tektoncd/pipeline/pkg/remote"
	resolution "github.com/tektoncd/pipeline/pkg/remoteresolution/resource"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/substitution"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmap"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Aliased for backwards compatibility; do not add additional reasons here
var (
	// ReasonCouldntGetPipeline indicates that the reason for the failure status is that the
	// associated Pipeline couldn't be retrieved
	ReasonCouldntGetPipeline = v1.PipelineRunReasonCouldntGetPipeline.String()
	// ReasonInvalidBindings indicates that the reason for the failure status is that the
	// PipelineResources bound in the PipelineRun didn't match those declared in the Pipeline
	ReasonInvalidBindings = v1.PipelineRunReasonInvalidBindings.String()
	// ReasonInvalidWorkspaceBinding indicates that a Pipeline expects a workspace but a
	// PipelineRun has provided an invalid binding.
	ReasonInvalidWorkspaceBinding = v1.PipelineRunReasonInvalidWorkspaceBinding.String()
	// ReasonInvalidTaskRunSpec indicates that PipelineRun.Spec.TaskRunSpecs[].PipelineTaskName is defined with
	// a not exist taskName in pipelineSpec.
	ReasonInvalidTaskRunSpec = v1.PipelineRunReasonInvalidTaskRunSpec.String()
	// ReasonParameterTypeMismatch indicates that the reason for the failure status is that
	// parameter(s) declared in the PipelineRun do not have the some declared type as the
	// parameters(s) declared in the Pipeline that they are supposed to override.
	ReasonParameterTypeMismatch = v1.PipelineRunReasonParameterTypeMismatch.String()
	// ReasonObjectParameterMissKeys indicates that the object param value provided from PipelineRun spec
	// misses some keys required for the object param declared in Pipeline spec.
	ReasonObjectParameterMissKeys = v1.PipelineRunReasonObjectParameterMissKeys.String()
	// ReasonParamArrayIndexingInvalid indicates that the use of param array indexing is out of bound.
	ReasonParamArrayIndexingInvalid = v1.PipelineRunReasonParamArrayIndexingInvalid.String()
	// ReasonCouldntGetTask indicates that the reason for the failure status is that the
	// associated Pipeline's Tasks couldn't all be retrieved
	ReasonCouldntGetTask = v1.PipelineRunReasonCouldntGetTask.String()
	// ReasonParameterMissing indicates that the reason for the failure status is that the
	// associated PipelineRun didn't provide all the required parameters
	ReasonParameterMissing = v1.PipelineRunReasonParameterMissing.String()
	// ReasonFailedValidation indicates that the reason for failure status is
	// that pipelinerun failed runtime validation
	ReasonFailedValidation = v1.PipelineRunReasonFailedValidation.String()
	// ReasonInvalidGraph indicates that the reason for the failure status is that the
	// associated Pipeline is an invalid graph (a.k.a wrong order, cycle, …)
	ReasonInvalidGraph = v1.PipelineRunReasonInvalidGraph.String()
	// ReasonCancelled indicates that a PipelineRun was cancelled.
	ReasonCancelled = v1.PipelineRunReasonCancelled.String()
	// ReasonPending indicates that a PipelineRun is pending.
	ReasonPending = v1.PipelineRunReasonPending.String()
	// ReasonCouldntCancel indicates that a PipelineRun was cancelled but attempting to update
	// all of the running TaskRuns as cancelled failed.
	ReasonCouldntCancel = v1.PipelineRunReasonCouldntCancel.String()
	// ReasonCouldntTimeOut indicates that a PipelineRun was timed out but attempting to update
	// all of the running TaskRuns as timed out failed.
	ReasonCouldntTimeOut = v1.PipelineRunReasonCouldntTimeOut.String()
	// ReasonInvalidMatrixParameterTypes indicates a matrix contains invalid parameter types
	ReasonInvalidMatrixParameterTypes = v1.PipelineRunReasonInvalidMatrixParameterTypes.String()
	// ReasonInvalidTaskResultReference indicates a task result was declared
	// but was not initialized by that task
	ReasonInvalidTaskResultReference = v1.PipelineRunReasonInvalidTaskResultReference.String()
	// ReasonRequiredWorkspaceMarkedOptional indicates an optional workspace
	// has been passed to a Task that is expecting a non-optional workspace
	ReasonRequiredWorkspaceMarkedOptional = v1.PipelineRunReasonRequiredWorkspaceMarkedOptional.String()
	// ReasonResolvingPipelineRef indicates that the PipelineRun is waiting for
	// its pipelineRef to be asynchronously resolved.
	ReasonResolvingPipelineRef = v1.PipelineRunReasonResolvingPipelineRef.String()
	// ReasonResourceVerificationFailed indicates that the pipeline fails the trusted resource verification,
	// it could be the content has changed, signature is invalid or public key is invalid
	ReasonResourceVerificationFailed = v1.PipelineRunReasonResourceVerificationFailed.String()
	// ReasonCreateRunFailed indicates that the pipeline fails to create the taskrun or other run resources
	ReasonCreateRunFailed = v1.PipelineRunReasonCreateRunFailed.String()
)

// constants used as kind descriptors for various types of runs; these constants
// match their corresponding controller names. Given that it's odd to use a
// "ControllerName" const in describing the type of run, we import these
// constants (for consistency) but rename them (for ergonomic semantics).
const (
	taskRun   = pipeline.TaskRunControllerName
	customRun = pipeline.CustomRunControllerName
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	KubeClientSet     kubernetes.Interface
	PipelineClientSet clientset.Interface
	Images            pipeline.Images
	Clock             clock.PassiveClock

	// listers index properties about resources
	pipelineRunLister        listers.PipelineRunLister
	taskRunLister            listers.TaskRunLister
	customRunLister          beta1listers.CustomRunLister
	verificationPolicyLister alpha1listers.VerificationPolicyLister
	cloudEventClient         cloudevent.CEClient
	metrics                  *pipelinerunmetrics.Recorder
	pvcHandler               volumeclaim.PvcHandler
	resolutionRequester      resolution.Requester
	tracerProvider           trace.TracerProvider
}

var (
	// Check that our Reconciler implements pipelinerunreconciler.Interface
	_                              pipelinerunreconciler.Interface = (*Reconciler)(nil)
	filterReservedAnnotationRegexp                                 = regexp.MustCompile(pipeline.TektonReservedAnnotationExpr)
)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Pipeline Run
// resource with the current status of the resource.
func (c *Reconciler) ReconcileKind(ctx context.Context, pr *v1.PipelineRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	ctx = cloudevent.ToContext(ctx, c.cloudEventClient)
	ctx = initTracing(ctx, c.tracerProvider, pr)
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "PipelineRun:ReconcileKind")
	defer span.End()

	span.SetAttributes(
		attribute.String("pipelinerun", pr.Name), attribute.String("namespace", pr.Namespace),
	)

	// Read the initial condition
	before := pr.Status.GetCondition(apis.ConditionSucceeded)

	// Check if we are failing to mark this as timed out for a while. If we are, mark immediately and finish the
	// reconcile. We are assuming here that if the PipelineRun has timed out for a long time, it had time to run
	// before and it kept failing. One reason that can happen is exceeding etcd request size limit. Finishing it early
	// makes sure the request size is manageable
	if !pr.IsDone() && pr.HasTimedOutForALongTime(ctx, c.Clock) && !pr.IsTimeoutConditionSet() {
		if err := timeoutPipelineRun(ctx, logger, pr, c.PipelineClientSet); err != nil {
			return err
		}
		if err := c.finishReconcileUpdateEmitEvents(ctx, pr, before, nil); err != nil {
			return err
		}
		return controller.NewPermanentError(errors.New("PipelineRun has timed out for a long time"))
	}

	if !pr.HasStarted() && !pr.IsPending() {
		pr.Status.InitializeConditions(c.Clock)
		// In case node time was not synchronized, when controller has been scheduled to other nodes.
		if pr.Status.StartTime.Sub(pr.CreationTimestamp.Time) < 0 {
			logger.Warnf("PipelineRun %s createTimestamp %s is after the pipelineRun started %s", pr.GetNamespacedName().String(), pr.CreationTimestamp, pr.Status.StartTime)
			pr.Status.StartTime = &pr.CreationTimestamp
		}

		// Emit events. During the first reconcile the status of the PipelineRun may change twice
		// from not Started to Started and then to Running, so we need to sent the event here
		// and at the end of 'Reconcile' again.
		// We also want to send the "Started" event as soon as possible for anyone who may be waiting
		// on the event to perform user facing initialisations, such has reset a CI check status
		afterCondition := pr.Status.GetCondition(apis.ConditionSucceeded)
		events.Emit(ctx, nil, afterCondition, pr)

		// We already sent an event for start, so update `before` with the current status
		before = pr.Status.GetCondition(apis.ConditionSucceeded)
	}

	// list VerificationPolicies for trusted resources
	vp, err := c.verificationPolicyLister.VerificationPolicies(pr.Namespace).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list VerificationPolicies from namespace %s with error %w", pr.Namespace, err)
	}
	getPipelineFunc := resources.GetPipelineFunc(ctx, c.KubeClientSet, c.PipelineClientSet, c.resolutionRequester, pr, vp)

	if pr.IsDone() {
		pr.SetDefaults(ctx)
		err := c.cleanupAffinityAssistantsAndPVCs(ctx, pr)
		if err != nil {
			logger.Errorf("Failed to delete StatefulSet or PVC for PipelineRun %s: %v", pr.Name, err)
		}
		return c.finishReconcileUpdateEmitEvents(ctx, pr, before, err)
	}

	if err := propagatePipelineNameLabelToPipelineRun(pr); err != nil {
		logger.Errorf("Failed to propagate pipeline name label to pipelinerun %s: %v", pr.Name, err)
		return c.finishReconcileUpdateEmitEvents(ctx, pr, before, err)
	}

	// If the pipelinerun is cancelled, cancel tasks and update status
	if pr.IsCancelled() {
		err := cancelPipelineRun(ctx, logger, pr, c.PipelineClientSet)
		return c.finishReconcileUpdateEmitEvents(ctx, pr, before, err)
	}

	// Make sure that the PipelineRun status is in sync with the actual TaskRuns
	err = c.updatePipelineRunStatusFromInformer(ctx, pr)
	if err != nil {
		// This should not fail. Return the error so we can re-try later.
		logger.Errorf("Error while syncing the pipelinerun status: %v", err.Error())
		return c.finishReconcileUpdateEmitEvents(ctx, pr, before, err)
	}

	// Reconcile this copy of the pipelinerun and then write back any status or label
	// updates regardless of whether the reconciliation errored out.
	if err = c.reconcile(ctx, pr, getPipelineFunc, before); err != nil {
		logger.Errorf("Reconcile error: %v", err.Error())
	}

	if err = c.finishReconcileUpdateEmitEvents(ctx, pr, before, err); err != nil {
		return err
	}

	if pr.Status.StartTime != nil {
		// Compute the time since the task started.
		elapsed := c.Clock.Since(pr.Status.StartTime.Time)
		// Snooze this resource until the appropriate timeout has elapsed.
		// but if the timeout has been disabled by setting timeout to 0, we
		// do not want to subtract from 0, because a negative wait time will
		// result in the requeue happening essentially immediately
		timeout := pr.PipelineTimeout(ctx)
		taskTimeout := pr.TasksTimeout()
		waitTime := timeout - elapsed
		if timeout == config.NoTimeoutDuration {
			waitTime = time.Duration(config.FromContextOrDefaults(ctx).Defaults.DefaultTimeoutMinutes) * time.Minute
		}
		if pr.Status.FinallyStartTime == nil && taskTimeout != nil {
			waitTime = pr.TasksTimeout().Duration - elapsed
			if taskTimeout.Duration == config.NoTimeoutDuration {
				waitTime = time.Duration(config.FromContextOrDefaults(ctx).Defaults.DefaultTimeoutMinutes) * time.Minute
			}
		} else if pr.Status.FinallyStartTime != nil && pr.FinallyTimeout() != nil {
			finallyWaitTime := pr.FinallyTimeout().Duration - c.Clock.Since(pr.Status.FinallyStartTime.Time)
			if finallyWaitTime < waitTime {
				waitTime = finallyWaitTime
			}
		}
		return controller.NewRequeueAfter(waitTime)
	}
	return nil
}

func (c *Reconciler) durationAndCountMetrics(ctx context.Context, pr *v1.PipelineRun, beforeCondition *apis.Condition) {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "durationAndCountMetrics")
	defer span.End()
	logger := logging.FromContext(ctx)
	if pr.IsDone() {
		err := c.metrics.DurationAndCount(pr, beforeCondition)
		if err != nil {
			logger.Warnf("Failed to log the metrics : %v", err)
		}
	}
}

func (c *Reconciler) finishReconcileUpdateEmitEvents(ctx context.Context, pr *v1.PipelineRun, beforeCondition *apis.Condition, previousError error) error {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "finishReconcileUpdateEmitEvents")
	defer span.End()
	logger := logging.FromContext(ctx)

	afterCondition := pr.Status.GetCondition(apis.ConditionSucceeded)
	events.Emit(ctx, beforeCondition, afterCondition, pr)
	_, err := c.updateLabelsAndAnnotations(ctx, pr)
	if err != nil {
		logger.Warn("Failed to update PipelineRun labels/annotations", zap.Error(err))
		events.EmitError(controller.GetEventRecorder(ctx), err, pr)
	}

	merr := multierror.Append(previousError, err).ErrorOrNil()
	if controller.IsPermanentError(previousError) {
		return controller.NewPermanentError(merr)
	}
	return merr
}

// resolvePipelineState will attempt to resolve each referenced task in the pipeline's spec and all of the resources
// specified by those tasks.
func (c *Reconciler) resolvePipelineState(
	ctx context.Context,
	tasks []v1.PipelineTask,
	pipelineMeta *metav1.ObjectMeta,
	pr *v1.PipelineRun,
	pst resources.PipelineRunState,
) (resources.PipelineRunState, error) {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "resolvePipelineState")
	defer span.End()
	// Resolve each task individually because they each could have a different reference context (remote or local).
	for _, task := range tasks {
		// We need the TaskRun name to ensure that we don't perform an additional remote resolution request for a PipelineTask
		// in the TaskRun reconciler.
		trName := resources.GetTaskRunName(pr.Status.ChildReferences, task.Name, pr.Name)

		// list VerificationPolicies for trusted resources
		vp, err := c.verificationPolicyLister.VerificationPolicies(pr.Namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list VerificationPolicies from namespace %s with error %w", pr.Namespace, err)
		}
		fn := tresources.GetTaskFunc(ctx, c.KubeClientSet, c.PipelineClientSet, c.resolutionRequester, pr, task.TaskRef, trName, pr.Namespace, pr.Spec.TaskRunTemplate.ServiceAccountName, vp)

		getCustomRunFunc := func(name string) (*v1beta1.CustomRun, error) {
			r, err := c.customRunLister.CustomRuns(pr.Namespace).Get(name)
			if err != nil {
				return nil, err
			}
			return r, nil
		}

		resolvedTask, err := resources.ResolvePipelineTask(ctx,
			*pr,
			fn,
			func(name string) (*v1.TaskRun, error) {
				return c.taskRunLister.TaskRuns(pr.Namespace).Get(name)
			},
			getCustomRunFunc,
			task,
			pst,
		)
		if err != nil {
			if resolutioncommon.IsErrTransient(err) {
				return nil, err
			}
			if errors.Is(err, remote.ErrRequestInProgress) {
				return nil, err
			}
			var nfErr *resources.TaskNotFoundError
			if errors.As(err, &nfErr) {
				pr.Status.MarkFailed(v1.PipelineRunReasonCouldntGetTask.String(),
					"Pipeline %s/%s can't be Run; it contains Tasks that don't exist: %s",
					pipelineMeta.Namespace, pipelineMeta.Name, nfErr)
			} else {
				pr.Status.MarkFailed(v1.PipelineRunReasonFailedValidation.String(),
					"PipelineRun %s/%s can't be Run; couldn't resolve all references: %s",
					pipelineMeta.Namespace, pr.Name, pipelineErrors.WrapUserError(err))
			}
			return nil, controller.NewPermanentError(err)
		}
		if resolvedTask.ResolvedTask != nil && resolvedTask.ResolvedTask.VerificationResult != nil {
			cond, err := conditionFromVerificationResult(resolvedTask.ResolvedTask.VerificationResult, pr, task.Name)
			pr.Status.SetCondition(cond)
			if err != nil {
				pr.Status.MarkFailed(v1.PipelineRunReasonResourceVerificationFailed.String(), err.Error())
				return nil, controller.NewPermanentError(err)
			}
		}
		pst = append(pst, resolvedTask)
	}
	return pst, nil
}

func (c *Reconciler) reconcile(ctx context.Context, pr *v1.PipelineRun, getPipelineFunc rprp.GetPipeline, beforeCondition *apis.Condition) error {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "reconcile")
	defer span.End()
	defer c.durationAndCountMetrics(ctx, pr, beforeCondition)
	logger := logging.FromContext(ctx)
	pr.SetDefaults(ctx)

	// When pipeline run is pending, return to avoid creating the task
	if pr.IsPending() {
		pr.Status.MarkRunning(v1.PipelineRunReasonPending.String(), fmt.Sprintf("PipelineRun %q is pending", pr.Name))
		return nil
	}

	pipelineMeta, pipelineSpec, err := rprp.GetPipelineData(ctx, pr, getPipelineFunc)
	switch {
	case errors.Is(err, remote.ErrRequestInProgress):
		message := fmt.Sprintf("PipelineRun %s/%s awaiting remote resource", pr.Namespace, pr.Name)
		pr.Status.MarkRunning(v1.PipelineRunReasonResolvingPipelineRef.String(), message)
		return nil
	case errors.Is(err, apiserver.ErrReferencedObjectValidationFailed), errors.Is(err, apiserver.ErrCouldntValidateObjectPermanent):
		logger.Errorf("Failed dryRunValidation for PipelineRun %s: %w", pr.Name, err)
		pr.Status.MarkFailed(v1.PipelineRunReasonFailedValidation.String(),
			"Failed dryRunValidation for PipelineRun %s: %s",
			pr.Name, pipelineErrors.WrapUserError(err))
		return controller.NewPermanentError(err)
	case errors.Is(err, apiserver.ErrCouldntValidateObjectRetryable):
		return err
	case err != nil:
		logger.Errorf("Failed to determine Pipeline spec to use for pipelinerun %s: %v", pr.Name, err)
		pr.Status.MarkFailed(v1.PipelineRunReasonCouldntGetPipeline.String(),
			"Error retrieving pipeline for pipelinerun %s/%s: %s",
			pr.Namespace, pr.Name, err)
		return controller.NewPermanentError(err)
	default:
		// Store the fetched PipelineSpec on the PipelineRun for auditing
		if err := storePipelineSpecAndMergeMeta(ctx, pr, pipelineSpec, pipelineMeta); err != nil {
			logger.Errorf("Failed to store PipelineSpec on PipelineRun.Status for pipelinerun %s: %v", pr.Name, err)
		}
	}

	if pipelineMeta.VerificationResult != nil {
		cond, err := conditionFromVerificationResult(pipelineMeta.VerificationResult, pr, pipelineMeta.Name)
		pr.Status.SetCondition(cond)
		if err != nil {
			pr.Status.MarkFailed(v1.PipelineRunReasonResourceVerificationFailed.String(), err.Error())
			return controller.NewPermanentError(err)
		}
	}

	d, err := dag.Build(v1.PipelineTaskList(pipelineSpec.Tasks), v1.PipelineTaskList(pipelineSpec.Tasks).Deps())
	if err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(v1.PipelineRunReasonInvalidGraph.String(),
			"PipelineRun %s/%s's Pipeline DAG is invalid: %s",
			pr.Namespace, pr.Name, pipelineErrors.WrapUserError(err))
		return controller.NewPermanentError(err)
	}

	// build DAG with a list of final tasks, this DAG is used later to identify
	// if a task in PipelineRunState is final task or not
	// the finally section is optional and might not exist
	// dfinally holds an empty Graph in the absence of finally clause
	dfinally, err := dag.Build(v1.PipelineTaskList(pipelineSpec.Finally), map[string][]string{})
	if err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(v1.PipelineRunReasonInvalidGraph.String(),
			"PipelineRun %s/%s's Pipeline DAG is invalid for finally clause: %s",
			pr.Namespace, pr.Name, pipelineErrors.WrapUserError(err))
		return controller.NewPermanentError(err)
	}

	if err := pipelineSpec.Validate(ctx); err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(v1.PipelineRunReasonFailedValidation.String(),
			"Pipeline %s/%s can't be Run; it has an invalid spec: %s",
			pipelineMeta.Namespace, pipelineMeta.Name, pipelineErrors.WrapUserError(err))
		return controller.NewPermanentError(err)
	}

	// Ensure that the PipelineRun provides all the parameters required by the Pipeline
	if err := resources.ValidateRequiredParametersProvided(&pipelineSpec.Params, &pr.Spec.Params); err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(v1.PipelineRunReasonParameterMissing.String(),
			"PipelineRun %s/%s is missing some parameters required by Pipeline %s/%s: %s",
			pr.Namespace, pr.Name, pr.Namespace, pipelineMeta.Name, err)
		return controller.NewPermanentError(err)
	}

	// Ensure that the parameters from the PipelineRun are overriding Pipeline parameters with the same type.
	// Weird substitution issues can occur if this is not validated (ApplyParameters() does not verify type).
	if err = resources.ValidateParamTypesMatching(pipelineSpec, pr); err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(v1.PipelineRunReasonParameterTypeMismatch.String(),
			"PipelineRun %s/%s parameters have mismatching types with Pipeline %s/%s's parameters: %s",
			pr.Namespace, pr.Name, pr.Namespace, pipelineMeta.Name, err)
		return controller.NewPermanentError(err)
	}

	if config.FromContextOrDefaults(ctx).FeatureFlags.EnableParamEnum {
		if err := taskrun.ValidateEnumParam(ctx, pr.Spec.Params, pipelineSpec.Params); err != nil {
			logger.Errorf("PipelineRun %q Param Enum validation failed: %v", pr.Name, err)
			pr.Status.MarkFailed(v1.PipelineRunReasonInvalidParamValue.String(),
				"PipelineRun %s/%s parameters have invalid value: %s",
				pr.Namespace, pr.Name, pipelineErrors.WrapUserError(err))
			return controller.NewPermanentError(err)
		}
	}

	// Ensure that the keys of an object param declared in PipelineSpec are not missed in the PipelineRunSpec
	if err = resources.ValidateObjectParamRequiredKeys(pipelineSpec.Params, pr.Spec.Params); err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(v1.PipelineRunReasonObjectParameterMissKeys.String(),
			"PipelineRun %s/%s parameters is missing object keys required by Pipeline %s/%s's parameters: %s",
			pr.Namespace, pr.Name, pr.Namespace, pipelineMeta.Name, err)
		return controller.NewPermanentError(err)
	}

	// Ensure that the array reference is not out of bound
	if err := resources.ValidateParamArrayIndex(pipelineSpec, pr.Spec.Params); err != nil {
		// This Run has failed, so we need to mark it as failed and stop reconciling it
		pr.Status.MarkFailed(v1.PipelineRunReasonParamArrayIndexingInvalid.String(),
			"PipelineRun %s/%s failed validation: failed to validate Pipeline %s/%s's parameter which has an invalid index while referring to an array: %s",
			pr.Namespace, pr.Name, pr.Namespace, pipelineMeta.Name, err)
		return controller.NewPermanentError(err)
	}

	// Ensure that the workspaces expected by the Pipeline are provided by the PipelineRun.
	if err := resources.ValidateWorkspaceBindings(pipelineSpec, pr); err != nil {
		pr.Status.MarkFailed(v1.PipelineRunReasonInvalidWorkspaceBinding.String(),
			"PipelineRun %s/%s doesn't bind Pipeline %s/%s's Workspaces correctly: %s",
			pr.Namespace, pr.Name, pr.Namespace, pipelineMeta.Name, err)
		return controller.NewPermanentError(err)
	}

	// Ensure that the TaskRunSpecs defined are correct.
	if err := resources.ValidateTaskRunSpecs(pipelineSpec, pr); err != nil {
		pr.Status.MarkFailed(v1.PipelineRunReasonInvalidTaskRunSpec.String(),
			"PipelineRun %s/%s doesn't define taskRunSpecs correctly: %s",
			pr.Namespace, pr.Name, err)
		return controller.NewPermanentError(err)
	}

	resources.ApplyParametersToWorkspaceBindings(ctx, pr)
	// Make a deep copy of the Pipeline and its Tasks before value substution.
	// This is used to find referenced pipeline-level params at each PipelineTask when validate param enum subset requirement
	originalPipeline := pipelineSpec.DeepCopy()
	originalTasks := originalPipeline.Tasks
	originalTasks = append(originalTasks, originalPipeline.Finally...)

	// Apply parameter substitution from the PipelineRun
	pipelineSpec = resources.ApplyParameters(ctx, pipelineSpec, pr)
	pipelineSpec = resources.ApplyContexts(pipelineSpec, pipelineMeta.Name, pr)
	pipelineSpec = resources.ApplyWorkspaces(pipelineSpec, pr)
	// Update pipelinespec of pipelinerun's status field
	pr.Status.PipelineSpec = pipelineSpec

	// pipelineState holds a list of pipeline tasks after fetching their resolved Task specs.
	// pipelineState also holds a taskRun for each pipeline task after the taskRun is created
	// pipelineState is instantiated and updated on every reconcile cycle
	// Resolve the set of tasks (and possibly task runs).
	tasks := pipelineSpec.Tasks
	if len(pipelineSpec.Finally) > 0 {
		tasks = append(tasks, pipelineSpec.Finally...)
	}

	// We split tasks in two lists:
	// - those with a completed (Task|Custom)Run reference (i.e. those that finished running)
	// - those without a (Task|Custom)Run reference
	// We resolve the status for the former first, to collect all results available at this stage
	// We know that tasks in progress or completed have had their fan-out alteady calculated so
	// they can be safely processed in the first iteration. The underlying assumption is that if
	// a PipelineTask has at least one TaskRun associated, then all its TaskRuns have been
	// created already.
	// The second group takes as input the partial state built in the first iteration and finally
	// the two results are collated
	ranOrRunningTaskNames := sets.Set[string]{}
	ranOrRunningTasks := []v1.PipelineTask{}
	notStartedTasks := []v1.PipelineTask{}

	for _, child := range pr.Status.ChildReferences {
		ranOrRunningTaskNames.Insert(child.PipelineTaskName)
	}
	for _, task := range tasks {
		if ranOrRunningTaskNames.Has(task.Name) {
			ranOrRunningTasks = append(ranOrRunningTasks, task)
		} else {
			notStartedTasks = append(notStartedTasks, task)
		}
	}
	// First iteration
	pst := resources.PipelineRunState{}
	pipelineRunState, err := c.resolvePipelineState(ctx, ranOrRunningTasks, pipelineMeta.ObjectMeta, pr, pst)
	switch {
	case errors.Is(err, remote.ErrRequestInProgress):
		message := fmt.Sprintf("PipelineRun %s/%s awaiting remote resource", pr.Namespace, pr.Name)
		pr.Status.MarkRunning(v1.TaskRunReasonResolvingTaskRef, message)
		return nil
	case err != nil:
		return err
	default:
	}

	// Second iteration
	pipelineRunState, err = c.resolvePipelineState(ctx, notStartedTasks, pipelineMeta.ObjectMeta, pr, pipelineRunState)
	switch {
	case errors.Is(err, remote.ErrRequestInProgress):
		message := fmt.Sprintf("PipelineRun %s/%s awaiting remote resource", pr.Namespace, pr.Name)
		pr.Status.MarkRunning(v1.TaskRunReasonResolvingTaskRef, message)
		return nil
	case err != nil:
		return err
	default:
	}

	// Build PipelineRunFacts with a list of resolved pipeline tasks,
	// dag tasks graph and final tasks graph
	pipelineRunFacts := &resources.PipelineRunFacts{
		State:           pipelineRunState,
		SpecStatus:      pr.Spec.Status,
		TasksGraph:      d,
		FinalTasksGraph: dfinally,
		TimeoutsState: resources.PipelineRunTimeoutsState{
			Clock: c.Clock,
		},
	}
	if pr.Status.StartTime != nil {
		pipelineRunFacts.TimeoutsState.StartTime = &pr.Status.StartTime.Time
	}
	if pr.Status.FinallyStartTime != nil {
		pipelineRunFacts.TimeoutsState.FinallyStartTime = &pr.Status.FinallyStartTime.Time
	}
	tasksTimeout := pr.TasksTimeout()
	if tasksTimeout != nil {
		pipelineRunFacts.TimeoutsState.TasksTimeout = &tasksTimeout.Duration
	}
	finallyTimeout := pr.FinallyTimeout()
	if finallyTimeout != nil {
		pipelineRunFacts.TimeoutsState.FinallyTimeout = &finallyTimeout.Duration
	}
	if pipelineTimeout := pr.PipelineTimeout(ctx); pipelineTimeout != 0 {
		pipelineRunFacts.TimeoutsState.PipelineTimeout = &pipelineTimeout
	}

	for i, rpt := range pipelineRunFacts.State {
		if !rpt.IsCustomTask() {
			err := taskrun.ValidateResolvedTask(ctx, rpt.PipelineTask.Params, rpt.PipelineTask.Matrix, rpt.ResolvedTask)
			if err != nil {
				logger.Errorf("Failed to validate pipelinerun %s with error %w", pr.Name, err)
				pr.Status.MarkFailed(v1.PipelineRunReasonFailedValidation.String(),
					"Validation failed for pipelinerun %s with error %s",
					pr.Name, pipelineErrors.WrapUserError(err))
				return controller.NewPermanentError(err)
			}

			if config.FromContextOrDefaults(ctx).FeatureFlags.EnableParamEnum {
				if err := resources.ValidateParamEnumSubset(originalTasks[i].Params, pipelineSpec.Params, rpt.ResolvedTask); err != nil {
					logger.Errorf("Failed to validate pipelinerun %q with error %w", pr.Name, err)
					pr.Status.MarkFailed(v1.PipelineRunReasonFailedValidation.String(),
						"Validation failed for pipelinerun with error %s",
						pipelineErrors.WrapUserError(err))
					return controller.NewPermanentError(err)
				}
			}
		}
	}

	// Evaluate the CEL of PipelineTask after the variable substitutions and validations.
	for _, rpt := range pipelineRunFacts.State {
		err := rpt.EvaluateCEL()
		if err != nil {
			logger.Errorf("Error evaluating CEL %s: %v", pr.Name, err)
			pr.Status.MarkFailed(string(v1.PipelineRunReasonCELEvaluationFailed),
				"Error evaluating CEL %s: %w", pr.Name, pipelineErrors.WrapUserError(err))
			return controller.NewPermanentError(err)
		}
	}

	// check if pipeline run is not gracefully cancelled and there are active task runs, which require cancelling
	if pr.IsGracefullyCancelled() && pipelineRunFacts.IsRunning() {
		// If the pipelinerun is cancelled, cancel tasks, but run finally
		err := gracefullyCancelPipelineRun(ctx, logger, pr, c.PipelineClientSet)
		if err != nil {
			// failed to cancel tasks, maybe retry would help (don't return permanent error)
			return err
		}
	}

	if pipelineRunFacts.State.IsBeforeFirstTaskRun() {
		if err := resources.ValidatePipelineTaskResults(pipelineRunFacts.State); err != nil {
			logger.Errorf("Failed to resolve task result reference for %q with error %v", pr.Name, err)
			pr.Status.MarkFailed(v1.PipelineRunReasonInvalidTaskResultReference.String(), err.Error())
			return controller.NewPermanentError(err)
		}

		if err := resources.ValidatePipelineResults(pipelineSpec, pipelineRunFacts.State); err != nil {
			logger.Errorf("Failed to resolve pipeline result reference for %q with error %w", pr.Name, err)
			pr.Status.MarkFailed(v1.PipelineRunReasonInvalidPipelineResultReference.String(),
				"Failed to resolve pipeline result reference for %q with error %w",
				pr.Name, err)
			return controller.NewPermanentError(err)
		}

		if err := resources.ValidateOptionalWorkspaces(pipelineSpec.Workspaces, pipelineRunFacts.State); err != nil {
			logger.Errorf("Optional workspace not supported by task: %w", err)
			pr.Status.MarkFailed(v1.PipelineRunReasonRequiredWorkspaceMarkedOptional.String(),
				"Optional workspace not supported by task: %w", pipelineErrors.WrapUserError(err))
			return controller.NewPermanentError(err)
		}

		aaBehavior, err := affinityassistant.GetAffinityAssistantBehavior(ctx)
		if err != nil {
			return controller.NewPermanentError(err)
		}
		if err := c.createOrUpdateAffinityAssistantsAndPVCs(ctx, pr, aaBehavior); err != nil {
			switch {
			case errors.Is(err, ErrPvcCreationFailed):
				logger.Errorf("Failed to create PVC for PipelineRun %s: %v", pr.Name, err)
				pr.Status.MarkFailed(volumeclaim.ReasonCouldntCreateWorkspacePVC,
					"Failed to create PVC for PipelineRun %s/%s correctly: %s",
					pr.Namespace, pr.Name, err)
			case errors.Is(err, ErrAffinityAssistantCreationFailed):
				logger.Errorf("Failed to create affinity assistant StatefulSet for PipelineRun %s: %v", pr.Name, err)
				pr.Status.MarkFailed(ReasonCouldntCreateOrUpdateAffinityAssistantStatefulSet,
					"Failed to create StatefulSet for PipelineRun %s/%s correctly: %s",
					pr.Namespace, pr.Name, err)
			default:
			}
			return controller.NewPermanentError(err)
		}
	}

	if pr.Status.FinallyStartTime == nil {
		if pr.HaveTasksTimedOut(ctx, c.Clock) {
			tasksToTimeOut := sets.NewString()
			for _, pt := range pipelineRunFacts.State {
				if !pt.IsFinalTask(pipelineRunFacts) && pt.IsRunning() {
					tasksToTimeOut.Insert(pt.PipelineTask.Name)
				}
			}
			if tasksToTimeOut.Len() > 0 {
				logger.Debugf("PipelineRun tasks timeout of %s reached, cancelling tasks", tasksTimeout)
				errs := timeoutPipelineTasksForTaskNames(ctx, logger, pr, c.PipelineClientSet, tasksToTimeOut)
				if len(errs) > 0 {
					errString := strings.Join(errs, "\n")
					logger.Errorf("Failed to timeout tasks for PipelineRun %s/%s: %s", pr.Namespace, pr.Name, errString)
					return fmt.Errorf("error(s) from cancelling TaskRun(s) from PipelineRun %s: %s", pr.Name, errString)
				}
			}
		}
	} else if pr.HasFinallyTimedOut(ctx, c.Clock) {
		tasksToTimeOut := sets.NewString()
		for _, pt := range pipelineRunFacts.State {
			if pt.IsFinalTask(pipelineRunFacts) && pt.IsRunning() {
				tasksToTimeOut.Insert(pt.PipelineTask.Name)
			}
		}
		if tasksToTimeOut.Len() > 0 {
			logger.Debugf("PipelineRun finally timeout of %s reached, cancelling finally tasks", finallyTimeout)
			errs := timeoutPipelineTasksForTaskNames(ctx, logger, pr, c.PipelineClientSet, tasksToTimeOut)
			if len(errs) > 0 {
				errString := strings.Join(errs, "\n")
				logger.Errorf("Failed to timeout finally tasks for PipelineRun %s/%s: %s", pr.Namespace, pr.Name, errString)
				return fmt.Errorf("error(s) from cancelling TaskRun(s) from PipelineRun %s: %s", pr.Name, errString)
			}
		}
	}
	if err := c.runNextSchedulableTask(ctx, pr, pipelineRunFacts); err != nil {
		return err
	}

	// Reset the skipped status to trigger recalculation
	pipelineRunFacts.ResetSkippedCache()

	// If the pipelinerun has timed out, mark tasks as timed out and update status
	if pr.HasTimedOut(ctx, c.Clock) {
		if err := timeoutPipelineRun(ctx, logger, pr, c.PipelineClientSet); err != nil {
			return err
		}
	}

	after := pipelineRunFacts.GetPipelineConditionStatus(ctx, pr, logger, c.Clock)
	switch after.Status {
	case corev1.ConditionTrue:
		pr.Status.MarkSucceeded(after.Reason, after.Message)
	case corev1.ConditionFalse:
		pr.Status.MarkFailed(after.Reason, after.Message)
	case corev1.ConditionUnknown:
		pr.Status.MarkRunning(after.Reason, after.Message)
	}
	// Read the condition the way it was set by the Mark* helpers
	after = pr.Status.GetCondition(apis.ConditionSucceeded)
	pr.Status.StartTime = pipelineRunFacts.State.AdjustStartTime(pr.Status.StartTime)

	pr.Status.ChildReferences = pipelineRunFacts.GetChildReferences()

	pr.Status.SkippedTasks = pipelineRunFacts.GetSkippedTasks()

	taskStatus := pipelineRunFacts.GetPipelineTaskStatus()
	finalTaskStatus := pipelineRunFacts.GetPipelineFinalTaskStatus()
	taskStatus = kmap.Union(taskStatus, finalTaskStatus)

	if after.Status == corev1.ConditionTrue || after.Status == corev1.ConditionFalse {
		pr.Status.Results, err = resources.ApplyTaskResultsToPipelineResults(ctx, pipelineSpec.Results,
			pipelineRunFacts.State.GetTaskRunsResults(), pipelineRunFacts.State.GetRunsResults(), taskStatus)
		if err != nil {
			pr.Status.MarkFailed(v1.PipelineRunReasonCouldntGetPipelineResult.String(),
				"Failed to get PipelineResult from TaskRun Results for PipelineRun %s: %s",
				pr.Name, err)
			return err
		}
	}

	logger.Infof("PipelineRun %s status is being set to %s", pr.Name, after)
	return nil
}

// runNextSchedulableTask gets the next schedulable Tasks from the dag based on the current
// pipeline run state, and starts them
// after all DAG tasks are done, it's responsible for scheduling final tasks and start executing them
func (c *Reconciler) runNextSchedulableTask(ctx context.Context, pr *v1.PipelineRun, pipelineRunFacts *resources.PipelineRunFacts) error {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "runNextSchedulableTask")
	defer span.End()

	logger := logging.FromContext(ctx)
	recorder := controller.GetEventRecorder(ctx)

	// nextRpts holds a list of pipeline tasks which should be executed next
	nextRpts, err := pipelineRunFacts.DAGExecutionQueue()
	if err != nil {
		logger.Errorf("Error getting potential next tasks for valid pipelinerun %s: %v", pr.Name, err)
		return controller.NewPermanentError(err)
	}

	// Check for Missing Result References
	err = resources.CheckMissingResultReferences(pipelineRunFacts.State, nextRpts)
	if err != nil {
		logger.Infof("Failed to resolve task result reference for %q with error %v", pr.Name, err)
		pr.Status.MarkFailed(v1.PipelineRunReasonInvalidTaskResultReference.String(), err.Error())
		return controller.NewPermanentError(err)
	}

	// GetFinalTasks only returns final tasks when a DAG is complete
	fNextRpts := pipelineRunFacts.GetFinalTasks()
	if len(fNextRpts) != 0 {
		// apply the runtime context just before creating taskRuns for final tasks in queue
		resources.ApplyPipelineTaskStateContext(fNextRpts, pipelineRunFacts.GetPipelineTaskStatus())

		// Before creating TaskRun for scheduled final task, check if it's consuming a task result
		// Resolve and apply task result wherever applicable, report warning in case resolution fails
		for _, rpt := range fNextRpts {
			resolvedResultRefs, _, err := resources.ResolveResultRef(pipelineRunFacts.State, rpt)
			if err != nil {
				logger.Infof("Final task %q is not executed as it could not resolve task params for %q: %v", rpt.PipelineTask.Name, pr.Name, err)
				continue
			}
			resources.ApplyTaskResults(resources.PipelineRunState{rpt}, resolvedResultRefs)

			if err := rpt.EvaluateCEL(); err != nil {
				logger.Errorf("Final task %q is not executed, due to error evaluating CEL %s: %v", rpt.PipelineTask.Name, pr.Name, err)
				pr.Status.MarkFailed(string(v1.PipelineRunReasonCELEvaluationFailed),
					"Error evaluating CEL %s: %w", pr.Name, pipelineErrors.WrapUserError(err))
				return controller.NewPermanentError(err)
			}

			nextRpts = append(nextRpts, rpt)
		}
	}

	// If FinallyStartTime is not set, and one or more final tasks has been created
	// Try to set the FinallyStartTime of this PipelineRun
	if pr.Status.FinallyStartTime == nil && pipelineRunFacts.IsFinalTaskStarted() {
		c.setFinallyStartedTimeIfNeeded(pr, pipelineRunFacts)
	}

	resources.ApplyResultsToWorkspaceBindings(pipelineRunFacts.State.GetTaskRunsResults(), pr)

	for _, rpt := range nextRpts {
		if rpt.IsFinalTask(pipelineRunFacts) {
			c.setFinallyStartedTimeIfNeeded(pr, pipelineRunFacts)
		}

		if rpt == nil || rpt.Skip(pipelineRunFacts).IsSkipped || rpt.IsFinallySkipped(pipelineRunFacts).IsSkipped {
			continue
		}

		// propagate previous task results
		resources.PropagateResults(rpt, pipelineRunFacts.State)

		// propagate previous task artifacts
		err = resources.PropagateArtifacts(rpt, pipelineRunFacts.State)
		if err != nil {
			logger.Errorf("Failed to propagate artifacts due to error: %v", err)
			return controller.NewPermanentError(err)
		}

		// Validate parameter types in matrix after apply substitutions from Task Results
		if rpt.PipelineTask.IsMatrixed() {
			if err := resources.ValidateParameterTypesInMatrix(pipelineRunFacts.State); err != nil {
				logger.Errorf("Failed to validate matrix %q with error %w", pr.Name, err)
				pr.Status.MarkFailed(v1.PipelineRunReasonInvalidMatrixParameterTypes.String(),
					"Failed to validate matrix %q with error %w", pipelineErrors.WrapUserError(err))
				return controller.NewPermanentError(err)
			}
		}

		if rpt.IsCustomTask() {
			rpt.CustomRuns, err = c.createCustomRuns(ctx, rpt, pr, pipelineRunFacts)
			if err != nil {
				recorder.Eventf(pr, corev1.EventTypeWarning, "RunsCreationFailed", "Failed to create CustomRuns %q: %v", rpt.CustomRunNames, err)
				err = fmt.Errorf("error creating CustomRuns called %s for PipelineTask %s from PipelineRun %s: %w", rpt.CustomRunNames, rpt.PipelineTask.Name, pr.Name, err)
				return err
			}
		} else {
			rpt.TaskRuns, err = c.createTaskRuns(ctx, rpt, pr, pipelineRunFacts)
			if err != nil {
				recorder.Eventf(pr, corev1.EventTypeWarning, "TaskRunsCreationFailed", "Failed to create TaskRuns %q: %v", rpt.TaskRunNames, err)
				err = fmt.Errorf("error creating TaskRuns called %s for PipelineTask %s from PipelineRun %s: %w", rpt.TaskRunNames, rpt.PipelineTask.Name, pr.Name, err)
				return err
			}
		}
	}
	return nil
}

// setFinallyStartedTimeIfNeeded sets the PipelineRun.Status.FinallyStartedTime to the current time if it's nil.
func (c *Reconciler) setFinallyStartedTimeIfNeeded(pr *v1.PipelineRun, facts *resources.PipelineRunFacts) {
	if pr.Status.FinallyStartTime == nil {
		pr.Status.FinallyStartTime = &metav1.Time{Time: c.Clock.Now()}
	}
	if facts.TimeoutsState.FinallyStartTime == nil {
		facts.TimeoutsState.FinallyStartTime = &pr.Status.FinallyStartTime.Time
	}
}

func (c *Reconciler) createTaskRuns(ctx context.Context, rpt *resources.ResolvedPipelineTask, pr *v1.PipelineRun, facts *resources.PipelineRunFacts) ([]*v1.TaskRun, error) {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "createTaskRuns")
	defer span.End()
	var taskRuns []*v1.TaskRun
	var matrixCombinations []v1.Params
	if rpt.PipelineTask.IsMatrixed() {
		matrixCombinations = rpt.PipelineTask.Matrix.FanOut()
	}
	// validate the param values meet resolved Task Param Enum requirements before creating TaskRuns
	if config.FromContextOrDefaults(ctx).FeatureFlags.EnableParamEnum {
		for i := range rpt.TaskRunNames {
			var params v1.Params
			if len(matrixCombinations) > i {
				params = matrixCombinations[i]
			}
			params = append(params, rpt.PipelineTask.Params...)
			if err := taskrun.ValidateEnumParam(ctx, params, rpt.ResolvedTask.TaskSpec.Params); err != nil {
				pr.Status.MarkFailed(v1.PipelineRunReasonInvalidParamValue.String(),
					"Invalid param value from PipelineTask \"%s\": %w",
					rpt.PipelineTask.Name, pipelineErrors.WrapUserError(err))
				return nil, controller.NewPermanentError(err)
			}
		}
	}
	for i, taskRunName := range rpt.TaskRunNames {
		var params v1.Params
		if len(matrixCombinations) > i {
			params = matrixCombinations[i]
		}
		taskRun, err := c.createTaskRun(ctx, taskRunName, params, rpt, pr, facts)
		if err != nil {
			err := c.handleRunCreationError(ctx, pr, err)
			return nil, err
		}
		taskRuns = append(taskRuns, taskRun)
	}
	return taskRuns, nil
}

func (c *Reconciler) createTaskRun(ctx context.Context, taskRunName string, params v1.Params, rpt *resources.ResolvedPipelineTask, pr *v1.PipelineRun, facts *resources.PipelineRunFacts) (*v1.TaskRun, error) {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "createTaskRun")
	defer span.End()
	logger := logging.FromContext(ctx)
	rpt.PipelineTask = resources.ApplyPipelineTaskContexts(rpt.PipelineTask, pr.Status, facts)
	taskRunSpec := pr.GetTaskRunSpec(rpt.PipelineTask.Name)
	params = append(params, rpt.PipelineTask.Params...)
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:            taskRunName,
			Namespace:       pr.Namespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(pr)},
			Labels:          combineTaskRunAndTaskSpecLabels(pr, rpt.PipelineTask),
			Annotations:     combineTaskRunAndTaskSpecAnnotations(pr, rpt.PipelineTask),
		},
		Spec: v1.TaskRunSpec{
			Retries:            rpt.PipelineTask.Retries,
			Params:             params,
			ServiceAccountName: taskRunSpec.ServiceAccountName,
			PodTemplate:        taskRunSpec.PodTemplate,
			StepSpecs:          taskRunSpec.StepSpecs,
			SidecarSpecs:       taskRunSpec.SidecarSpecs,
			ComputeResources:   taskRunSpec.ComputeResources,
		},
	}

	// Add current spanContext as annotations to TaskRun
	// so that tracing can be continued under the same traceId
	if spanContext, err := getMarshalledSpanFromContext(ctx); err == nil {
		tr.Annotations[TaskRunSpanContextAnnotation] = spanContext
	}
	if rpt.PipelineTask.OnError == v1.PipelineTaskContinue {
		tr.Annotations[v1.PipelineTaskOnErrorAnnotation] = string(v1.PipelineTaskContinue)
	}

	if rpt.PipelineTask.Timeout != nil {
		tr.Spec.Timeout = rpt.PipelineTask.Timeout
	}

	if rpt.ResolvedTask.TaskName != "" {
		// We pass the entire, original task ref because it may contain additional references like a Bundle url.
		tr.Spec.TaskRef = rpt.PipelineTask.TaskRef
	} else if rpt.ResolvedTask.TaskSpec != nil {
		tr.Spec.TaskSpec = rpt.ResolvedTask.TaskSpec
	}

	var pipelinePVCWorkspaceName string
	var err error
	tr.Spec.Workspaces, pipelinePVCWorkspaceName, err = c.getTaskrunWorkspaces(ctx, pr, rpt)
	if err != nil {
		return nil, err
	}

	aaBehavior, err := affinityassistant.GetAffinityAssistantBehavior(ctx)
	if err != nil {
		return nil, err
	}
	if aaAnnotationVal := getAffinityAssistantAnnotationVal(aaBehavior, pipelinePVCWorkspaceName, pr.Name); aaAnnotationVal != "" {
		tr.Annotations[workspace.AnnotationAffinityAssistantName] = aaAnnotationVal
	}

	logger.Infof("Creating a new TaskRun object %s for pipeline task %s", taskRunName, rpt.PipelineTask.Name)
	return c.PipelineClientSet.TektonV1().TaskRuns(pr.Namespace).Create(ctx, tr, metav1.CreateOptions{})
}

// handleRunCreationError marks the PipelineRun as failed and returns a permanent error if the run creation error is not retryable
func (c *Reconciler) handleRunCreationError(ctx context.Context, pr *v1.PipelineRun, err error) error {
	if controller.IsPermanentError(err) {
		pr.Status.MarkFailed(v1.PipelineRunReasonCreateRunFailed.String(), err.Error())
		return err
	}
	// This is not a complete list of permanent errors. Any permanent error with TaskRun/CustomRun creation can be added here.
	if apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) {
		pr.Status.MarkFailed(v1.PipelineRunReasonCreateRunFailed.String(), err.Error())
		return controller.NewPermanentError(err)
	}
	return err
}

func (c *Reconciler) createCustomRuns(ctx context.Context, rpt *resources.ResolvedPipelineTask, pr *v1.PipelineRun, facts *resources.PipelineRunFacts) ([]*v1beta1.CustomRun, error) {
	var customRuns []*v1beta1.CustomRun
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "createCustomRuns")
	defer span.End()
	var matrixCombinations []v1.Params

	if rpt.PipelineTask.IsMatrixed() {
		matrixCombinations = rpt.PipelineTask.Matrix.FanOut()
	}
	for i, customRunName := range rpt.CustomRunNames {
		var params v1.Params
		if len(matrixCombinations) > i {
			params = matrixCombinations[i]
		}
		customRun, err := c.createCustomRun(ctx, customRunName, params, rpt, pr, facts)
		if err != nil {
			err := c.handleRunCreationError(ctx, pr, err)
			return nil, err
		}
		customRuns = append(customRuns, customRun)
	}
	return customRuns, nil
}

func (c *Reconciler) createCustomRun(ctx context.Context, runName string, params v1.Params, rpt *resources.ResolvedPipelineTask, pr *v1.PipelineRun, facts *resources.PipelineRunFacts) (*v1beta1.CustomRun, error) {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "createCustomRun")
	defer span.End()
	logger := logging.FromContext(ctx)
	rpt.PipelineTask = resources.ApplyPipelineTaskContexts(rpt.PipelineTask, pr.Status, facts)
	taskRunSpec := pr.GetTaskRunSpec(rpt.PipelineTask.Name)
	params = append(params, rpt.PipelineTask.Params...)

	taskTimeout := rpt.PipelineTask.Timeout
	var pipelinePVCWorkspaceName string
	var err error
	var workspaces []v1.WorkspaceBinding
	workspaces, pipelinePVCWorkspaceName, err = c.getTaskrunWorkspaces(ctx, pr, rpt)
	if err != nil {
		return nil, err
	}

	objectMeta := metav1.ObjectMeta{
		Name:            runName,
		Namespace:       pr.Namespace,
		OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(pr)},
		Labels:          getTaskrunLabels(pr, rpt.PipelineTask.Name, true),
		Annotations:     getTaskrunAnnotations(pr),
	}

	// TaskRef, Params and Workspaces are converted to v1beta1 since CustomRuns
	// is still in v1beta1 apiVersion
	var customRef *v1beta1.TaskRef
	if rpt.PipelineTask.TaskRef != nil {
		customRef = &v1beta1.TaskRef{}
		customRef.ConvertFrom(ctx, *rpt.PipelineTask.TaskRef)
	}

	customRunParams := v1beta1.Params{}
	for _, p := range params {
		v1beta1Param := v1beta1.Param{}
		v1beta1Param.ConvertFrom(ctx, p)
		customRunParams = append(customRunParams, v1beta1Param)
	}

	customRunWorkspaces := []v1beta1.WorkspaceBinding{}
	for _, w := range workspaces {
		v1beta1WorkspaceBinding := v1beta1.WorkspaceBinding{}
		v1beta1WorkspaceBinding.ConvertFrom(ctx, w)
		customRunWorkspaces = append(customRunWorkspaces, v1beta1WorkspaceBinding)
	}

	r := &v1beta1.CustomRun{
		ObjectMeta: objectMeta,
		Spec: v1beta1.CustomRunSpec{
			Retries:            rpt.PipelineTask.Retries,
			CustomRef:          customRef,
			Params:             customRunParams,
			ServiceAccountName: taskRunSpec.ServiceAccountName,
			Timeout:            taskTimeout,
			Workspaces:         customRunWorkspaces,
		},
	}

	if rpt.PipelineTask.TaskSpec != nil {
		j, err := json.Marshal(rpt.PipelineTask.TaskSpec.Spec)
		if err != nil {
			return nil, err
		}
		r.Spec.CustomSpec = &v1beta1.EmbeddedCustomRunSpec{
			TypeMeta: runtime.TypeMeta{
				APIVersion: rpt.PipelineTask.TaskSpec.APIVersion,
				Kind:       rpt.PipelineTask.TaskSpec.Kind,
			},
			Metadata: v1beta1.PipelineTaskMetadata(rpt.PipelineTask.TaskSpec.Metadata),
			Spec: runtime.RawExtension{
				Raw: j,
			},
		}
	}

	// Set the affinity assistant annotation in case the custom task creates TaskRuns or Pods
	// that can take advantage of it.
	aaBehavior, err := affinityassistant.GetAffinityAssistantBehavior(ctx)
	if err != nil {
		return nil, err
	}
	if aaAnnotationVal := getAffinityAssistantAnnotationVal(aaBehavior, pipelinePVCWorkspaceName, pr.Name); aaAnnotationVal != "" {
		r.Annotations[workspace.AnnotationAffinityAssistantName] = aaAnnotationVal
	}

	logger.Infof("Creating a new CustomRun object %s", runName)
	return c.PipelineClientSet.TektonV1beta1().CustomRuns(pr.Namespace).Create(ctx, r, metav1.CreateOptions{})
}

// propagateWorkspaces identifies the workspaces that the pipeline task usess
// It adds the additional workspaces to the pipeline task's workspaces after
// creating workspace bindings. Finally, it returns the updated resolved pipeline task.
func propagateWorkspaces(rpt *resources.ResolvedPipelineTask) (*resources.ResolvedPipelineTask, error) {
	ts := rpt.PipelineTask.TaskSpec.TaskSpec

	workspacesUsedInSteps, err := workspace.FindWorkspacesUsedByTask(ts)
	if err != nil {
		return rpt, err
	}

	ptw := sets.NewString()
	for _, ws := range rpt.PipelineTask.Workspaces {
		ptw.Insert(ws.Name)
	}

	for wSpace := range workspacesUsedInSteps {
		if !ptw.Has(wSpace) {
			rpt.PipelineTask.Workspaces = append(rpt.PipelineTask.Workspaces, v1.WorkspacePipelineTaskBinding{Name: wSpace})
		}
	}
	return rpt, nil
}

func (c *Reconciler) getTaskrunWorkspaces(ctx context.Context, pr *v1.PipelineRun, rpt *resources.ResolvedPipelineTask) ([]v1.WorkspaceBinding, string, error) {
	var err error
	var workspaces []v1.WorkspaceBinding
	var pipelinePVCWorkspaceName string
	pipelineRunWorkspaces := make(map[string]v1.WorkspaceBinding)
	for _, binding := range pr.Spec.Workspaces {
		pipelineRunWorkspaces[binding.Name] = binding
	}

	// Propagate required workspaces from pipelineRun to the pipelineTasks
	if rpt.PipelineTask.TaskSpec != nil {
		rpt, err = propagateWorkspaces(rpt)
		if err != nil {
			// This error cannot be recovered without modifying the TaskSpec
			return nil, "", controller.NewPermanentError(err)
		}
	}

	for _, ws := range rpt.PipelineTask.Workspaces {
		taskWorkspaceName, pipelineTaskSubPath, pipelineWorkspaceName := ws.Name, ws.SubPath, ws.Workspace
		pipelineWorkspace := pipelineWorkspaceName

		if pipelineWorkspaceName == "" {
			pipelineWorkspace = taskWorkspaceName
		}

		if b, hasBinding := pipelineRunWorkspaces[pipelineWorkspace]; hasBinding {
			if b.PersistentVolumeClaim != nil || b.VolumeClaimTemplate != nil {
				pipelinePVCWorkspaceName = pipelineWorkspace
			}

			aaBehavior, err := affinityassistant.GetAffinityAssistantBehavior(ctx)
			if err != nil {
				return nil, "", err
			}

			workspace := c.taskWorkspaceByWorkspaceVolumeSource(ctx, pipelinePVCWorkspaceName, pr.Name, b, taskWorkspaceName, pipelineTaskSubPath, *kmeta.NewControllerRef(pr), aaBehavior)
			workspaces = append(workspaces, workspace)
		} else {
			workspaceIsOptional := false
			if rpt.ResolvedTask != nil && rpt.ResolvedTask.TaskSpec != nil {
				for _, taskWorkspaceDeclaration := range rpt.ResolvedTask.TaskSpec.Workspaces {
					if taskWorkspaceDeclaration.Name == taskWorkspaceName && taskWorkspaceDeclaration.Optional {
						workspaceIsOptional = true
						break
					}
				}
			}
			if !workspaceIsOptional {
				err = fmt.Errorf("expected workspace %q to be provided by pipelinerun for pipeline task %q", pipelineWorkspace, rpt.PipelineTask.Name)
				// This error cannot be recovered without modifying the PipelineRun
				return nil, "", controller.NewPermanentError(err)
			}
		}
	}

	// replace pipelineRun context variables in workspace subPath in the workspace binding
	var p string
	if pr.Spec.PipelineRef != nil {
		p = pr.Spec.PipelineRef.Name
	}
	for j := range workspaces {
		workspaces[j].SubPath = substitution.ApplyReplacements(workspaces[j].SubPath, resources.GetContextReplacements(p, pr))
	}

	return workspaces, pipelinePVCWorkspaceName, nil
}

// taskWorkspaceByWorkspaceVolumeSource returns the WorkspaceBinding to be bound to each TaskRun in the Pipeline Task.
// If the PipelineRun WorkspaceBinding is a volumeClaimTemplate, the returned WorkspaceBinding references a PersistentVolumeClaim created for the PipelineRun WorkspaceBinding based on the PipelineRun as OwnerReference.
// Otherwise, the returned WorkspaceBinding references the same volume as the PipelineRun WorkspaceBinding, with the file path joined with pipelineTaskSubPath as the binding subpath.
func (c *Reconciler) taskWorkspaceByWorkspaceVolumeSource(ctx context.Context, pipelineWorkspaceName string, prName string, wb v1.WorkspaceBinding, taskWorkspaceName string, pipelineTaskSubPath string, owner metav1.OwnerReference, aaBehavior affinityassistant.AffinityAssistantBehavior) v1.WorkspaceBinding {
	if wb.VolumeClaimTemplate == nil {
		binding := *wb.DeepCopy()
		binding.Name = taskWorkspaceName
		binding.SubPath = combinedSubPath(wb.SubPath, pipelineTaskSubPath)
		return binding
	}

	binding := v1.WorkspaceBinding{
		SubPath:               combinedSubPath(wb.SubPath, pipelineTaskSubPath),
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
	}
	binding.Name = taskWorkspaceName

	switch aaBehavior {
	case affinityassistant.AffinityAssistantPerWorkspace, affinityassistant.AffinityAssistantDisabled:
		binding.PersistentVolumeClaim.ClaimName = volumeclaim.GeneratePVCNameFromWorkspaceBinding(wb.VolumeClaimTemplate.Name, wb, owner)
	case affinityassistant.AffinityAssistantPerPipelineRun, affinityassistant.AffinityAssistantPerPipelineRunWithIsolation:
		binding.PersistentVolumeClaim.ClaimName = getPersistentVolumeClaimNameWithAffinityAssistant("", prName, wb, owner)
	}

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

func getTaskrunAnnotations(pr *v1.PipelineRun) map[string]string {
	// Propagate annotations from PipelineRun to TaskRun.
	annotations := make(map[string]string, len(pr.ObjectMeta.Annotations)+1)
	for key, val := range pr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	return kmap.Filter(annotations, func(s string) bool {
		return filterReservedAnnotationRegexp.MatchString(s)
	})
}

func propagatePipelineNameLabelToPipelineRun(pr *v1.PipelineRun) error {
	if pr.ObjectMeta.Labels == nil {
		pr.ObjectMeta.Labels = make(map[string]string)
	}

	if _, ok := pr.ObjectMeta.Labels[pipeline.PipelineLabelKey]; ok {
		return nil
	}

	switch {
	case pr.Spec.PipelineRef != nil && pr.Spec.PipelineRef.Name != "":
		pr.ObjectMeta.Labels[pipeline.PipelineLabelKey] = pr.Spec.PipelineRef.Name
	case pr.Spec.PipelineSpec != nil:
		pr.ObjectMeta.Labels[pipeline.PipelineLabelKey] = pr.Name
	case pr.Spec.PipelineRef != nil && pr.Spec.PipelineRef.Resolver != "":
		pr.ObjectMeta.Labels[pipeline.PipelineLabelKey] = pr.Name

		// https://tekton.dev/docs/pipelines/cluster-resolver/#pipeline-resolution
		var kind, name string
		for _, param := range pr.Spec.PipelineRef.Params {
			if param.Name == "kind" {
				kind = param.Value.StringVal
			}
			if param.Name == "name" {
				name = param.Value.StringVal
			}
		}
		if kind == "pipeline" {
			pr.ObjectMeta.Labels[pipeline.PipelineLabelKey] = name
		}
	default:
		return fmt.Errorf("pipelineRun %s not providing PipelineRef or PipelineSpec", pr.Name)
	}
	return nil
}

func getTaskrunLabels(pr *v1.PipelineRun, pipelineTaskName string, includePipelineLabels bool) map[string]string {
	// Propagate labels from PipelineRun to TaskRun.
	labels := make(map[string]string, len(pr.ObjectMeta.Labels)+1)
	if includePipelineLabels {
		for key, val := range pr.ObjectMeta.Labels {
			labels[key] = val
		}
	}
	labels[pipeline.PipelineRunLabelKey] = pr.Name
	if pipelineTaskName != "" {
		labels[pipeline.PipelineTaskLabelKey] = pipelineTaskName
	}
	if pr.Status.PipelineSpec != nil {
		// check if a task is part of the "tasks" section, add a label to identify it during the runtime
		for _, f := range pr.Status.PipelineSpec.Tasks {
			if pipelineTaskName == f.Name {
				labels[pipeline.MemberOfLabelKey] = v1.PipelineTasks
				break
			}
		}
		// check if a task is part of the "finally" section, add a label to identify it during the runtime
		for _, f := range pr.Status.PipelineSpec.Finally {
			if pipelineTaskName == f.Name {
				labels[pipeline.MemberOfLabelKey] = v1.PipelineFinallyTasks
				break
			}
		}
	}
	return labels
}

func combineTaskRunAndTaskSpecLabels(pr *v1.PipelineRun, pipelineTask *v1.PipelineTask) map[string]string {
	labels := make(map[string]string)

	taskRunSpec := pr.GetTaskRunSpec(pipelineTask.Name)
	if taskRunSpec.Metadata != nil {
		addMetadataByPrecedence(labels, taskRunSpec.Metadata.Labels)
	}

	addMetadataByPrecedence(labels, getTaskrunLabels(pr, pipelineTask.Name, true))

	if pipelineTask.TaskSpec != nil {
		addMetadataByPrecedence(labels, pipelineTask.TaskSpecMetadata().Labels)
	}

	return labels
}

func combineTaskRunAndTaskSpecAnnotations(pr *v1.PipelineRun, pipelineTask *v1.PipelineTask) map[string]string {
	annotations := make(map[string]string)

	taskRunSpec := pr.GetTaskRunSpec(pipelineTask.Name)
	if taskRunSpec.Metadata != nil {
		addMetadataByPrecedence(annotations, taskRunSpec.Metadata.Annotations)
	}

	addMetadataByPrecedence(annotations, getTaskrunAnnotations(pr))

	if pipelineTask.TaskSpec != nil {
		addMetadataByPrecedence(annotations, pipelineTask.TaskSpecMetadata().Annotations)
	}

	return annotations
}

// addMetadataByPrecedence() adds the elements in addedMetadata to metadata. If the same key is present in both maps, the value from metadata will be used.
func addMetadataByPrecedence(metadata map[string]string, addedMetadata map[string]string) {
	for key, value := range addedMetadata {
		// add new annotations/labels  if the key not exists in current ones
		if _, ok := metadata[key]; !ok {
			metadata[key] = value
		}
	}
}

func (c *Reconciler) updateLabelsAndAnnotations(ctx context.Context, pr *v1.PipelineRun) (*v1.PipelineRun, error) {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "updateLabelsAndAnnotations")
	defer span.End()
	newPr, err := c.pipelineRunLister.PipelineRuns(pr.Namespace).Get(pr.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting PipelineRun %s when updating labels/annotations: %w", pr.Name, err)
	}
	if !reflect.DeepEqual(pr.ObjectMeta.Labels, newPr.ObjectMeta.Labels) || !reflect.DeepEqual(pr.ObjectMeta.Annotations, newPr.ObjectMeta.Annotations) {
		// Note that this uses Update vs. Patch because the former is significantly easier to test.
		// If we want to switch this to Patch, then we will need to teach the utilities in test/controller.go
		// to deal with Patch (setting resourceVersion, and optimistic concurrency checks).
		newPr = newPr.DeepCopy()
		// Properly merge labels and annotations, as the labels *might* have changed during the reconciliation
		newPr.Labels = kmap.Union(newPr.Labels, pr.Labels)
		newPr.Annotations = kmap.Union(newPr.Annotations, pr.Annotations)
		return c.PipelineClientSet.TektonV1().PipelineRuns(pr.Namespace).Update(ctx, newPr, metav1.UpdateOptions{})
	}
	return newPr, nil
}

func storePipelineSpecAndMergeMeta(ctx context.Context, pr *v1.PipelineRun, ps *v1.PipelineSpec, meta *resolutionutil.ResolvedObjectMeta) error {
	// Only store the PipelineSpec once, if it has never been set before.
	if pr.Status.PipelineSpec == nil {
		pr.Status.PipelineSpec = ps
		if meta == nil {
			return nil
		}

		// Propagate labels from Pipeline to PipelineRun. PipelineRun labels take precedences over Pipeline.
		pr.ObjectMeta.Labels = kmap.Union(meta.Labels, pr.ObjectMeta.Labels)
		if len(meta.Name) > 0 {
			pr.ObjectMeta.Labels[pipeline.PipelineLabelKey] = meta.Name
		}

		// Propagate annotations from Pipeline to PipelineRun. PipelineRun annotations take precedences over Pipeline.
		pr.ObjectMeta.Annotations = kmap.Union(kmap.ExcludeKeys(meta.Annotations, tknreconciler.KubectlLastAppliedAnnotationKey), pr.ObjectMeta.Annotations)
	}

	// Propagate refSource from remote resolution to PipelineRun Status
	// This lives outside of the status.spec check to avoid the case where only the spec is available in the first reconcile and source comes in next reconcile.
	cfg := config.FromContextOrDefaults(ctx)
	if cfg.FeatureFlags.EnableProvenanceInStatus {
		if pr.Status.Provenance == nil {
			pr.Status.Provenance = &v1.Provenance{}
		}
		// Store FeatureFlags in the Provenance.
		pr.Status.Provenance.FeatureFlags = cfg.FeatureFlags

		if meta != nil && meta.RefSource != nil && pr.Status.Provenance.RefSource == nil {
			pr.Status.Provenance.RefSource = meta.RefSource
		}
	}

	return nil
}

func (c *Reconciler) updatePipelineRunStatusFromInformer(ctx context.Context, pr *v1.PipelineRun) error {
	ctx, span := c.tracerProvider.Tracer(TracerName).Start(ctx, "updatePipelineRunStatusFromInformer")
	defer span.End()
	logger := logging.FromContext(ctx)

	// Get the pipelineRun label that is set on each TaskRun.  Do not include the propagated labels from the
	// Pipeline and PipelineRun.  The user could change them during the lifetime of the PipelineRun so the
	// current labels may not be set on the previously created TaskRuns.
	pipelineRunLabels := getTaskrunLabels(pr, "", false)
	taskRuns, err := c.taskRunLister.TaskRuns(pr.Namespace).List(k8slabels.SelectorFromSet(pipelineRunLabels))
	if err != nil {
		logger.Errorf("Could not list TaskRuns %#v", err)
		return err
	}

	customRuns, err := c.customRunLister.CustomRuns(pr.Namespace).List(k8slabels.SelectorFromSet(pipelineRunLabels))
	if err != nil {
		logger.Errorf("Could not list CustomRuns %#v", err)
		return err
	}
	return updatePipelineRunStatusFromChildObjects(ctx, logger, pr, taskRuns, customRuns)
}

func updatePipelineRunStatusFromChildObjects(ctx context.Context, logger *zap.SugaredLogger, pr *v1.PipelineRun, taskRuns []*v1.TaskRun, customRuns []*v1beta1.CustomRun) error {
	updatePipelineRunStatusFromChildRefs(logger, pr, taskRuns, customRuns)

	return validateChildObjectsInPipelineRunStatus(ctx, pr.Status)
}

func validateChildObjectsInPipelineRunStatus(ctx context.Context, prs v1.PipelineRunStatus) error {
	var err error

	for _, cr := range prs.ChildReferences {
		switch cr.Kind {
		case taskRun, customRun:
			continue
		default:
			err = multierror.Append(err, fmt.Errorf("child with name %s has unknown kind %s", cr.Name, cr.Kind))
		}
	}

	return err
}

// filterTaskRunsForPipelineRunStatus returns TaskRuns owned by the PipelineRun.
func filterTaskRunsForPipelineRunStatus(logger *zap.SugaredLogger, pr *v1.PipelineRun, trs []*v1.TaskRun) []*v1.TaskRun {
	var ownedTaskRuns []*v1.TaskRun

	for _, tr := range trs {
		// Only process TaskRuns that are owned by this PipelineRun.
		// This skips TaskRuns that are indirectly created by the PipelineRun (e.g. by custom tasks).
		if len(tr.OwnerReferences) < 1 || tr.OwnerReferences[0].UID != pr.ObjectMeta.UID {
			logger.Debugf("Found a TaskRun %s that is not owned by this PipelineRun", tr.Name)
			continue
		}
		ownedTaskRuns = append(ownedTaskRuns, tr)
	}

	return ownedTaskRuns
}

// filterCustomRunsForPipelineRunStatus filters the given slice of customRuns, returning information only those owned by the given PipelineRun.
func filterCustomRunsForPipelineRunStatus(logger *zap.SugaredLogger, pr *v1.PipelineRun, customRuns []*v1beta1.CustomRun) ([]string, []string, []schema.GroupVersionKind, []*v1beta1.CustomRunStatus) {
	var names []string
	var taskLabels []string
	var gvks []schema.GroupVersionKind
	var statuses []*v1beta1.CustomRunStatus

	// Loop over all the customRuns associated to Tasks
	for _, cr := range customRuns {
		// Only process customRuns that are owned by this PipelineRun.
		// This skips customRuns that are indirectly created by the PipelineRun (e.g. by custom tasks).
		if len(cr.GetObjectMeta().GetOwnerReferences()) < 1 || cr.GetObjectMeta().GetOwnerReferences()[0].UID != pr.ObjectMeta.UID {
			logger.Debugf("Found a %s %s that is not owned by this PipelineRun", cr.GetObjectKind().GroupVersionKind().Kind, cr.GetObjectMeta().GetName())
			continue
		}

		names = append(names, cr.GetObjectMeta().GetName())
		taskLabels = append(taskLabels, cr.GetObjectMeta().GetLabels()[pipeline.PipelineTaskLabelKey])

		statuses = append(statuses, &cr.Status)
		// We can't just get the gvk from the customRun's TypeMeta because that isn't populated for resources created through the fake client.
		gvks = append(gvks, v1beta1.SchemeGroupVersion.WithKind(customRun))
	}

	// NAMES are names

	return names, taskLabels, gvks, statuses
}

func updatePipelineRunStatusFromChildRefs(logger *zap.SugaredLogger, pr *v1.PipelineRun, trs []*v1.TaskRun, customRuns []*v1beta1.CustomRun) {
	// If no TaskRun or CustomRun was found, nothing to be done. We never remove child references from the status.
	// We do still return an empty map of TaskRun/Run names keyed by PipelineTask name for later functions.
	if len(trs) == 0 && len(customRuns) == 0 {
		return
	}

	// Map PipelineTask names to TaskRun child references that were already in the status
	childRefByName := make(map[string]*v1.ChildStatusReference)

	for i := range pr.Status.ChildReferences {
		childRefByName[pr.Status.ChildReferences[i].Name] = &pr.Status.ChildReferences[i]
	}

	taskRuns := filterTaskRunsForPipelineRunStatus(logger, pr, trs)

	// Loop over all the TaskRuns associated to Tasks
	for _, tr := range taskRuns {
		lbls := tr.GetLabels()
		pipelineTaskName := lbls[pipeline.PipelineTaskLabelKey]

		if _, ok := childRefByName[tr.Name]; !ok {
			// This tr was missing from the status.
			// Add it without conditions, which are handled in the next loop
			logger.Infof("Found a TaskRun %s that was missing from the PipelineRun status", tr.Name)

			// Since this was recovered now, add it to the map, or it might be overwritten
			childRefByName[tr.Name] = &v1.ChildStatusReference{
				TypeMeta: runtime.TypeMeta{
					APIVersion: v1.SchemeGroupVersion.String(),
					Kind:       taskRun,
				},
				Name:             tr.Name,
				PipelineTaskName: pipelineTaskName,
			}
		}
	}

	// Get the names, their task label values, and their group/version/kind info for all CustomRuns or Runs associated with the PipelineRun
	names, taskLabels, gvks, _ := filterCustomRunsForPipelineRunStatus(logger, pr, customRuns)

	// Loop over that data and populate the child references
	for idx := range names {
		name := names[idx]
		taskLabel := taskLabels[idx]
		gvk := gvks[idx]

		if _, ok := childRefByName[name]; !ok {
			// This run was missing from the status.
			// Add it without conditions, which are handled in the next loop
			logger.Infof("Found a %s %s that was missing from the PipelineRun status", gvk.Kind, name)

			// Since this was recovered now, add it to the map, or it might be overwritten
			childRefByName[name] = &v1.ChildStatusReference{
				TypeMeta: runtime.TypeMeta{
					APIVersion: gvk.GroupVersion().String(),
					Kind:       gvk.Kind,
				},
				Name:             name,
				PipelineTaskName: taskLabel,
			}
		}
	}

	var newChildRefs []v1.ChildStatusReference
	for k := range childRefByName {
		newChildRefs = append(newChildRefs, *childRefByName[k])
	}
	pr.Status.ChildReferences = newChildRefs
}

// conditionFromVerificationResult returns the ConditionTrustedResourcesVerified condition based on the VerificationResult, err is returned when the VerificationResult type is VerificationError
func conditionFromVerificationResult(verificationResult *trustedresources.VerificationResult, pr *v1.PipelineRun, resourceName string) (*apis.Condition, error) {
	var condition *apis.Condition
	var err error
	switch verificationResult.VerificationResultType {
	case trustedresources.VerificationError:
		err = fmt.Errorf("pipelineRun %s/%s referred resource %s failed signature verification: %w", pr.Namespace, pr.Name, resourceName, verificationResult.Err)
		condition = &apis.Condition{
			Type:    trustedresources.ConditionTrustedResourcesVerified,
			Status:  corev1.ConditionFalse,
			Message: err.Error(),
		}
	case trustedresources.VerificationWarn:
		condition = &apis.Condition{
			Type:    trustedresources.ConditionTrustedResourcesVerified,
			Status:  corev1.ConditionFalse,
			Message: verificationResult.Err.Error(),
		}
	case trustedresources.VerificationPass:
		condition = &apis.Condition{
			Type:   trustedresources.ConditionTrustedResourcesVerified,
			Status: corev1.ConditionTrue,
		}
	case trustedresources.VerificationSkip:
		// do nothing
	}
	return condition, err
}
