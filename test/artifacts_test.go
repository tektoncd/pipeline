//go:build e2e
// +build e2e

// /*
// Copyright 2024 The Tekton Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var (
	requireEnableStepArtifactsGate = map[string]string{
		"enable-artifacts": "true",
	}
)

func TestSurfaceArtifacts(t *testing.T) {
	tests := []struct {
		desc                   string
		resultExtractionMethod string
	}{
		{
			desc:                   "surface artifacts through termination message",
			resultExtractionMethod: config.ResultExtractionMethodTerminationMessage},
		{
			desc:                   "surface artifacts through sidecar logs",
			resultExtractionMethod: config.ResultExtractionMethodSidecarLogs},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			featureFlags := getFeatureFlagsBaseOnAPIFlag(t)
			checkFlagsEnabled := requireAllGates(requireEnableStepArtifactsGate)

			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)
			checkFlagsEnabled(ctx, t, c, "")
			previous := featureFlags.ResultExtractionMethod
			updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
				"results-from": tc.resultExtractionMethod,
			})

			knativetest.CleanupOnInterrupt(func() {
				updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
					"results-from": previous,
				})
				tearDown(ctx, t, c, namespace)
			}, t.Logf)
			defer func() {
				updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
					"results-from": previous,
				})
				tearDown(ctx, t, c, namespace)
			}()

			taskRunName := helpers.ObjectNameForTest(t)

			fqImageName := getTestImage(busyboxImage)

			t.Logf("Creating Task and TaskRun in namespace %s", namespace)
			task := simpleArtifactProducerTask(t, namespace, fqImageName)
			if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Task: %s", err)
			}
			taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, taskRunName, namespace, task.Name))
			if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun: %s", err)
			}

			if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
			}

			taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
			}
			if d := cmp.Diff([]v1.TaskRunStepArtifact{{Name: "source",
				Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha256": "b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"},
					Uri: "git:jjjsss",
				}},
			}}, taskrun.Status.Steps[0].Inputs); d != "" {
				t.Fatalf(`The expected stepState Inputs does not match created taskrun stepState Inputs. Here is the diff: %v`, d)
			}
			if d := cmp.Diff([]v1.TaskRunStepArtifact{{Name: "image", BuildOutput: true,
				Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha1": "95588b8f34c31eb7d62c92aaa4e6506639b06ef2", "sha256": "df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48"},
					Uri: "pkg:balba",
				}},
			}}, taskrun.Status.Steps[0].Outputs); d != "" {
				t.Fatalf(`The expected stepState Outputs does not match created taskrun stepState Outputs. Here is the diff: %v`, d)
			}
		})
	}
}

func TestSurfaceArtifactsThroughTerminationMessageScriptProducesArtifacts(t *testing.T) {
	featureFlags := getFeatureFlagsBaseOnAPIFlag(t)
	checkFlagsEnabled := requireAllGates(requireEnableStepArtifactsGate)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	checkFlagsEnabled(ctx, t, c, "")
	previous := featureFlags.ResultExtractionMethod
	updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
		"results-from": config.ResultExtractionMethodTerminationMessage,
	})

	knativetest.CleanupOnInterrupt(func() {
		updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
			"results-from": previous,
		})
		tearDown(ctx, t, c, namespace)
	}, t.Logf)
	defer func() {
		updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
			"results-from": previous,
		})
		tearDown(ctx, t, c, namespace)
	}()

	taskRunName := helpers.ObjectNameForTest(t)

	fqImageName := getTestImage(busyboxImage)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := simpleArtifactScriptProducerTask(t, namespace, fqImageName)
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, taskRunName, namespace, task.Name))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
	}
	if d := cmp.Diff([]v1.TaskRunStepArtifact{{Name: "input-artifacts",
		Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha256": "b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"},
			Uri: "git:jjjsss",
		}},
	}}, taskrun.Status.Steps[0].Inputs); d != "" {
		t.Fatalf(`The expected stepState Inputs does not match created taskrun stepState Inputs. Here is the diff: %v`, d)
	}
	if d := cmp.Diff([]v1.TaskRunStepArtifact{{Name: "build-result", BuildOutput: false,
		Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha1": "95588b8f34c31eb7d62c92aaa4e6506639b06ef2", "sha256": "df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48"},
			Uri: "pkg:balba",
		}},
	}}, taskrun.Status.Steps[0].Outputs); d != "" {
		t.Fatalf(`The expected stepState Outputs does not match created taskrun stepState Outputs. Here is the diff: %v`, d)
	}
}

