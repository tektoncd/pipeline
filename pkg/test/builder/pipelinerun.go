package builder

import (
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineRunCreationTimestamp sets the creation time of the pipeline
func PipelineRunCreationTimestamp(t time.Time) tb.PipelineRunOp {
	return func(p *v1alpha1.PipelineRun) {
		p.CreationTimestamp = metav1.Time{Time: t}
	}
}

// PipelineRunCompletionTime sets the completion time  to the PipelineRunStatus.
func PipelineRunCompletionTime(t time.Time) tb.PipelineRunStatusOp {
	return func(s *v1alpha1.PipelineRunStatus) {
		s.CompletionTime = &metav1.Time{Time: t}
	}
}
