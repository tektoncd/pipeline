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

package pod

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/tektoncd/pipeline/internal/sidecarlogresults"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/pkg/termination"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
)

const (
	// ReasonCouldntGetTask indicates that the reason for the failure status is that the
	// Task couldn't be found
	ReasonCouldntGetTask = "CouldntGetTask"

	// ReasonFailedResolution indicated that the reason for failure status is
	// that references within the TaskRun could not be resolved
	ReasonFailedResolution = "TaskRunResolutionFailed"

	// ReasonFailedValidation indicated that the reason for failure status is
	// that taskrun failed runtime validation
	ReasonFailedValidation = "TaskRunValidationFailed"

	// ReasonExceededResourceQuota indicates that the TaskRun failed to create a pod due to
	// a ResourceQuota in the namespace
	ReasonExceededResourceQuota = "ExceededResourceQuota"

	// ReasonExceededNodeResources indicates that the TaskRun's pod has failed to start due
	// to resource constraints on the node
	ReasonExceededNodeResources = "ExceededNodeResources"

	// ReasonPullImageFailed indicates that the TaskRun's pod failed to pull image
	ReasonPullImageFailed = "PullImageFailed"

	// ReasonCreateContainerConfigError indicates that the TaskRun failed to create a pod due to
	// config error of container
	ReasonCreateContainerConfigError = "CreateContainerConfigError"

	// ReasonPodCreationFailed indicates that the reason for the current condition
	// is that the creation of the pod backing the TaskRun failed
	ReasonPodCreationFailed = "PodCreationFailed"

	// ReasonPodAdmissionFailed indicates that the TaskRun's pod failed to pass admission validation
	ReasonPodAdmissionFailed = "PodAdmissionFailed"

	// ReasonPending indicates that the pod is in corev1.Pending, and the reason is not
	// ReasonExceededNodeResources or isPodHitConfigError
	ReasonPending = "Pending"

	// ReasonResourceVerificationFailed indicates that the task fails the trusted resource verification,
	// it could be the content has changed, signature is invalid or public key is invalid
	ReasonResourceVerificationFailed = "ResourceVerificationFailed"

	// timeFormat is RFC3339 with millisecond
	timeFormat = "2006-01-02T15:04:05.000Z07:00"
)

const (
	oomKilled = "OOMKilled"
	evicted   = "Evicted"
)

// SidecarsReady returns true if all of the Pod's sidecars are Ready or
// Terminated.
func SidecarsReady(podStatus corev1.PodStatus) bool {
	if podStatus.Phase != corev1.PodRunning {
		return false
	}
	for _, s := range podStatus.ContainerStatuses {
		// If the step indicates that it's a step, skip it.
		// An injected sidecar might not have the "sidecar-" prefix, so
		// we can't just look for that prefix, we need to look at any
		// non-step container.
		if IsContainerStep(s.Name) {
			continue
		}
		if s.State.Running != nil && s.Ready {
			continue
		}
		if s.State.Terminated != nil {
			continue
		}
		return false
	}
	return true
}

