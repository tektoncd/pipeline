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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/result"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestLookForResults_FanOutAndWait(t *testing.T) {
	for _, c := range []struct {
		desc    string
		results []SidecarLogResult
	}{{
		desc: "multiple results",
		results: []SidecarLogResult{{
			Name:  "foo",
			Value: "bar",
		}, {
			Name:  "foo2",
			Value: "bar2",
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			dir := t.TempDir()
			resultNames := []string{}
			wantResults := []byte{}
			for _, result := range c.results {
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
			err := LookForResults(got, dir2, dir, resultNames)
			if err != nil {
				t.Fatalf("Did not expect any error but got: %v", err)
			}
			// sort because the order of results is not always the same because of go routines.
			sort.Slice(wantResults, func(i int, j int) bool { return wantResults[i] < wantResults[j] })
			sort.Slice(got.Bytes(), func(i int, j int) bool { return got.Bytes()[i] < got.Bytes()[j] })
			if d := cmp.Diff(wantResults, got.Bytes()); d != "" {
				t.Errorf(diff.PrintWantGot(d))
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
			err := LookForResults(got, dir2, dir, []string{c.resultName})
			if err != nil {
				t.Fatalf("Did not expect any error but got: %v", err)
			}
			if d := cmp.Diff(want, got.Bytes()); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestExtractResultsFromLogs(t *testing.T) {
	inputResults := []SidecarLogResult{
		{
			Name:  "result1",
			Value: "foo",
		}, {
			Name:  "result2",
			Value: "bar",
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
		}, {
			Name:  "result2",
			Value: `{"IMAGE_URL":"ar.com", "IMAGE_DIGEST":"sha234"}`,
		}, {
			Name:  "result3",
			Value: `["hello","world"]`,
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

func TestParseResults_Failure(t *testing.T) {
	result := SidecarLogResult{
		Name:  "result2",
		Value: strings.Repeat("k", 4098),
	}
	res1, _ := json.Marshal("result1 v1")
	res2, _ := json.Marshal(&result)
	podLogs := []string{string(res1), string(res2)}
	want := []string{
		"Invalid result json: cannot unmarshal string into Go value of type sidecarlogresults.SidecarLogResult",
		ErrSizeExceeded.Error(),
	}
	got := []string{}
	for _, plog := range podLogs {
		_, err := parseResults([]byte(plog), 4096)
		got = append(got, err.Error())
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatal(diff.PrintWantGot(d))
	}
}

func TestGetResultsFromSidecarLogs(t *testing.T) {
	for _, c := range []struct {
		desc      string
		podPhase  v1.PodPhase
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
			ctx := context.Background()
			clientset := fakekubeclientset.NewSimpleClientset()
			pod := &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "foo",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "container",
							Image: "image",
						},
					},
				},
				Status: v1.PodStatus{
					Phase: c.podPhase,
				},
			}
			pod, err := clientset.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
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

func createResult(t *testing.T, dir string, resultName string, resultValue string) {
	t.Helper()
	resultFile := filepath.Join(dir, resultName)
	err := os.WriteFile(resultFile, []byte(resultValue), 0644)
	if err != nil {
		t.Fatal(err)
	}
}

func createRun(t *testing.T, dir string, causeErr bool) {
	t.Helper()
	stepFile := filepath.Join(dir, "1")
	_ = os.Mkdir(stepFile, 0755)
	stepFile = filepath.Join(stepFile, "out")
	if causeErr {
		stepFile += ".err"
	}
	err := os.WriteFile(stepFile, []byte(""), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
