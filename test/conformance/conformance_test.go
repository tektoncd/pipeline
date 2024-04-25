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
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/test/parse"
	"knative.dev/pkg/test/helpers"
)

const (
	succeedConditionStatus = "True"
	conformanceVersion     = "v1"
	failureConditionStatus = "False"
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

const (
	TaskRunInputType     = "TaskRun"
	PipelineRunInputType = "PipelineRun"
	ExpectRunToFail      = true
)

func TestStepScript(t *testing.T) {
	t.Parallel()
	expectedSteps := map[string]string{
		"node":                      "Completed",
		"perl":                      "Completed",
		"params-applied":            "Completed",
		"args-allowed":              "Completed",
		"dollar-signs-allowed":      "Completed",
		"bash-variable-evaluations": "Completed",
	}

	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    params:
    - name: PARAM
      default: param-value
    steps:
    - name: node
      image: node
      script: |
        #!/usr/bin/env node
        console.log("Hello from Node!")
    - name: perl
      image: perl:devel-bullseye
      script: |
        #!/usr/bin/perl
        print "Hello from Perl!"
    # Test that param values are replaced.
    - name: params-applied
      image: python
      script: |
        #!/usr/bin/env python3
        v = '$(params.PARAM)'
        if v != 'param-value':
          print('Param values not applied')
          print('Got: ', v)
          exit(1)
    # Test that args are allowed and passed to the script as expected.
    - name: args-allowed
      image: ubuntu
      args: ['hello', 'world']
      script: |
        #!/usr/bin/env bash
        [[ $# == 2 ]]
        [[ $1 == "hello" ]]
        [[ $2 == "world" ]]
    # Test that multiple dollar signs next to each other are not replaced by Kubernetes
    - name: dollar-signs-allowed
      image: python
      script: |
        #!/usr/bin/env python3
        if '$' != '\u0024':
          print('single dollar signs ($) are not passed through as expected :(')
          exit(1)
        if '$$' != '\u0024\u0024':
          print('double dollar signs ($$) are not passed through as expected :(')
          exit(2)
        if '$$$' != '\u0024\u0024\u0024':
          print('three dollar signs ($$$) are not passed through as expected :(')
          exit(3)
        if '$$$$' != '\u0024\u0024\u0024\u0024':
          print('four dollar signs ($$$$) are not passed through as expected :(')
          exit(4)
        print('dollar signs appear to be handled correctly! :)')

    # Test that bash scripts with variable evaluations work as expected
    - name: bash-variable-evaluations
      image: bash:5.1.8
      script: |
        #!/usr/bin/env bash
        set -xe
        var1=var1_value
        var2=var1
        echo $(eval echo \$$var2) > tmpfile
        eval_result=$(cat tmpfile)
        if [ "$eval_result" != "var1_value" ] ; then
          echo "unexpected eval result: $eval_result"
          exit 1
        fi
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if len(resolvedTR.Status.Steps) != len(expectedSteps) {
		t.Errorf("Expected length of steps %v but has: %v", len(expectedSteps), len(resolvedTR.Status.Steps))
	}

	for _, resolvedStep := range resolvedTR.Status.Steps {
		resolvedStepTerminatedReason := resolvedStep.Terminated.Reason
		if expectedStepState, ok := expectedSteps[resolvedStep.Name]; ok {
			if resolvedStepTerminatedReason != expectedStepState {
				t.Fatalf("Expect step %s to have completed successfully but it has Termination Reason: %s", resolvedStep.Name, resolvedStepTerminatedReason)
			}
		} else {
			t.Fatalf("Does not expect to have step: %s", resolvedStep.Name)
		}
	}
}

func TestStepEnv(t *testing.T) {
	t.Parallel()
	envVarName := "FOO"
	envVarVal := "foooooooo"

	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    steps:
    - name: bash
      image: ubuntu
      env:
      - name: %s
        value: %s
      script: |
        #!/usr/bin/env bash
        set -euxo pipefail
        echo "Hello from Bash!"
        echo FOO is ${FOO}
        echo substring is ${FOO:2:4}
`, helpers.ObjectNameForTest(t), envVarName, envVarVal)

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	resolvedStep := resolvedTR.Status.Steps[0]
	resolvedStepTerminatedReason := resolvedStep.Terminated.Reason
	if resolvedStepTerminatedReason != "Completed" {
		t.Fatalf("Expect step %s to have completed successfully but it has Termination Reason: %s", resolvedStep.Name, resolvedStepTerminatedReason)
	}

	resolvedStepEnv := resolvedTR.Status.TaskSpec.Steps[0].Env[0]
	if resolvedStepEnv.Name != envVarName {
		t.Fatalf("Expect step %s to have EnvVar Name %s but it has: %s", resolvedStep.Name, envVarName, resolvedStepEnv.Name)
	}
	if resolvedStepEnv.Value != envVarVal {
		t.Fatalf("Expect step %s to have EnvVar Value %s but it has: %s", resolvedStep.Name, envVarVal, resolvedStepEnv.Value)
	}
}

func TestStepWorkingDir(t *testing.T) {
	t.Parallel()
	defaultWorkingDir := "/workspace"
	overrideWorkingDir := "/a/path/too/far"

	expectedWorkingDirs := map[string]string{
		"default":  defaultWorkingDir,
		"override": overrideWorkingDir,
	}

	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    steps:
    - name: default
      image: ubuntu
      workingDir: %s
      script: |
        #!/usr/bin/env bash
        if [[ $PWD != /workspace ]]; then
          exit 1
        fi
    - name: override
      image: ubuntu
      workingDir: %s
      script: |
        #!/usr/bin/env bash
        if [[ $PWD != /a/path/too/far ]]; then
          exit 1
        fi
`, helpers.ObjectNameForTest(t), defaultWorkingDir, overrideWorkingDir)

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	for _, resolvedStep := range resolvedTR.Status.Steps {
		resolvedStepTerminatedReason := resolvedStep.Terminated.Reason
		if resolvedStepTerminatedReason != "Completed" {
			t.Fatalf("Expect step %s to have completed successfully but it has Termination Reason: %s", resolvedStep.Name, resolvedStepTerminatedReason)
		}
	}

	for _, resolvedStepSpec := range resolvedTR.Status.TaskSpec.Steps {
		resolvedStepWorkingDir := resolvedStepSpec.WorkingDir
		if resolvedStepWorkingDir != expectedWorkingDirs[resolvedStepSpec.Name] {
			t.Fatalf("Expect step %s to have WorkingDir %s but it has: %s", resolvedStepSpec.Name, expectedWorkingDirs[resolvedStepSpec.Name], resolvedStepWorkingDir)
		}
	}
}

func TestStepStateImageID(t *testing.T) {
	t.Parallel()
	// Step images can be specified by digest.
	image := "busybox@sha256:1303dbf110c57f3edf68d9f5a16c082ec06c4cf7604831669faf2c712260b5a0"
	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    steps:
    - image: %s
      args: ['-c', 'echo hello']
`, helpers.ObjectNameForTest(t), image)

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

	if len(resolvedTR.Status.Steps) != 1 {
		t.Errorf("Expect vendor service to provide 1 Step in StepState but it has: %v", len(resolvedTR.Status.Steps))
	}

	if !strings.HasSuffix(resolvedTR.Status.Steps[0].ImageID, image) {
		t.Errorf("Expect vendor service to provide image %s in StepState but it has: %s", image, resolvedTR.Status.Steps[0].ImageID)
	}
}

func TestStepStateName(t *testing.T) {
	t.Parallel()
	stepName := "step-foo"
	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    steps:
    - name: %s
      image: busybox
      args: ['-c', 'echo hello']
`, helpers.ObjectNameForTest(t), stepName)

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

	if len(resolvedTR.Status.Steps) != 1 {
		t.Errorf("Expect vendor service to provide 1 Step in StepState but it has: %v", len(resolvedTR.Status.Steps))
	}

	if resolvedTR.Status.Steps[0].Name != stepName {
		t.Errorf("Expect vendor service to provide Name %s in StepState but it has: %s", stepName, resolvedTR.Status.Steps[0].Name)
	}
}

// Examines the ContainerStateTerminated ExitCode, StartedAt, FinishtedAt and Reason
func TestStepStateContainerStateTerminated(t *testing.T) {
	t.Parallel()
	successInputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    steps:
    - image: busybox
      args: ['-c', 'echo hello']
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	successOutputYAML, err := ProcessAndSendToTekton(successInputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	successResolvedTR := parse.MustParseV1TaskRun(t, successOutputYAML)

	if err := checkTaskRunConditionSucceeded(successResolvedTR.Status, succeedConditionStatus, "Succeeded"); err != nil {
		t.Error(err)
	}

	if len(successResolvedTR.Status.Steps) != 1 {
		t.Errorf("Expect vendor service to provide 1 Step in StepState but it has: %v", len(successResolvedTR.Status.Steps))
	}

	startTime := successResolvedTR.Status.Steps[0].Terminated.StartedAt
	finishTime := successResolvedTR.Status.Steps[0].Terminated.FinishedAt

	if startTime.IsZero() {
		t.Errorf("Expect vendor service to provide StartTimeStamp in StepState.Terminated but it does not provide so")
	}

	if finishTime.IsZero() {
		t.Errorf("Expect vendor service to provide FinishTimeStamp in StepState.Terminated but it does not provide so")
	}

	if finishTime.Before(&startTime) {
		t.Errorf("Expect vendor service to provide StartTimeStamp %v earlier than FinishTimeStamp in StepState.Terminated %v but it does not provide so", startTime, finishTime)
	}

	if successResolvedTR.Status.Steps[0].Terminated.ExitCode != 0 {
		t.Errorf("Expect vendor service to provide ExitCode in StepState.Terminated to be 0 but it has: %v", successResolvedTR.Status.Steps[0].Terminated.ExitCode)
	}

	if successResolvedTR.Status.Steps[0].Terminated.Reason != "Completed" {
		t.Errorf("Expect vendor service to provide Reason in StepState.Terminated to be Completed but it has: %s", successResolvedTR.Status.Steps[0].Terminated.Reason)
	}

	failureInputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    steps:
    - image: busybox
      script: exit 1
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	failureOutputYAML, err := ProcessAndSendToTekton(failureInputYAML, TaskRunInputType, t, ExpectRunToFail)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	failureResolvedTR := parse.MustParseV1TaskRun(t, failureOutputYAML)

	if err := checkTaskRunConditionSucceeded(failureResolvedTR.Status, failureConditionStatus, "Failed"); err != nil {
		t.Error(err)
	}

	if len(failureResolvedTR.Status.Steps) != 1 {
		t.Errorf("Expect vendor service to provide 1 Step in StepState but it has: %v", len(failureResolvedTR.Status.Steps))
	}

	startTime = failureResolvedTR.Status.Steps[0].Terminated.StartedAt
	finishTime = failureResolvedTR.Status.Steps[0].Terminated.FinishedAt

	if startTime.IsZero() {
		t.Errorf("Expect vendor service to provide StartTimeStamp in StepState.Terminated but it does not provide so")
	}

	if finishTime.IsZero() {
		t.Errorf("Expect vendor service to provide FinishTimeStamp in StepState.Terminated but it does not provide so")
	}

	if finishTime.Before(&startTime) {
		t.Errorf("Expect vendor service to provide StartTimeStamp %v earlier than FinishTimeStamp in StepState.Terminated %v but it does not provide so", startTime, finishTime)
	}

	if failureResolvedTR.Status.Steps[0].Terminated.ExitCode != 1 {
		t.Errorf("Expect vendor service to provide ExitCode in StepState.Terminated to be 0 but it has: %v", failureResolvedTR.Status.Steps[0].Terminated.ExitCode)
	}

	if failureResolvedTR.Status.Steps[0].Terminated.Reason != "Error" {
		t.Errorf("Expect vendor service to provide Reason in StepState.Terminated to be Error but it has: %s", failureResolvedTR.Status.Steps[0].Terminated.Reason)
	}
}

func TestSidecarName(t *testing.T) {
	sidecarName := "hello-sidecar"
	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    sidecars:
    - name: %s
      image: ubuntu
      script: echo "hello from sidecar"
    steps:
    - name: hello-step
      image: ubuntu
      script: echo "hello from step"
`, helpers.ObjectNameForTest(t), sidecarName)

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if err := checkTaskRunConditionSucceeded(resolvedTR.Status, SucceedConditionStatus, "Succeeded"); err != nil {
		t.Error(err)
	}

	if len(resolvedTR.Spec.TaskSpec.Sidecars) != 1 {
		t.Errorf("Expect vendor service to provide 1 Sidcar but it has: %v", len(resolvedTR.Spec.TaskSpec.Sidecars))
	}

	if resolvedTR.Spec.TaskSpec.Sidecars[0].Name != sidecarName {
		t.Errorf("Expect vendor service to provide Sidcar name %s but it has: %s", sidecarName, resolvedTR.Spec.TaskSpec.Sidecars[0].Name)
	}
}

// This test relies on the support of Sidecar Script and its volumeMounts.
// For sidecar tests, sidecars don't have /workspace mounted by default, so we have to define
// our own shared volume. For vendor services, please feel free to override the shared workspace
// supported in your sidecar. Otherwise there are no existing v1 conformance `REQUIRED` fields that
// are going to be used for verifying Sidecar functionality.
func TestSidecarScriptSuccess(t *testing.T) {
	succeedInputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    sidecars:
    - name: slow-sidecar
      image: ubuntu
      script: |
        echo "hello from sidecar" > /shared/message
      volumeMounts:
      - name: shared
        mountPath: /shared

    steps:
    - name: check-ready
      image: ubuntu
      script: cat /shared/message
      volumeMounts:
      - name: shared
        mountPath: /shared

    # Sidecars don't have /workspace mounted by default, so we have to define
    # our own shared volume.
    volumes:
    - name: shared
      emptyDir: {}
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	succeedOutputYAML, err := ProcessAndSendToTekton(succeedInputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	succeededResolvedTR := parse.MustParseV1TaskRun(t, succeedOutputYAML)

	if err := checkTaskRunConditionSucceeded(succeededResolvedTR.Status, SucceedConditionStatus, "Succeeded"); err != nil {
		t.Error(err)
	}
}

func TestSidecarScriptFailure(t *testing.T) {
	failInputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    sidecars:
    - name: exit-sidecar
      image: ubuntu
      script: exit 1

    steps:
    - name: check-ready
      image: ubuntu
      script: cat /shared/message
      volumeMounts:
        - name: shared
          mountPath: /shared

    # Sidecars don't have /workspace mounted by default, so we have to define
    # our own shared volume.
    volumes:
    - name: shared
      emptyDir: {}
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	failOutputYAML, err := ProcessAndSendToTekton(failInputYAML, TaskRunInputType, t, ExpectRunToFail)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	failResolvedTR := parse.MustParseV1TaskRun(t, failOutputYAML)

	if len(failResolvedTR.Spec.TaskSpec.Sidecars) != 1 {
		t.Errorf("Expect vendor service to provide 1 Sidcar but it has: %v", len(failResolvedTR.Spec.TaskSpec.Sidecars))
	}

	if err := checkTaskRunConditionSucceeded(failResolvedTR.Status, "False", "Failed"); err != nil {
		t.Error(err)
	}
}

func TestSidecarArgAndCommand(t *testing.T) {
	failInputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  taskSpec:
    sidecars:
    - name: slow-sidecar
      image: ubuntu
      command: [/bin/bash]
      args: [-c, "echo 'hello from sidecar' > /shared/message"]
      volumeMounts:
      - name: shared
        mountPath: /shared
    steps:
    - name: check-ready
      image: ubuntu
      command:
      - cat
      args:
      - '/shared/message'
      volumeMounts:
      - name: shared
        mountPath: /shared
    
    # Sidecars don't have /workspace mounted by default, so we have to define
    # our own shared volume.
    volumes:
    - name: shared
      emptyDir: {}
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	failOutputYAML, err := ProcessAndSendToTekton(failInputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	failResolvedTR := parse.MustParseV1TaskRun(t, failOutputYAML)

	if len(failResolvedTR.Spec.TaskSpec.Sidecars) != 1 {
		t.Errorf("Expect vendor service to provide 1 Sidcar but it has: %v", len(failResolvedTR.Spec.TaskSpec.Sidecars))
	}

	if err := checkTaskRunConditionSucceeded(failResolvedTR.Status, SucceedConditionStatus, "Succeeded"); err != nil {
		t.Error(err)
	}
}

func TestStringTaskParam(t *testing.T) {
	stringParam := "foo-string"

	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  params:
    - name: "string-param"
      value: %s
  taskSpec:
    params:
      - name: "string-param"
        type: string
    steps:
      - name: "check-param"
        image: bash
        script: |
          if [[ $(params.string-param) != %s ]]; then
            exit 1
          fi
`, helpers.ObjectNameForTest(t), stringParam, stringParam)

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if len(resolvedTR.Spec.Params) != 1 {
		t.Errorf("Expect vendor service to provide 1 Param but it has: %v", len(resolvedTR.Spec.Params))
	}

	if err := checkTaskRunConditionSucceeded(resolvedTR.Status, SucceedConditionStatus, "Succeeded"); err != nil {
		t.Error(err)
	}

}

func TestArrayTaskParam(t *testing.T) {
	var arrayParam0, arrayParam1 = "foo", "bar"

	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  params:
    - name: array-to-concat
      value:
        - %s
        - %s
  taskSpec:
    results:
    - name: "concat-array"
    params:
      - name: array-to-concat
        type: array
    steps:
    - name: concat-array-params
      image: alpine
      command: ["/bin/sh", "-c"]
      args:
      - echo -n $(params.array-to-concat[0])"-"$(params.array-to-concat[1]) | tee $(results.concat-array.path);
`, helpers.ObjectNameForTest(t), arrayParam0, arrayParam1)

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if len(resolvedTR.Spec.Params) != 1 {
		t.Errorf("Examining TaskRun Param: expect vendor service to provide TaskRun with 1 Array Param but it has: %v", len(resolvedTR.Spec.Params))
	}
	if len(resolvedTR.Spec.Params[0].Value.ArrayVal) != 2 {
		t.Errorf("Examining TaskParams: expect vendor service to provide 2 Task Array Param values but it has: %v", len(resolvedTR.Spec.Params[0].Value.ArrayVal))
	}

	// Utilizing TaskResult to verify functionality of Array Params
	if len(resolvedTR.Status.Results) != 1 {
		t.Errorf("Expect vendor service to provide 1 result but it has: %v", len(resolvedTR.Status.Results))
	}
	if resolvedTR.Status.Results[0].Value.StringVal != arrayParam0+"-"+arrayParam1 {
		t.Errorf("Not producing correct result, expect to get \"%s\" but has: \"%s\"", arrayParam0+"-"+arrayParam1, resolvedTR.Status.Results[0].Value.StringVal)
	}
}

func TestTaskParamDefaults(t *testing.T) {
	stringParam := "string-foo"
	arrayParam := []string{"array-foo", "array-bar"}
	expectedStringParamResultVal := "string-foo-string-baz-default"
	expectedArrayParamResultVal := "array-foo-array-bar-default"

	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  params:
    - name: array-param
      value:
        - %s
        - %s
    - name: string-param
      value: %s
  taskSpec:
    results:
    - name: array-output
    - name: string-output
    params:
      - name: array-param
        type: array
      - name: array-defaul-param
        type: array
        default:
        - "array-foo-default"
        - "array-bar-default"
      - name: string-param
        type: string
      - name: string-default
        type: string
        default: "string-baz-default"
    steps:
      - name: string-params-to-result
        image: bash:3.2
        command: ["/bin/sh", "-c"]
        args:
        - echo -n $(params.string-param)"-"$(params.string-default) | tee $(results.string-output.path);
      - name: array-params-to-result
        image: bash:3.2
        command: ["/bin/sh", "-c"]
        args:
        - echo -n $(params.array-param[0])"-"$(params.array-defaul-param[1]) | tee $(results.array-output.path);
`, helpers.ObjectNameForTest(t), arrayParam[0], arrayParam[1], stringParam)

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if len(resolvedTR.Spec.Params) != 2 {
		t.Errorf("Expect vendor service to provide 2 Params but it has: %v", len(resolvedTR.Spec.Params))
	}
	if len(resolvedTR.Spec.Params[0].Value.ArrayVal) != 2 {
		t.Errorf("Expect vendor service to provide 2 Task Array Params but it has: %v", len(resolvedTR.Spec.Params))
	}
	for _, param := range resolvedTR.Spec.Params {
		if param.Name == "array-param" {
			paramArr := param.Value.ArrayVal
			for i, _ := range paramArr {
				if paramArr[i] != arrayParam[i] {
					t.Errorf("Expect Params to match %s: %v", arrayParam[i], paramArr[i])
				}
			}
		}
		if param.Name == "string-param" {
			if param.Value.StringVal != stringParam {
				t.Errorf("Not producing correct result, expect to get \"%s\" but has: \"%s\"", stringParam, param.Value.StringVal)
			}
		}
	}

	// Utilizing TaskResult to verify functionality of Task Params Defaults
	if len(resolvedTR.Status.Results) != 2 {
		t.Errorf("Expect vendor service to provide 2 result but it has: %v", len(resolvedTR.Status.Results))
	}

	for _, result := range resolvedTR.Status.Results {
		if result.Name == "string-output" {
			resultVal := result.Value.StringVal
			if resultVal != expectedStringParamResultVal {
				t.Errorf("Not producing correct result, expect to get \"%s\" but has: \"%s\"", expectedStringParamResultVal, resultVal)
			}
		}
		if result.Name == "array-output" {
			resultVal := result.Value.StringVal
			if resultVal != expectedArrayParamResultVal {
				t.Errorf("Not producing correct result, expect to get \"%s\" but has: \"%s\"", expectedArrayParamResultVal, resultVal)
			}
		}
	}
}