func TestConsumeArtifacts(t *testing.T) {
	tests := []struct {
		desc                   string
		resultExtractionMethod string
	}{
		{
			desc:                   "surface artifacts through termination message",
			resultExtractionMethod: config.ResultExtractionMethodTerminationMessage},
		{
			desc:                   "surface artifacts through sidecar logs",
			resultExtractionMethod: config.ResultExtractionMethodSidecarLogs},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			featureFlags := getFeatureFlagsBaseOnAPIFlag(t)
			checkFlagsEnabled := requireAllGates(map[string]string{
				"enable-artifacts":    "true",
				"enable-step-actions": "true",
			})

			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)
			checkFlagsEnabled(ctx, t, c, "")
			previous := featureFlags.ResultExtractionMethod
			updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
				"results-from": tc.resultExtractionMethod,
			})

			knativetest.CleanupOnInterrupt(func() {
				updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
					"results-from": previous,
				})
				tearDown(ctx, t, c, namespace)
			}, t.Logf)

			defer func() {
				updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
					"results-from": previous,
				})
				tearDown(ctx, t, c, namespace)
			}()
			taskRunName := helpers.ObjectNameForTest(t)

			fqImageName := getTestImage(busyboxImage)

			t.Logf("Creating Task and TaskRun in namespace %s", namespace)
			task := simpleArtifactProducerTask(t, namespace, fqImageName)
			task.Spec.Steps = append(task.Spec.Steps,
				v1.Step{Name: "consume-outputs", Image: fqImageName,
					Command: []string{"sh", "-c", "echo -n $(steps.hello.outputs.image) >> $(step.results.result1.path)"},
					Results: []v1.StepResult{{Name: "result1", Type: v1.ResultsTypeString}}},
				v1.Step{Name: "consume-inputs", Image: fqImageName,
					Command: []string{"sh", "-c", "echo -n $(steps.hello.inputs.source) >> $(step.results.result2.path)"},
					Results: []v1.StepResult{{Name: "result2", Type: v1.ResultsTypeString}}},
			)
			if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Task: %s", err)
			}
			taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, taskRunName, namespace, task.Name))
			if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun: %s", err)
			}

			if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
			}

			taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
			}
			wantOut := `[{digest:{sha1:95588b8f34c31eb7d62c92aaa4e6506639b06ef2,sha256:df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48},uri:pkg:balba}]`
			gotOut := taskrun.Status.Steps[1].Results[0].Value.StringVal
			if d := cmp.Diff(wantOut, gotOut); d != "" {
				t.Fatalf(`The expected artifact outputs consumption result doesnot match expected. Here is the diff: %v`, d)
			}
			wantIn := `[{digest:{sha256:b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0},uri:git:jjjsss}]`
			gotIn := taskrun.Status.Steps[2].Results[0].Value.StringVal
			if d := cmp.Diff(wantIn, gotIn); d != "" {
				t.Fatalf(`The expected artifact Inputs consumption result doesnot match expected. Here is the diff: %v`, d)
			}
		})
	}
}

