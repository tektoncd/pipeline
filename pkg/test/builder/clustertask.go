package builder

import (
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TaskCreationTime sets CreationTimestamp of the task
func ClusterTaskCreationTime(t time.Time) tb.ClusterTaskOp {
	return func(clustertask *v1alpha1.ClusterTask) {
		clustertask.CreationTimestamp = metav1.Time{Time: t}
	}
}
