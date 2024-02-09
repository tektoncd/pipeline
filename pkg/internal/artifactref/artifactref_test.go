package artifactref

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	fmt.Println(StepArtifactOutputRegex.Match([]byte("$(steps.fsdaa.outputs.ssss)")))
	fmt.Println(StepArtifactOutputRegex.Match([]byte("$(steps.fsdaa.outputs)")))
	fmt.Println(StepArtifactOutputRegex.Match([]byte("$(steps.fsdaa.outputs.sss.ssss)")))
}
