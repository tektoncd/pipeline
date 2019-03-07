package v1alpha1

import (
	"testing"

	"github.com/knative/pkg/webhook"
)

func TestTypes(t *testing.T) {
	// Assert that types satisfy webhook interface.
	var _ webhook.GenericCRD = (*ClusterTask)(nil)
	var _ webhook.GenericCRD = (*TaskRun)(nil)
	var _ webhook.GenericCRD = (*PipelineResource)(nil)
	var _ webhook.GenericCRD = (*Task)(nil)
	var _ webhook.GenericCRD = (*TaskRun)(nil)
}