func TestTaskParamDescription(t *testing.T) {
	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
    name: %s
spec:
  taskSpec:
    params:
    - name: foo
      description: foo param
      default: "foo"
    steps:
    - name: add
      image: alpine
      env:
      - name: OP1
        value: $(params.foo)
      command: ["/bin/sh", "-c"]
      args:
        - echo -n ${OP1}
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if resolvedTR.Spec.TaskSpec.Params[0].Description != "foo param" {
		t.Errorf("Expect vendor service to provide Param Description \"foo param\" but it has: %s", resolvedTR.Spec.TaskSpec.Params[0].Description)
	}

	if resolvedTR.Status.TaskSpec.Params[0].Description != "foo param" {
		t.Errorf("Expect vendor service to provide Param Description \"foo param\" but it has: %s", resolvedTR.Spec.TaskSpec.Params[0].Description)
	}
}

// The goal of the Taskrun Workspace test is to verify if different Steps in the TaskRun could
// pass data among each other.
func TestTaskRunWorkspace(t *testing.T) {
	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  workspaces:
    - name: custom-workspace
      # Please note that vendor services are welcomed to override the following actual workspace binding type.
      # This is considered as the implementation detail for the conformant workspace fields.
      emptyDir: {}
  taskSpec:
    steps:
    - name: write
      image: ubuntu
      script: echo $(workspaces.custom-workspace.path) > $(workspaces.custom-workspace.path)/foo
    - name: read
      image: ubuntu
      script: cat $(workspaces.custom-workspace.path)/foo
    - name: check
      image: ubuntu
      script: |
        if [ "$(cat $(workspaces.custom-workspace.path)/foo)" != "/workspace/custom-workspace" ]; then
          echo $(cat $(workspaces.custom-workspace.path)/foo)
          exit 1
        fi
    workspaces:
    - name: custom-workspace
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if err := checkTaskRunConditionSucceeded(resolvedTR.Status, SucceedConditionStatus, "Succeeded"); err != nil {
		t.Error(err)
	}

	if len(resolvedTR.Spec.Workspaces) != 1 {
		t.Errorf("Expect vendor service to provide 1 Workspace but it has: %v", len(resolvedTR.Spec.Workspaces))
	}

	if resolvedTR.Spec.Workspaces[0].Name != "custom-workspace" {
		t.Errorf("Expect vendor service to provide Workspace 'custom-workspace' but it has: %s", resolvedTR.Spec.Workspaces[0].Name)
	}

	if resolvedTR.Status.TaskSpec.Workspaces[0].Name != "custom-workspace" {
		t.Errorf("Expect vendor service to provide Workspace 'custom-workspace' in TaskRun.Status.TaskSpec but it has: %s", resolvedTR.Spec.Workspaces[0].Name)
	}
}

