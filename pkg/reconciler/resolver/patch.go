package resolver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type metadataPatch struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type taskRunStatusPatch struct {
	TaskSpec v1beta1.TaskSpec `json:"taskSpec"`
}

type pipelineRunStatusPatch struct {
	PipelineSpec v1beta1.PipelineSpec `json:"pipelineSpec"`
}

// PatchResolvedTaskRun accepts a TaskRun with its task spec resolved and updates the
// stored resource with the labels, annotations and resolved spec.
func PatchResolvedTaskRun(ctx context.Context, kClient kubernetes.Interface, pClient clientset.Interface, tr *v1beta1.TaskRun) (*v1beta1.TaskRun, error) {
	metadataBytes, err := json.Marshal(map[string]metadataPatch{
		"metadata": {
			Labels:      tr.ObjectMeta.Labels,
			Annotations: tr.ObjectMeta.Annotations,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error constructing metadata patch: %w", err)
	}

	statusBytes, err := json.Marshal(map[string]taskRunStatusPatch{
		"status": {
			TaskSpec: *tr.Status.TaskSpec,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error constructing status patch: %w", err)
	}

	tr, err = pClient.TektonV1beta1().TaskRuns(tr.Namespace).Patch(ctx, tr.Name, types.MergePatchType, metadataBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("error patching metadata: %w", err)
	}

	tr, err = pClient.TektonV1beta1().TaskRuns(tr.Namespace).Patch(ctx, tr.Name, types.MergePatchType, statusBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return nil, fmt.Errorf("error patching status: %w", err)
	}

	return tr, nil
}

// PatchResolvedPipelineRun accepts a PipelineRun with its pipeline spec resolved and updates
// the stored resource with the labels, annotations and resolved spec.
func PatchResolvedPipelineRun(ctx context.Context, kClient kubernetes.Interface, pClient clientset.Interface, pr *v1beta1.PipelineRun) (*v1beta1.PipelineRun, error) {
	metadataBytes, err := json.Marshal(map[string]metadataPatch{
		"metadata": {
			Labels:      pr.ObjectMeta.Labels,
			Annotations: pr.ObjectMeta.Annotations,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error constructing metadata patch: %w", err)
	}

	statusBytes, err := json.Marshal(map[string]pipelineRunStatusPatch{
		"status": {
			PipelineSpec: *pr.Status.PipelineSpec,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error constructing status patch: %w", err)
	}

	pr, err = pClient.TektonV1beta1().PipelineRuns(pr.Namespace).Patch(ctx, pr.Name, types.MergePatchType, metadataBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("error patching metadata: %w", err)
	}

	pr, err = pClient.TektonV1beta1().PipelineRuns(pr.Namespace).Patch(ctx, pr.Name, types.MergePatchType, statusBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return nil, fmt.Errorf("error patching status: %w", err)
	}

	return pr, nil
}
