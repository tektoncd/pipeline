/*
Copyright 2023 The Tekton Authors

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

package sidecarlogresults

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestLookForResults_FanOutAndWait(t *testing.T) {
	for _, c := range []struct {
		desc    string
		Results []SidecarLogResult `json:"result"`
	}{{
		desc: "multiple results",
		Results: []SidecarLogResult{{
			Name:  "foo",
			Value: "bar",
			Type:  "task",
		}, {
			Name:  "foo2",
			Value: "bar2",
			Type:  "task",
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			dir := t.TempDir()
			resultNames := []string{}
			wantResults := []byte{}
			for _, result := range c.Results {
				createResult(t, dir, result.Name, result.Value)
				resultNames = append(resultNames, result.Name)
				encodedResult, err := json.Marshal(result)
				if err != nil {
					t.Error(err)
				}
				// encode adds a newline character at the end.
				// We need to do the same before comparing
				encodedResult = append(encodedResult, '\n')
				wantResults = append(wantResults, encodedResult...)
			}
			dir2 := t.TempDir()
			createRun(t, dir2, false)
			got := new(bytes.Buffer)
			err := LookForResults(got, dir2, dir, resultNames, "", map[string][]string{})
			if err != nil {
				t.Fatalf("Did not expect any error but got: %v", err)
			}
			// sort because the order of results is not always the same because of go routines.
			sort.Slice(wantResults, func(i int, j int) bool { return wantResults[i] < wantResults[j] })
			sort.Slice(got.Bytes(), func(i int, j int) bool { return got.Bytes()[i] < got.Bytes()[j] })
			if d := cmp.Diff(wantResults, got.Bytes()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestLookForResults(t *testing.T) {
	for _, c := range []struct {
		desc         string
		resultName   string
		resultValue  string
		createResult bool
		stepError    bool
	}{{
		desc:         "good result",
		resultName:   "foo",
		resultValue:  "bar",
		createResult: true,
		stepError:    false,
	}, {
		desc:         "empty result",
		resultName:   "foo",
		resultValue:  "",
		createResult: true,
		stepError:    true,
	}, {
		desc:         "missing result",
		resultName:   "missing",
		resultValue:  "",
		createResult: false,
		stepError:    false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			dir := t.TempDir()
			if c.createResult == true {
				createResult(t, dir, c.resultName, c.resultValue)
			}
			dir2 := t.TempDir()
			createRun(t, dir2, c.stepError)

			var want []byte
			if c.createResult == true {
				// This is the expected result
				result := SidecarLogResult{
					Name:  c.resultName,
					Value: c.resultValue,
					Type:  "task",
				}
				encodedResult, err := json.Marshal(result)
				if err != nil {
					t.Error(err)
				}
				// encode adds a newline character at the end.
				// We need to do the same before comparing
				encodedResult = append(encodedResult, '\n')
				want = encodedResult
			}
			got := new(bytes.Buffer)
			err := LookForResults(got, dir2, dir, []string{c.resultName}, "", map[string][]string{})
			if err != nil {
				t.Fatalf("Did not expect any error but got: %v", err)
			}
			if d := cmp.Diff(want, got.Bytes()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestLookForStepResults(t *testing.T) {
	for _, c := range []struct {
		desc         string
		stepName     string
		resultName   string
		resultValue  string
		createResult bool
		stepError    bool
	}{{
		desc:         "good result",
		stepName:     "step-foo",
		resultName:   "foo",
		resultValue:  "bar",
		createResult: true,
		stepError:    false,
	}, {
		desc:         "empty result",
		stepName:     "step-foo",
		resultName:   "foo",
		resultValue:  "",
		createResult: true,
		stepError:    true,
	}, {
		desc:         "missing result",
		stepName:     "step-foo",
		resultName:   "missing",
		resultValue:  "",
		createResult: false,
		stepError:    false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			dir := t.TempDir()
			if c.createResult == true {
				createStepResult(t, dir, c.stepName, c.resultName, c.resultValue)
			}
			dir2 := t.TempDir()
			createRun(t, dir2, c.stepError)

			var want []byte
			if c.createResult == true {
				// This is the expected result
				result := SidecarLogResult{
					Name:  fmt.Sprintf("%s.%s", c.stepName, c.resultName),
					Value: c.resultValue,
					Type:  "step",
				}
				encodedResult, err := json.Marshal(result)
				if err != nil {
					t.Error(err)
				}
				// encode adds a newline character at the end.
				// We need to do the same before comparing
				encodedResult = append(encodedResult, '\n')
				want = encodedResult
			}
			got := new(bytes.Buffer)
			stepResults := map[string][]string{
				c.stepName: {c.resultName},
			}
			err := LookForResults(got, dir2, "", []string{}, dir, stepResults)
			if err != nil {
				t.Fatalf("Did not expect any error but got: %v", err)
			}
			if d := cmp.Diff(want, got.Bytes()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestExtractResultsFromLogs(t *testing.T) {
	inputResults := []SidecarLogResult{
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
		res, _ := json.Marshal(&r)
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

func TestExtractResultsFromLogs_Failure(t *testing.T) {
	inputResults := []SidecarLogResult{
		{
			Name:  "result1",
			Value: strings.Repeat("v", 4098),
			Type:  "task",
		},
	}
	podLogs := ""
	for _, r := range inputResults {
		res, _ := json.Marshal(&r)
		podLogs = fmt.Sprintf("%s%s\n", podLogs, string(res))
	}
	logs := strings.NewReader(podLogs)

	_, err := extractResultsFromLogs(logs, []result.RunResult{}, 4096)
	if !errors.Is(err, ErrSizeExceeded) {
		t.Fatalf("Expected error %v but got %v", ErrSizeExceeded, err)
	}
}

func TestParseResults(t *testing.T) {
	results := []SidecarLogResult{
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
		res, _ := json.Marshal(&r)
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
	results := []SidecarLogResult{{
		Name:  "result1",
		Value: "foo",
		Type:  "invalid",
	}}
	podLogs := []string{}
	for _, r := range results {
		res, _ := json.Marshal(&r)
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
	result := SidecarLogResult{
		Name:  "result2",
		Value: strings.Repeat("k", 4098),
		Type:  "task",
	}
	res1, _ := json.Marshal("result1 v1")
	res2, _ := json.Marshal(&result)
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
			_, err = GetResultsFromSidecarLogs(ctx, clientset, "foo", "pod", "container", pod.Status.Phase)
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
	gotStep, gotResult, err := ExtractStepAndResultFromSidecarResultName(sidecarResultName)
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
	_, _, err := ExtractStepAndResultFromSidecarResultName(sidecarResultName)
	wantErr := errors.New("invalid string step-foo-resultName : expected somtthing that looks like <stepName>.<resultName>")
	if d := cmp.Diff(wantErr.Error(), err.Error()); d != "" {
		t.Fatal(diff.PrintWantGot(d))
	}
}

func TestLookForArtifacts(t *testing.T) {
	base := basicArtifacts()
	modified := base.DeepCopy()
	modified.Outputs[0].Name = "tests"
	type Arg struct {
		stepName      string
		artifacts     *v1.Artifacts
		customContent []byte
	}
	tests := []struct {
		desc     string
		wantErr  bool
		args     []Arg
		expected []SidecarLogResult
	}{
		{
			desc: "one step produces artifacts, read success",
			args: []Arg{{stepName: "first", artifacts: &base}},
			expected: []SidecarLogResult{{
				Name:  "first",
				Type:  stepArtifactType,
				Value: mustJSON(&base),
			}},
		},
		{
			desc: "two step produce artifacts, read success",
			args: []Arg{{stepName: "first", artifacts: &base}, {stepName: "second", artifacts: modified}},
			expected: []SidecarLogResult{{
				Name:  "first",
				Type:  stepArtifactType,
				Value: mustJSON(&base),
			}, {
				Name:  "second",
				Type:  stepArtifactType,
				Value: mustJSON(modified),
			}},
		},
		{
			desc: "one step produces artifacts,  one step does not, read success",
			args: []Arg{{stepName: "first", artifacts: &base}, {stepName: "second"}},
			expected: []SidecarLogResult{{
				Name:  "first",
				Type:  stepArtifactType,
				Value: mustJSON(&base),
			}},
		},
		{
			desc: "two step produces,  one read success, one not, error out and result is not empty.",
			args: []Arg{{stepName: "first", artifacts: &base}, {stepName: "second", artifacts: modified, customContent: []byte("this is to break json")}},
			expected: []SidecarLogResult{{
				Name:  "first",
				Type:  stepArtifactType,
				Value: mustJSON(&base),
			}},
			wantErr: true,
		},
		{
			desc:     "two step produces,  first read fails,  error out and result is empty.",
			args:     []Arg{{stepName: "first", artifacts: modified, customContent: []byte("this is to break json")}, {stepName: "second", artifacts: &base}},
			expected: []SidecarLogResult{},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			dir := t.TempDir()
			curStepDir := stepDir
			stepDir = dir
			t.Cleanup(func() {
				stepDir = curStepDir
			})

			var names []string
			for _, arg := range tc.args {
				names = append(names, arg.stepName)
				if err := os.MkdirAll(filepath.Join(dir, arg.stepName, "artifacts"), os.ModePerm); err != nil {
					t.Errorf("failed to create artifacts folder, err: %v", err)
				}
				if _, err := os.Create(filepath.Join(dir, arg.stepName, "out")); err != nil {
					t.Errorf("failed to file, err: %v", err)
				}
				if arg.artifacts != nil {
					if err := writeArtifacts(filepath.Join(dir, arg.stepName, "artifacts", "provenance.json"), arg.artifacts); err != nil {
						t.Errorf("failed to write artifacts to provenance.json, err: %v", err)
					}
				}
				if arg.customContent != nil {
					if err := os.WriteFile(filepath.Join(dir, arg.stepName, "artifacts", "provenance.json"), arg.customContent, os.ModePerm); err != nil {
						t.Errorf("failed to write customContent to provenance.json, err: %v", err)
					}
				}
			}
			var buf bytes.Buffer
			err := LookForArtifacts(&buf, names, dir)
			if (err != nil) != tc.wantErr {
				t.Errorf("error checking failed, wantErr: %v, got: %v", tc.wantErr, err)
			}
			want := ""
			for _, logResult := range tc.expected {
				want += mustJSON(logResult) + "\n"
			}
			got := buf.String()

			if d := cmp.Diff(want, got); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func writeArtifacts(path string, artifacts *v1.Artifacts) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	res := json.NewEncoder(f).Encode(artifacts)
	return res
}

func basicArtifacts() v1.Artifacts {
	data := `{
            "inputs":[
              {
                "name":"inputs",
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
          }
`
	var ars v1.Artifacts
	err := json.Unmarshal([]byte(data), &ars)
	if err != nil {
		panic(err)
	}
	return ars
}

func createStepResult(t *testing.T, dir, stepName, resultName, resultValue string) {
	t.Helper()
	resultDir := filepath.Join(dir, stepName, "results")
	_ = os.MkdirAll(resultDir, 0o755)
	resultFile := filepath.Join(resultDir, resultName)
	err := os.WriteFile(resultFile, []byte(resultValue), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}

func createResult(t *testing.T, dir string, resultName string, resultValue string) {
	t.Helper()
	resultFile := filepath.Join(dir, resultName)
	err := os.WriteFile(resultFile, []byte(resultValue), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}

func createRun(t *testing.T, dir string, causeErr bool) {
	t.Helper()
	stepFile := filepath.Join(dir, "1")
	_ = os.Mkdir(stepFile, 0o755)
	stepFile = filepath.Join(stepFile, "out")
	if causeErr {
		stepFile += ".err"
	}
	err := os.WriteFile(stepFile, []byte(""), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}

func mustJSON(data any) string {
	marshal, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return string(marshal)
}
