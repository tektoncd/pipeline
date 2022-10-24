package artifacts

import "fmt"

// GetPVCName returns the name that should be used for the PVC for a PipelineRun
func GetPVCName(n named) string {
	return fmt.Sprintf("%s-pvc", n.GetName())
}

type named interface {
	GetName() string
}
