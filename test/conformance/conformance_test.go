//go:build conformance
// +build conformance

/*
Copyright 2024 The Tekton Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
The following tests are the OSS conformance test suite.
For a more detailed reference of the Tekton Conformance API spec please refer
to https://github.com/tektoncd/pipeline/blob/main/docs/api-spec.md.

Please implement the `ProcessAndSendToTekton` with the respective conformant
Tekton service corresponding to the function signature below:

// ProcessAndSendToTekton takes in vanilla Tekton PipelineRun and TaskRun,
// waits for the object to succeed and outputs the final PipelineRun and
// TaskRun with status.
// The parameters are inputYAML and its Primitive type {PipelineRun, TaskRun}
// And the return values will be the output YAML string and errors.
func ProcessAndSendToTekton(inputYAML, primitiveType string, customInputs ...interface{}) (string, error) {
}

Once `ProcessAndSendToTekton` is implemented, please use the following for
triggering the test and record the corresponding outputs:
go test -v -tags=conformance -count=1 ./test -run ^Test
*/

package conformance_test

import (
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/test/parse"
	"knative.dev/pkg/test/helpers"
)

const (
	succeedConditionStatus = "True"
	conformanceVersion     = "v1"
)

// TestTaskRunConditions examines population of Conditions
// fields. It creates the a TaskRun with minimal specifications and checks the
// required Condition Status and Type.
func TestTaskRunConditions(t *testing.T) {
	t.Parallel()

	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    steps:
    - name: add
      image: ubuntu
      script: |
        echo Hello world!
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if err := checkTaskRunConditionSucceeded(resolvedTR.Status, succeedConditionStatus, "Succeeded"); err != nil {
		t.Error(err)
	}
}

// TestPipelineRunConditions examines population of Conditions
// fields. It creates the a PipelineRun with minimal specifications and checks the
// required Condition Status and Type.
func TestPipelineRunConditions(t *testing.T) {
	t.Parallel()

	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: %s
spec:
  pipelineSpec:
    tasks:
    - name: pipeline-task-0
      taskSpec:
        steps:
        - name: add
          image: ubuntu
          script: |
            echo Hello world!
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, PipelineRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedPR := parse.MustParseV1PipelineRun(t, outputYAML)

	if err := checkPipelineRunConditionSucceeded(resolvedPR.Status, succeedConditionStatus, "Succeeded"); err != nil {
		t.Error(err)
	}
}
