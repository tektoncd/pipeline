package builder

import (
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TaskCreationTime sets CreationTimestamp of the task
func TaskCreationTime(t time.Time) tb.TaskOp {
	return func(task *v1alpha1.Task) {
		task.CreationTimestamp = metav1.Time{Time: t}
	}
}
