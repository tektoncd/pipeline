package formatted

import (
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
)

// Condition returns a human readable text based on the Status of the Condition
func Condition(c apis.Condition) string {
	var status string
	switch c.Status {
	case corev1.ConditionFalse:
		status = "Failed"
	case corev1.ConditionTrue:
		status = "Succeeded"
	case corev1.ConditionUnknown:
		status = "Running"
	}
	if c.Reason != "" && c.Reason != status {
		status = status + "(" + c.Reason + ")"
	}

	return status
}