// TestTaskRunTimeout examines the Timeout behaviour for
// TaskRun level. It creates a TaskRun with Timeout and wait in the Step of the
// inline Task for the time length longer than the specified Timeout.
// The TaskRun is expected to fail with the Reason `TaskRunTimeout`.
func TestTaskRunTimeout(t *testing.T) {
	expectedFailedStatus := true
	inputYAML := fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: %s
spec:
  timeout: 15s
  taskSpec:
    steps:
    - image: busybox
      command: ['/bin/sh']
      args: ['-c', 'sleep 15001']
`, helpers.ObjectNameForTest(t))

	// Execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t, expectedFailedStatus)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if err := checkTaskRunConditionSucceeded(resolvedTR.Status, "False", "TaskRunTimeout"); err != nil {
		t.Error(err)
	}
}

// TestConditions examines population of Conditions
// fields. It creates the a TaskRun with minimal specifications and checks the
// required Condition Status and Type.
func TestConditions(t *testing.T) {
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
      script:
        echo Hello world!
`, helpers.ObjectNameForTest(t))

	// The execution of Pipeline CRDs that should be implemented by Vendor service
	outputYAML, err := ProcessAndSendToTekton(inputYAML, TaskRunInputType, t)
	if err != nil {
		t.Fatalf("Vendor service failed processing inputYAML: %s", err)
	}

	// Parse and validate output YAML
	resolvedTR := parse.MustParseV1TaskRun(t, outputYAML)

	if err := checkTaskRunConditionSucceeded(resolvedTR.Status, SucceedConditionStatus, "Succeeded"); err != nil {
		t.Error(err)
	}
}