func TestStepProduceResultsAndArtifacts(t *testing.T) {
	tests := []struct {
		desc                   string
		resultExtractionMethod string
	}{
		{
			desc:                   "surface artifacts through termination message",
			resultExtractionMethod: config.ResultExtractionMethodTerminationMessage},
		{
			desc:                   "surface artifacts through sidecar logs",
			resultExtractionMethod: config.ResultExtractionMethodSidecarLogs},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			featureFlags := getFeatureFlagsBaseOnAPIFlag(t)
			checkFlagsEnabled := requireAllGates(map[string]string{
				"enable-artifacts":    "true",
				"enable-step-actions": "true",
			})

			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)
			checkFlagsEnabled(ctx, t, c, "")
			previous := featureFlags.ResultExtractionMethod
			updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
				"results-from": tc.resultExtractionMethod,
			})

			knativetest.CleanupOnInterrupt(func() {
				updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
					"results-from": previous,
				})
				tearDown(ctx, t, c, namespace)
			}, t.Logf)

			defer func() {
				updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
					"results-from": previous,
				})
				tearDown(ctx, t, c, namespace)
			}()
			taskRunName := helpers.ObjectNameForTest(t)

			fqImageName := getTestImage(busyboxImage)

			t.Logf("Creating Task and TaskRun in namespace %s", namespace)
			task := produceResultsAndArtifactsTask(t, namespace, fqImageName)

			if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Task: %s", err)
			}
			taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, taskRunName, namespace, task.Name))
			if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create TaskRun: %s", err)
			}

			if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1Version); err != nil {
				t.Errorf("Error waiting for TaskRun to finish: %s", err)
			}

			taskrun, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
			}
			if d := cmp.Diff([]v1.TaskRunStepArtifact{{Name: "source",
				Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha256": "b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"},
					Uri: "git:jjjsss",
				}},
			}}, taskrun.Status.Steps[0].Inputs); d != "" {
				t.Fatalf(`The expected stepState Inputs does not match created taskrun stepState Inputs. Here is the diff: %v`, d)
			}
			if d := cmp.Diff([]v1.TaskRunStepArtifact{{Name: "image",
				Values: []v1.ArtifactValue{{Digest: map[v1.Algorithm]string{"sha1": "95588b8f34c31eb7d62c92aaa4e6506639b06ef2", "sha256": "df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48"},
					Uri: "pkg:balba",
				}},
			}}, taskrun.Status.Steps[0].Outputs); d != "" {
				t.Fatalf(`The expected stepState Outputs does not match created taskrun stepState Outputs. Here is the diff: %v`, d)
			}

			wantResult := `result1Value`
			gotResult := taskrun.Status.Steps[0].Results[0].Value.StringVal
			if d := cmp.Diff(wantResult, gotResult); d != "" {
				t.Fatalf(`The expected artifact outputs consumption result doesnot match expected. Here is the diff: %v`, d)
			}
		})
	}
}

func simpleArtifactProducerTask(t *testing.T, namespace string, fqImageName string) *v1.Task {
	t.Helper()
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: %s
    command: ['/bin/sh']
    args: 
      - "-c"
      - |
        cat > $(step.artifacts.path) << EOF
          {
            "inputs":[
              {
                "name":"source",
                "values":[
                  {
                    "uri":"git:jjjsss",
                    "digest":{
                      "sha256":"b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"
                    }
                  }
                ]
              }
            ],
            "outputs":[
              {
                "name":"image",
                "buildOutput":true,
                "values":[
                  {
                    "uri":"pkg:balba",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
                ]
              }
            ]
          }
        EOF
`, helpers.ObjectNameForTest(t), namespace, fqImageName))
	return task
}

func produceResultsAndArtifactsTask(t *testing.T, namespace string, fqImageName string) *v1.Task {
	t.Helper()
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: %s
    command: ['/bin/sh']
    results:
      - name: result1
    args: 
      - "-c"
      - |
        cat > $(step.artifacts.path) << EOF
          {
            "inputs":[
              {
                "name":"source",
                "values":[
                  {
                    "uri":"git:jjjsss",
                    "digest":{
                      "sha256":"b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"
                    }
                  }
                ]
              }
            ],
            "outputs":[
              {
                "name":"image",
                "values":[
                  {
                    "uri":"pkg:balba",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
                ]
              }
            ]
          }
        EOF
        echo -n result1Value >> $(step.results.result1.path)
`, helpers.ObjectNameForTest(t), namespace, fqImageName))
	return task
}

func simpleArtifactScriptProducerTask(t *testing.T, namespace string, fqImageName string) *v1.Task {
	t.Helper()
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: %s
    script: | 
        cat > $(step.artifacts.path) << EOF
          {
            "inputs":[
              {
                "name":"input-artifacts",
                "values":[
                  {
                    "uri":"git:jjjsss",
                    "digest":{
                      "sha256":"b35cacccfdb1e24dc497d15d553891345fd155713ffe647c281c583269eaaae0"
                    }
                  }
                ]
              }
            ],
            "outputs":[
              {
                "name":"build-result",
                "buildOutput":false,
                "values":[
                  {
                    "uri":"pkg:balba",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
                ]
              }
            ]
          }
        EOF
`, helpers.ObjectNameForTest(t), namespace, fqImageName))
	return task
}