// MakeTaskRunStatus returns a TaskRunStatus based on the Pod's status.
func MakeTaskRunStatus(ctx context.Context, logger *zap.SugaredLogger, tr v1.TaskRun, pod *corev1.Pod, kubeclient kubernetes.Interface, ts *v1.TaskSpec) (v1.TaskRunStatus, error) {
	trs := &tr.Status
	if trs.GetCondition(apis.ConditionSucceeded) == nil || trs.GetCondition(apis.ConditionSucceeded).Status == corev1.ConditionUnknown {
		// If the taskRunStatus doesn't exist yet, it's because we just started running
		markStatusRunning(trs, v1.TaskRunReasonRunning.String(), "Not all Steps in the Task have finished executing")
	}

	sortPodContainerStatuses(pod.Status.ContainerStatuses, pod.Spec.Containers)

	complete := areStepsComplete(pod) || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed

	if complete {
		updateCompletedTaskRunStatus(logger, trs, pod)
	} else {
		updateIncompleteTaskRunStatus(trs, pod)
	}

	trs.PodName = pod.Name
	trs.Steps = []v1.StepState{}
	trs.Sidecars = []v1.SidecarState{}

	var stepStatuses []corev1.ContainerStatus
	var sidecarStatuses []corev1.ContainerStatus
	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			stepStatuses = append(stepStatuses, s)
		} else if isContainerSidecar(s.Name) {
			sidecarStatuses = append(sidecarStatuses, s)
		}
	}

	var merr *multierror.Error
	if err := setTaskRunStatusBasedOnStepStatus(ctx, logger, stepStatuses, &tr, pod.Status.Phase, kubeclient, ts); err != nil {
		merr = multierror.Append(merr, err)
	}

	setTaskRunStatusBasedOnSidecarStatus(sidecarStatuses, trs)

	trs.Results = removeDuplicateResults(trs.Results)

	return *trs, merr.ErrorOrNil()
}

func setTaskRunStatusBasedOnStepStatus(ctx context.Context, logger *zap.SugaredLogger, stepStatuses []corev1.ContainerStatus, tr *v1.TaskRun, podPhase corev1.PodPhase, kubeclient kubernetes.Interface, ts *v1.TaskSpec) *multierror.Error {
	trs := &tr.Status
	var merr *multierror.Error

	// collect results from taskrun spec and taskspec
	specResults := []v1.TaskResult{}
	if tr.Spec.TaskSpec != nil {
		specResults = append(specResults, tr.Spec.TaskSpec.Results...)
	}
	if ts != nil {
		specResults = append(specResults, ts.Results...)
	}

	// Extract results from sidecar logs
	sidecarLogsResultsEnabled := config.FromContextOrDefaults(ctx).FeatureFlags.ResultExtractionMethod == config.ResultExtractionMethodSidecarLogs
	if sidecarLogsResultsEnabled && tr.Status.TaskSpec.Results != nil {
		// extraction of results from sidecar logs
		sidecarLogResults, err := sidecarlogresults.GetResultsFromSidecarLogs(ctx, kubeclient, tr.Namespace, tr.Status.PodName, pipeline.ReservedResultsSidecarContainerName, podPhase)
		if err != nil {
			merr = multierror.Append(merr, err)
		}

		// populate task run CRD with results from sidecar logs
		taskResults, _ := filterResults(sidecarLogResults, specResults)
		if tr.IsDone() {
			trs.Results = append(trs.Results, taskResults...)
		}
	}
	// Continue with extraction of termination messages
	for _, s := range stepStatuses {
		// Avoid changing the original value by modifying the pointer value.
		state := s.State.DeepCopy()
		if state.Terminated != nil && len(state.Terminated.Message) != 0 {
			msg := state.Terminated.Message

			results, err := termination.ParseMessage(logger, msg)
			if err != nil {
				logger.Errorf("termination message could not be parsed as JSON: %v", err)
				merr = multierror.Append(merr, err)
			} else {
				time, err := extractStartedAtTimeFromResults(results)
				if err != nil {
					logger.Errorf("error setting the start time of step %q in taskrun %q: %v", s.Name, tr.Name, err)
					merr = multierror.Append(merr, err)
				}
				exitCode, err := extractExitCodeFromResults(results)
				if err != nil {
					logger.Errorf("error extracting the exit code of step %q in taskrun %q: %v", s.Name, tr.Name, err)
					merr = multierror.Append(merr, err)
				}

				taskResults, filteredResults := filterResults(results, specResults)
				if tr.IsDone() {
					trs.Results = append(trs.Results, taskResults...)
				}
				msg, err = createMessageFromResults(filteredResults)
				if err != nil {
					logger.Errorf("%v", err)
					merr = multierror.Append(merr, err)
				} else {
					state.Terminated.Message = msg
				}
				if time != nil {
					state.Terminated.StartedAt = *time
				}
				if exitCode != nil {
					state.Terminated.ExitCode = *exitCode
				}
			}
		}
		trs.Steps = append(trs.Steps, v1.StepState{
			ContainerState: *state,
			Name:           trimStepPrefix(s.Name),
			Container:      s.Name,
			ImageID:        s.ImageID,
		})
	}

	return merr
}

