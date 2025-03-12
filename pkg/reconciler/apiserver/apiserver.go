package apiserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	pipelineErrors "github.com/tektoncd/pipeline/pkg/apis/pipeline/errors"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	ErrReferencedObjectValidationFailed = errors.New("validation failed for referenced object")
	ErrCouldntValidateObjectRetryable   = errors.New("retryable error validating referenced object")
	ErrCouldntValidateObjectPermanent   = errors.New("permanent error validating referenced object")
)

// DryRunValidate validates the obj by issuing a dry-run create request for it in the given namespace.
// This allows validating admission webhooks to process the object without actually creating it.
// obj must be a v1/v1beta1 Task or Pipeline.
func DryRunValidate(ctx context.Context, namespace string, obj runtime.Object, tekton clientset.Interface) (runtime.Object, error) {
	dryRunObjName := uuid.NewString() // Use a randomized name for the Pipeline/Task in case there is already another Pipeline/Task of the same name

	switch obj := obj.(type) {
	case *v1.Pipeline:
		dryRunObj := obj.DeepCopy()
		dryRunObj.Name = dryRunObjName
		dryRunObj.Namespace = namespace // Make sure the namespace is the same as the PipelineRun
		mutatedObj, err := tekton.TektonV1().Pipelines(namespace).Create(ctx, dryRunObj, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil {
			return nil, handleDryRunCreateErr(err, obj.Name)
		}
		return mutatedObj, nil
	case *v1beta1.Pipeline:
		dryRunObj := obj.DeepCopy()
		dryRunObj.Name = dryRunObjName
		dryRunObj.Namespace = namespace // Make sure the namespace is the same as the PipelineRun
		mutatedObj, err := tekton.TektonV1beta1().Pipelines(namespace).Create(ctx, dryRunObj, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil {
			return nil, handleDryRunCreateErr(err, obj.Name)
		}
		return mutatedObj, nil
	case *v1.Task:
		dryRunObj := obj.DeepCopy()
		dryRunObj.Name = dryRunObjName
		dryRunObj.Namespace = namespace // Make sure the namespace is the same as the TaskRun
		mutatedObj, err := tekton.TektonV1().Tasks(namespace).Create(ctx, dryRunObj, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil {
			return nil, handleDryRunCreateErr(err, obj.Name)
		}
		return mutatedObj, nil
	case *v1beta1.Task:
		dryRunObj := obj.DeepCopy()
		dryRunObj.Name = dryRunObjName
		dryRunObj.Namespace = namespace // Make sure the namespace is the same as the TaskRun
		mutatedObj, err := tekton.TektonV1beta1().Tasks(namespace).Create(ctx, dryRunObj, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil {
			return nil, handleDryRunCreateErr(err, obj.Name)
		}
		return mutatedObj, nil
	case *v1alpha1.StepAction:
		dryRunObj := obj.DeepCopy()
		dryRunObj.Name = dryRunObjName
		dryRunObj.Namespace = namespace // Make sure the namespace is the same as the StepAction
		mutatedObj, err := tekton.TektonV1alpha1().StepActions(namespace).Create(ctx, dryRunObj, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil {
			return nil, handleDryRunCreateErr(err, obj.Name)
		}
		return mutatedObj, nil

	case *v1beta1.StepAction:
		dryRunObj := obj.DeepCopy()
		dryRunObj.Name = dryRunObjName
		dryRunObj.Namespace = namespace // Make sure the namespace is the same as the StepAction
		mutatedObj, err := tekton.TektonV1beta1().StepActions(namespace).Create(ctx, dryRunObj, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil {
			return nil, handleDryRunCreateErr(err, obj.Name)
		}
		return mutatedObj, nil

	default:
		return nil, fmt.Errorf("unsupported object GVK %s", obj.GetObjectKind().GroupVersionKind())
	}
}

func handleDryRunCreateErr(err error, objectName string) error {
	var errType error
	switch {
	case apierrors.IsBadRequest(err): // Object rejected by validating webhook
		errType = ErrReferencedObjectValidationFailed
	case apierrors.IsInvalid(err), apierrors.IsMethodNotSupported(err):
		errType = pipelineErrors.WrapUserError(ErrCouldntValidateObjectPermanent)
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err), apierrors.IsTooManyRequests(err):
		errType = ErrCouldntValidateObjectRetryable
	default:
		// Assume unknown errors are retryable
		// Additional errors can be added to the switch statements as needed
		errType = ErrCouldntValidateObjectRetryable
	}
	return fmt.Errorf("%w %s: %s", errType, objectName, err.Error())
}
