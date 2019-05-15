package builder

import (
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineCreationTimestamp sets the creation time of the pipeline
func PipelineCreationTimestamp(t time.Time) tb.PipelineOp {
	return func(p *v1alpha1.Pipeline) {
		p.CreationTimestamp = metav1.Time{Time: t}
	}
}