func setTaskRunStatusBasedOnSidecarStatus(sidecarStatuses []corev1.ContainerStatus, trs *v1.TaskRunStatus) {
	for _, s := range sidecarStatuses {
		trs.Sidecars = append(trs.Sidecars, v1.SidecarState{
			ContainerState: *s.State.DeepCopy(),
			Name:           TrimSidecarPrefix(s.Name),
			Container:      s.Name,
			ImageID:        s.ImageID,
		})
	}
}

func createMessageFromResults(results []result.RunResult) (string, error) {
	if len(results) == 0 {
		return "", nil
	}
	bytes, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("error marshalling remaining results back into termination message: %w", err)
	}
	return string(bytes), nil
}

// filterResults filters the RunResults and TaskResults based on the results declared in the task spec.
// It returns a slice of any of the input results that are defined in the task spec, converted to TaskRunResults,
// and a slice of any of the RunResults that don't represent internal values (i.e. those that should not be displayed in the TaskRun status.
func filterResults(results []result.RunResult, specResults []v1.TaskResult) ([]v1.TaskRunResult, []result.RunResult) {
	var taskResults []v1.TaskRunResult
	var filteredResults []result.RunResult
	neededTypes := make(map[string]v1.ResultsType)
	for _, r := range specResults {
		neededTypes[r.Name] = r.Type
	}
	for _, r := range results {
		switch r.ResultType {
		case result.TaskRunResultType:
			var taskRunResult v1.TaskRunResult
			if neededTypes[r.Key] == v1.ResultsTypeString {
				taskRunResult = v1.TaskRunResult{
					Name:  r.Key,
					Type:  v1.ResultsTypeString,
					Value: *v1.NewStructuredValues(r.Value),
				}
			} else {
				v := v1.ResultValue{}
				err := v.UnmarshalJSON([]byte(r.Value))
				if err != nil {
					continue
				}
				taskRunResult = v1.TaskRunResult{
					Name:  r.Key,
					Type:  v1.ResultsType(v.Type),
					Value: v,
				}
			}
			taskResults = append(taskResults, taskRunResult)
			filteredResults = append(filteredResults, r)
		case result.InternalTektonResultType:
			// Internal messages are ignored because they're not used as external result
			continue
		default:
			filteredResults = append(filteredResults, r)
		}
	}

	return taskResults, filteredResults
}

