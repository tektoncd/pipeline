/*
Copyright 2026 The Tekton Authors

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

package pod

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/internal/sidecarlogresults"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestExtractResultsFromLogs(t *testing.T) {
	inputResults := []sidecarlogresults.SidecarLogResult{
		{
			Name:  "result1",
			Value: "foo",
			Type:  "task",
		}, {
			Name:  "result2",
			Value: "bar",
			Type:  "task",
		},
	}
	podLogs := ""
	for _, r := range inputResults {
		res, err := json.Marshal(&r)
		if err != nil {
			t.Fatal(err)
		}
		podLogs = fmt.Sprintf("%s%s\n", podLogs, string(res))
	}
	logs := strings.NewReader(podLogs)

	results, err := extractResultsFromLogs(logs, []result.RunResult{}, 4096)
	if err != nil {
		t.Error(err)
	}
	want := []result.RunResult{
		{
			Key:        "result1",
			Value:      "foo",
			ResultType: result.TaskRunResultType,
		}, {
			Key:        "result2",
			Value:      "bar",
			ResultType: result.TaskRunResultType,
		},
	}
	if d := cmp.Diff(want, results); d != "" {
		t.Fatal(diff.PrintWantGot(d))
	}
}

// TestExtractResultsFromLogs_LargeResult is a regression test for
// https://github.com/tektoncd/pipeline/issues/10402
// Results whose JSON line exceeds bufio.Reader's default 4096-byte buffer
// but is within maxResultLimit must be extracted correctly.
func TestExtractResultsFromLogs_LargeResult(t *testing.T) {
	// Value >4096 bytes but within maxResultLimit (8192)
	largeValue := strings.Repeat("x", 5000)
	inputResults := []sidecarlogresults.SidecarLogResult{
		{
			Name:  "small-result",
			Value: "hello",
			Type:  "task",
		},
		{
			Name:  "large-result",
			Value: largeValue,
			Type:  "task",
		},
		{
			Name:  "after-large",
			Value: "world",
			Type:  "task",
		},
	}
	podLogs := ""
	for _, r := range inputResults {
		res, err := json.Marshal(&r)
		if err != nil {
			t.Fatal(err)
		}
		podLogs = fmt.Sprintf("%s%s\n", podLogs, string(res))
	}
	logs := strings.NewReader(podLogs)

	results, err := extractResultsFromLogs(logs, []result.RunResult{}, 8192)
	if err != nil {
		t.Fatalf("expected no error but got: %v", err)
	}
	want := []result.RunResult{
		{
			Key:        "small-result",
			Value:      "hello",
			ResultType: result.TaskRunResultType,
		},
		{
			Key:        "large-result",
			Value:      largeValue,
			ResultType: result.TaskRunResultType,
		},
		{
			Key:        "after-large",
			Value:      "world",
			ResultType: result.TaskRunResultType,
		},
	}
	if d := cmp.Diff(want, results); d != "" {
		t.Fatalf("results mismatch (-want +got):\n%s", d)
	}
}

func TestExtractResultsFromLogs_Failure(t *testing.T) {
	inputResults := []sidecarlogresults.SidecarLogResult{
		{
			Name:  "result1",
			Value: strings.Repeat("v", 4098),
			Type:  "task",
		},
	}
	podLogs := ""
	for _, r := range inputResults {
		res, err := json.Marshal(&r)
		if err != nil {
			t.Fatal(err)
		}
		podLogs = fmt.Sprintf("%s%s\n", podLogs, string(res))
	}
	logs := strings.NewReader(podLogs)

	_, err := extractResultsFromLogs(logs, []result.RunResult{}, 4096)
	if !errors.Is(err, ErrSizeExceeded) {
		t.Fatalf("Expected error %v but got %v", ErrSizeExceeded, err)
	}
}

func TestParseResults(t *testing.T) {
	results := []sidecarlogresults.SidecarLogResult{
		{
			Name:  "result1",
			Value: "foo",
			Type:  "task",
		},
		{
			Name:  "result2",
			Value: `{"IMAGE_URL":"ar.com", "IMAGE_DIGEST":"sha234"}`,
			Type:  "task",
		},
		{
			Name:  "result3",
			Value: `["hello","world"]`,
			Type:  "task",
		},
		{
			Name:  "step-foo.result1",
			Value: "foo",
			Type:  "step",
		},
		{
			Name:  "step-foo.result2",
			Value: `{"IMAGE_URL":"ar.com", "IMAGE_DIGEST":"sha234"}`,
			Type:  "step",
		},
		{
			Name:  "step-foo.result3",
			Value: `["hello","world"]`,
			Type:  "step",
		},
		{
			Name: "step-artifacts-result",
			Value: `{
            "inputs":[
              {
                "name":"input-artifacts",
                "values":[
                  {
                    "uri":"pkg:example.github.com/inputs",
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
                    "uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
                ]
              }
            ]
          }`,
			Type: "stepArtifact",
		},
		{
			Name: "task-run-artifacts-result",
			Value: `{
            "inputs":[
              {
                "name":"input-artifacts",
                "values":[
                  {
                    "uri":"pkg:example.github.com/inputs",
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
                    "uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
                ]
              }
            ]
          }`,
			Type: "taskArtifact",
		},
	}
	podLogs := []string{}
	for _, r := range results {
		res, err := json.Marshal(&r)
		if err != nil {
			t.Fatal(err)
		}
		podLogs = append(podLogs, string(res))
	}
	want := []result.RunResult{{
		Key:        "result1",
		Value:      "foo",
		ResultType: result.TaskRunResultType,
	}, {
		Key:        "result2",
		Value:      `{"IMAGE_URL":"ar.com", "IMAGE_DIGEST":"sha234"}`,
		ResultType: result.TaskRunResultType,
	}, {
		Key:        "result3",
		Value:      `["hello","world"]`,
		ResultType: result.TaskRunResultType,
	}, {
		Key:        "step-foo.result1",
		Value:      "foo",
		ResultType: result.StepResultType,
	}, {
		Key:        "step-foo.result2",
		Value:      `{"IMAGE_URL":"ar.com", "IMAGE_DIGEST":"sha234"}`,
		ResultType: result.StepResultType,
	}, {
		Key:        "step-foo.result3",
		Value:      `["hello","world"]`,
		ResultType: result.StepResultType,
	}, {
		Key: "step-artifacts-result",
		Value: `{
            "inputs":[
              {
                "name":"input-artifacts",
                "values":[
                  {
                    "uri":"pkg:example.github.com/inputs",
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
                    "uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
                ]
              }
            ]
          }`,
		ResultType: result.StepArtifactsResultType,
	}, {
		Key: "task-run-artifacts-result",
		Value: `{
            "inputs":[
              {
                "name":"input-artifacts",
                "values":[
                  {
                    "uri":"pkg:example.github.com/inputs",
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
                    "uri":"pkg:github/package-url/purl-spec@244fd47e07d1004f0aed9c",
                    "digest":{
                      "sha256":"df85b9e3983fe2ce20ef76ad675ecf435cc99fc9350adc54fa230bae8c32ce48",
                      "sha1":"95588b8f34c31eb7d62c92aaa4e6506639b06ef2"
                    }
                  }
                ]
              }
            ]
          }`,
		ResultType: result.TaskRunArtifactsResultType,
	}}
	stepResults := []result.RunResult{}
	for _, plog := range podLogs {
		res, err := parseResults([]byte(plog), 4096)
		if err != nil {
			t.Error(err)
		}
		stepResults = append(stepResults, res)
	}
	if d := cmp.Diff(want, stepResults); d != "" {
		t.Fatal(diff.PrintWantGot(d))
	}
}

func TestParseResults_InvalidType(t *testing.T) {
	results := []sidecarlogresults.SidecarLogResult{{
		Name:  "result1",
		Value: "foo",
		Type:  "invalid",
	}}
	podLogs := []string{}
	for _, r := range results {
		res, err := json.Marshal(&r)
		if err != nil {
			t.Fatal(err)
		}
		podLogs = append(podLogs, string(res))
	}
	for _, plog := range podLogs {
		_, err := parseResults([]byte(plog), 4096)
		wantErr := errors.New("invalid sidecar result type invalid. Must be task or step or stepArtifact")
		if d := cmp.Diff(wantErr.Error(), err.Error()); d != "" {
			t.Fatal(diff.PrintWantGot(d))
		}
	}
}

func TestParseResults_Failure(t *testing.T) {
	maxResultLimit := 4096
	result := sidecarlogresults.SidecarLogResult{
		Name:  "result2",
		Value: strings.Repeat("k", 4098),
		Type:  "task",
	}
	res1, err := json.Marshal("result1 v1")
	if err != nil {
		t.Fatal(err)
	}
	res2, err := json.Marshal(&result)
	if err != nil {
		t.Fatal(err)
	}
	podLogs := []string{string(res1), string(res2)}
	want := []string{
		"invalid result \"\": json: cannot unmarshal string into Go value of type sidecarlogresults.SidecarLogResult",
		fmt.Sprintf("invalid result \"%s\": %s of %d", result.Name, ErrSizeExceeded.Error(), maxResultLimit),
	}
	got := []string{}
	for _, plog := range podLogs {
		_, err := parseResults([]byte(plog), maxResultLimit)
		got = append(got, err.Error())
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatal(diff.PrintWantGot(d))
	}
}

func TestGetResultsFromSidecarLogs(t *testing.T) {
	for _, c := range []struct {
		desc      string
		podPhase  corev1.PodPhase
		wantError bool
	}{{
		desc:      "pod pending to start",
		podPhase:  corev1.PodPending,
		wantError: false,
	}, {
		desc:      "pod running extract logs",
		podPhase:  corev1.PodRunning,
		wantError: true,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			ctx := t.Context()
			clientset := fakekubeclientset.NewSimpleClientset()
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "foo",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container",
							Image: "image",
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: c.podPhase,
				},
			}
			pod, err := clientset.CoreV1().Pods(pod.Namespace).Create(t.Context(), pod, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Error occurred while creating pod %s: %s", pod.Name, err.Error())
			}

			// Fake logs are not formatted properly so there will be an error
			_, err = getResultsFromSidecarLogs(ctx, clientset, "foo", "pod", "container", pod.Status.Phase)
			if err != nil && !c.wantError {
				t.Fatalf("did not expect an error but got: %v", err)
			}
			if c.wantError && err == nil {
				t.Fatal("expected to get an error but did not")
			}
		})
	}
}

func TestExtractStepAndResultFromSidecarResultName(t *testing.T) {
	sidecarResultName := "step-foo.resultName"
	wantResult := "resultName"
	wantStep := "step-foo"
	gotStep, gotResult, err := extractStepAndResultFromSidecarResultName(sidecarResultName)
	if err != nil {
		t.Fatalf("did not expect an error but got: %v", err)
	}
	if gotStep != wantStep {
		t.Fatalf("failed to extract step name from string %s. Expexted %s but got %s", sidecarResultName, wantStep, gotStep)
	}
	if gotResult != wantResult {
		t.Fatalf("failed to extract result name from string %s. Expexted %s but got %s", sidecarResultName, wantResult, gotResult)
	}
}

func TestExtractStepAndResultFromSidecarResultName_Error(t *testing.T) {
	sidecarResultName := "step-foo-resultName"
	_, _, err := extractStepAndResultFromSidecarResultName(sidecarResultName)
	wantErr := errors.New("invalid string step-foo-resultName : expected somtthing that looks like <stepName>.<resultName>")
	if d := cmp.Diff(wantErr.Error(), err.Error()); d != "" {
		t.Fatal(diff.PrintWantGot(d))
	}
}