func removeDuplicateResults(taskRunResult []v1.TaskRunResult) []v1.TaskRunResult {
	if len(taskRunResult) == 0 {
		return nil
	}

	uniq := make([]v1.TaskRunResult, 0)
	latest := make(map[string]v1.TaskRunResult, 0)
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

func extractStartedAtTimeFromResults(results []result.RunResult) (*metav1.Time, error) {
	for _, result := range results {
		if result.Key == "StartedAt" {
			t, err := time.Parse(timeFormat, result.Value)
			if err != nil {
				return nil, fmt.Errorf("could not parse time value %q in StartedAt field: %w", result.Value, err)
			}
			startedAt := metav1.NewTime(t)
			return &startedAt, nil
		}
	}
	return nil, nil //nolint:nilnil // would be more ergonomic to return a sentinel error
}

func extractExitCodeFromResults(results []result.RunResult) (*int32, error) {
	for _, result := range results {
		if result.Key == "ExitCode" {
			// We could just pass the string through but this provides extra validation
			i, err := strconv.ParseUint(result.Value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("could not parse int value %q in ExitCode field: %w", result.Value, err)
			}
			exitCode := int32(i)
			return &exitCode, nil
		}
	}
	return nil, nil //nolint:nilnil // would be more ergonomic to return a sentinel error
}

func updateCompletedTaskRunStatus(logger *zap.SugaredLogger, trs *v1.TaskRunStatus, pod *corev1.Pod) {
	if DidTaskRunFail(pod) {
		msg := getFailureMessage(logger, pod)
		markStatusFailure(trs, v1.TaskRunReasonFailed.String(), msg)
	} else {
		markStatusSuccess(trs)
	}

	// update tr completed time
	trs.CompletionTime = &metav1.Time{Time: time.Now()}
}

func updateIncompleteTaskRunStatus(trs *v1.TaskRunStatus, pod *corev1.Pod) {
	switch pod.Status.Phase {
	case corev1.PodRunning:
		markStatusRunning(trs, v1.TaskRunReasonRunning.String(), "Not all Steps in the Task have finished executing")
	case corev1.PodPending:
		switch {
		case IsPodExceedingNodeResources(pod):
			markStatusRunning(trs, ReasonExceededNodeResources, "TaskRun Pod exceeded available resources")
		case isPodHitConfigError(pod):
			markStatusFailure(trs, ReasonCreateContainerConfigError, "Failed to create pod due to config error")
		case isPullImageError(pod):
			markStatusRunning(trs, ReasonPullImageFailed, getWaitingMessage(pod))
		default:
			markStatusRunning(trs, ReasonPending, getWaitingMessage(pod))
		}
	case corev1.PodSucceeded, corev1.PodFailed, corev1.PodUnknown:
		// Do nothing; pod has completed or is in an unknown state.
	}
}

// DidTaskRunFail check the status of pod to decide if related taskrun is failed
func DidTaskRunFail(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed {
		return true
	}

	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated != nil {
				if s.State.Terminated.ExitCode != 0 || isOOMKilled(s) {
					return true
				}
			}
		}
	}
	return false
}

// IsPodArchived indicates if a pod is archived in the retriesStatus.
func IsPodArchived(pod *corev1.Pod, trs *v1.TaskRunStatus) bool {
	for _, retryStatus := range trs.RetriesStatus {
		if retryStatus.PodName == pod.GetName() {
			return true
		}
	}
	return false
}

func areStepsComplete(pod *corev1.Pod) bool {
	stepsComplete := len(pod.Status.ContainerStatuses) > 0 && pod.Status.Phase == corev1.PodRunning
	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated == nil {
				stepsComplete = false
			}
		}
	}
	return stepsComplete
}

func getFailureMessage(logger *zap.SugaredLogger, pod *corev1.Pod) string {
	// If a pod was evicted, use the pods status message before trying to
	// determine a failure message from the pod's container statuses. A
	// container may have a generic exit code that contains less information,
	// such as an exit code and message related to not being located.
	if pod.Status.Reason == evicted {
		return pod.Status.Message
	}

	// First, try to surface an error about the actual init container that failed.
	for _, status := range pod.Status.InitContainerStatuses {
		if msg := extractContainerFailureMessage(logger, status, pod.ObjectMeta); len(msg) > 0 {
			return fmt.Sprintf("init container failed, %s", msg)
		}
	}

	// Next, try to surface an error about the actual build step that failed.
	for _, status := range pod.Status.ContainerStatuses {
		if msg := extractContainerFailureMessage(logger, status, pod.ObjectMeta); len(msg) > 0 {
			return msg
		}
	}
	// Next, return the Pod's status message if it has one.
	if pod.Status.Message != "" {
		return pod.Status.Message
	}

	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated != nil {
				if isOOMKilled(s) {
					return oomKilled
				}
			}
		}
	}

	// Lastly fall back on a generic error message.
	return "build failed for unspecified reasons."
}

// extractContainerFailureMessage returns the container failure message by container status or init container status.
func extractContainerFailureMessage(logger *zap.SugaredLogger, status corev1.ContainerStatus, podMetaData metav1.ObjectMeta) string {
	term := status.State.Terminated
	if term != nil {
		msg := status.State.Terminated.Message
		r, _ := termination.ParseMessage(logger, msg)
		for _, runResult := range r {
			if runResult.ResultType == result.InternalTektonResultType && runResult.Key == "Reason" && runResult.Value == "TimeoutExceeded" {
				// Newline required at end to prevent yaml parser from breaking the log help text at 80 chars
				return fmt.Sprintf("%q exited because the step exceeded the specified timeout limit; for logs run: kubectl -n %s logs %s -c %s\n",
					status.Name,
					podMetaData.Namespace, podMetaData.Name, status.Name)
			}
		}
		if term.ExitCode != 0 {
			// Newline required at end to prevent yaml parser from breaking the log help text at 80 chars
			return fmt.Sprintf("%q exited with code %d (image: %q); for logs run: kubectl -n %s logs %s -c %s\n",
				status.Name, term.ExitCode, status.ImageID,
				podMetaData.Namespace, podMetaData.Name, status.Name)
		}
	}

	return ""
}

// IsPodExceedingNodeResources returns true if the Pod's status indicates there
// are insufficient resources to schedule the Pod.
func IsPodExceedingNodeResources(pod *corev1.Pod) bool {
	for _, podStatus := range pod.Status.Conditions {
		if podStatus.Reason == corev1.PodReasonUnschedulable && strings.Contains(podStatus.Message, "Insufficient") {
			return true
		}
	}
	return false
}

// isPodHitConfigError returns true if the Pod's status undicates there are config error raised
func isPodHitConfigError(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == ReasonCreateContainerConfigError {
			return true
		}
	}
	return false
}

// isPullImageError returns true if the Pod's status indicates there are any error when pulling image
func isPullImageError(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && isImageErrorReason(containerStatus.State.Waiting.Reason) {
			return true
		}
	}
	return false
}

func isImageErrorReason(reason string) bool {
	// Reference from https://github.com/kubernetes/kubernetes/blob/a1c8e9386af844757333733714fa1757489735b3/pkg/kubelet/images/types.go#L26
	imageErrorReasons := []string{
		"ImagePullBackOff",
		"ImageInspectError",
		"ErrImagePull",
		"ErrImageNeverPull",
		"RegistryUnavailable",
		"InvalidImageName",
	}
	for _, imageReason := range imageErrorReasons {
		if imageReason == reason {
			return true
		}
	}
	return false
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

// markStatusRunning sets taskrun status to running
func markStatusRunning(trs *v1.TaskRunStatus, reason, message string) {
	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  reason,
		Message: message,
	})
}

// markStatusFailure sets taskrun status to failure with specified reason
func markStatusFailure(trs *v1.TaskRunStatus, reason string, message string) {
	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
}

// markStatusSuccess sets taskrun status to success
func markStatusSuccess(trs *v1.TaskRunStatus) {
	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  v1.TaskRunReasonSuccessful.String(),
		Message: "All Steps have completed executing",
	})
}

// sortPodContainerStatuses reorders a pod's container statuses so that
// they're in the same order as the step containers from the TaskSpec.
func sortPodContainerStatuses(podContainerStatuses []corev1.ContainerStatus, podSpecContainers []corev1.Container) {
	statuses := map[string]corev1.ContainerStatus{}
	for _, status := range podContainerStatuses {
		statuses[status.Name] = status
	}
	for i, c := range podSpecContainers {
		// prevent out-of-bounds panic on incorrectly formed lists
		if i < len(podContainerStatuses) {
			podContainerStatuses[i] = statuses[c.Name]
		}
	}
}

func isOOMKilled(s corev1.ContainerStatus) bool {
	return s.State.Terminated.Reason == oomKilled
}
