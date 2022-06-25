/*
Copyright 2022 The Tekton Authors

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

package spire

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// Task result sign and verify
func TestSpireMock_TaskRunResultsSign(t *testing.T) {
	spireMockClient := &MockClient{}
	var (
		cc ControllerAPIClient   = spireMockClient
		ec EntrypointerAPIClient = spireMockClient
	)

	testCases := []struct {
		// description of test
		desc string
		// skip entry creation of pod identity
		skipEntryCreate bool
		// set wrong pod identity for signer
		wrongPodIdentity bool
		// whether sign/verify procedure should succeed
		success bool
	}{
		{
			desc:    "regular sign/verify result",
			success: true,
		},
		{
			desc:            "sign/verify result when entry isn't created",
			skipEntryCreate: true,
			success:         false,
		},
		{
			desc:             "sign/verify result when signing with wrong pod identity",
			wrongPodIdentity: true,
			success:          false,
		},
	}

	for _, tt := range testCases {
		ctx := context.Background()
		for _, tr := range testTaskRuns() {

			var err error
			if !tt.skipEntryCreate {
				// Pod should not be nil, but it isn't used in mocking
				// implementation so should not matter
				err = cc.CreateEntries(ctx, tr, genPodObj(tr, ""), 10000)
				if err != nil {
					t.Fatalf("unable to create entry")
				}
			}

			for _, results := range testPipelineResourceResults() {
				success := func() bool {
					spireMockClient.SignIdentities = []string{spireMockClient.GetIdentity(tr)}
					if tt.wrongPodIdentity {
						spireMockClient.SignIdentities = []string{"wrong-identity"}
					}

					sigResults, err := ec.Sign(ctx, results)
					if err != nil {
						return false
					}

					results = append(results, sigResults...)

					err = cc.VerifyTaskRunResults(ctx, results, tr)
					if err != nil {
						return false
					}

					return true
				}()

				if success != tt.success {
					t.Fatalf("test %v expected verify %v, got %v", tt.desc, tt.success, success)
				}
			}

			err = cc.DeleteEntry(ctx, tr, genPodObj(tr, ""))
			if err != nil {
				t.Fatalf("unable to delete entry: %v", err)
			}
		}
	}
}

// Task result sign, modify signature/content and verify
func TestSpireMock_TaskRunResultsSignTamper(t *testing.T) {
	spireMockClient := &MockClient{}
	var (
		cc ControllerAPIClient   = spireMockClient
		ec EntrypointerAPIClient = spireMockClient
	)

	genPr := func() []v1beta1.PipelineResourceResult {
		return []v1beta1.PipelineResourceResult{
			{
				Key:          "foo",
				Value:        "foo-value",
				ResourceName: "source-image",
				ResultType:   v1beta1.TaskRunResultType,
			},
			{
				Key:          "bar",
				Value:        "bar-value",
				ResourceName: "source-image2",
				ResultType:   v1beta1.TaskRunResultType,
			},
		}
	}

	testCases := []struct {
		// description of test
		desc string
		// function to tamper
		tamperFn func([]v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult
		// whether sign/verify procedure should succeed
		success bool
	}{
		{
			desc:    "no tamper",
			success: true,
		},
		{
			desc: "non-intrusive tamper",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				prs = append(prs, v1beta1.PipelineResourceResult{
					Key:   "not-taskrun-result-type-add",
					Value: "abc:12345",
				})
				return prs
			},
			success: true,
		},
		{
			desc: "tamper SVID",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeySVID {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "tamper result manifest",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "tamper result manifest signature",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest+KeySignatureSuffix {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "tamper result field",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == "foo" {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "tamper result field signature",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == "foo"+KeySignatureSuffix {
						prs[i].Value = "tamper-value"
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete SVID",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeySVID {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete result manifest",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete result manifest signature",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest+KeySignatureSuffix {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete result field",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == "foo" {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "delete result field signature",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == "foo"+KeySignatureSuffix {
						return append(prs[:i], prs[i+1:]...)
					}
				}
				return prs
			},
			success: false,
		},
		{
			desc: "add to result manifest",
			tamperFn: func(prs []v1beta1.PipelineResourceResult) []v1beta1.PipelineResourceResult {
				for i, pr := range prs {
					if pr.Key == KeyResultManifest {
						prs[i].Value += ",xyz"
					}
				}
				return prs
			},
			success: false,
		},
	}

	for _, tt := range testCases {
		ctx := context.Background()
		for _, tr := range testTaskRuns() {

			var err error
			// Pod should not be nil, but it isn't used in mocking
			// implementation so should not matter
			err = cc.CreateEntries(ctx, tr, genPodObj(tr, ""), 10000)
			if err != nil {
				t.Fatalf("unable to create entry")
			}

			results := genPr()
			success := func() bool {
				spireMockClient.SignIdentities = []string{spireMockClient.GetIdentity(tr)}

				sigResults, err := ec.Sign(ctx, results)
				if err != nil {
					return false
				}

				results = append(results, sigResults...)
				if tt.tamperFn != nil {
					results = tt.tamperFn(results)
				}

				err = cc.VerifyTaskRunResults(ctx, results, tr)
				if err != nil {
					return false
				}

				return true
			}()

			if success != tt.success {
				t.Fatalf("test %v expected verify %v, got %v", tt.desc, tt.success, success)
			}

			err = cc.DeleteEntry(ctx, tr, genPodObj(tr, ""))
			if err != nil {
				t.Fatalf("unable to delete entry: %v", err)
			}
		}
	}
}

func objectMeta(name, ns string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   ns,
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}
}

func testTaskRuns() []*v1beta1.TaskRun {
	return []*v1beta1.TaskRun{
		// taskRun 1
		{
			ObjectMeta: objectMeta("taskrun-example", "foo"),
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name:       "taskname",
					APIVersion: "a1",
				},
				ServiceAccountName: "test-sa",
			},
		},
		// taskRun 2
		{
			ObjectMeta: objectMeta("taskrun-example-populated", "foo"),
			Spec: v1beta1.TaskRunSpec{
				TaskRef:            &v1beta1.TaskRef{Name: "unit-test-task"},
				ServiceAccountName: "test-sa",
				Resources:          &v1beta1.TaskRunResources{},
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					Steps: []v1beta1.StepState{{
						ContainerState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{ExitCode: int32(0)},
						},
					}},
				},
			},
		},
		// taskRun 3
		{
			ObjectMeta: objectMeta("taskrun-example-with-objmeta", "foo"),
			Spec: v1beta1.TaskRunSpec{
				TaskRef:            &v1beta1.TaskRef{Name: "unit-test-task"},
				ServiceAccountName: "test-sa",
				Resources:          &v1beta1.TaskRunResources{},
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{
						apis.Condition{
							Type: apis.ConditionSucceeded,
						},
					},
				},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					Steps: []v1beta1.StepState{{
						ContainerState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{ExitCode: int32(0)},
						},
					}},
				},
			},
		},
		{
			ObjectMeta: objectMeta("taskrun-example-with-objmeta-annotations", "foo"),
			Spec: v1beta1.TaskRunSpec{
				TaskRef:            &v1beta1.TaskRef{Name: "unit-test-task"},
				ServiceAccountName: "test-sa",
				Resources:          &v1beta1.TaskRunResources{},
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{
						apis.Condition{
							Type: apis.ConditionSucceeded,
						},
					},
					Annotations: map[string]string{
						"annotation1": "a1value",
						"annotation2": "a2value",
					},
				},
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					Steps: []v1beta1.StepState{{
						ContainerState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{ExitCode: int32(0)},
						},
					}},
				},
			},
		},
	}
}

func testPipelineResourceResults() [][]v1beta1.PipelineResourceResult {
	return [][]v1beta1.PipelineResourceResult{
		// Single result
		{
			{
				Key:          "digest",
				Value:        "sha256:12345",
				ResourceName: "source-image",
				ResultType:   v1beta1.TaskRunResultType,
			},
		},
		// array result
		{
			{
				Key:          "resultName",
				Value:        "[\"hello\",\"world\"]",
				ResourceName: "source-image",
				ResultType:   v1beta1.TaskRunResultType,
			},
		},
		// multi result
		{
			{
				Key:          "foo",
				Value:        "abc",
				ResourceName: "source-image",
				ResultType:   v1beta1.TaskRunResultType,
			},
			{
				Key:          "bar",
				Value:        "xyz",
				ResourceName: "source-image2",
				ResultType:   v1beta1.TaskRunResultType,
			},
		},
		// mix result type
		{
			{
				Key:          "foo",
				Value:        "abc",
				ResourceName: "source-image",
				ResultType:   v1beta1.TaskRunResultType,
			},
			{
				Key:          "bar",
				Value:        "xyz",
				ResourceName: "source-image2",
				ResultType:   v1beta1.TaskRunResultType,
			},
			{
				Key:          "resultName",
				Value:        "[\"hello\",\"world\"]",
				ResourceName: "source-image",
				ResultType:   v1beta1.TaskRunResultType,
			},
		},
		// not TaskRunResultType
		{
			{
				Key:          "not-taskrun-result-type",
				Value:        "sha256:12345",
				ResourceName: "source-image",
			},
		},
		// empty result
		{},
	}
}

func genPodObj(tr *v1beta1.TaskRun, uid string) *corev1.Pod {
	if uid == "" {
		uid = uuid.NewString()
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tr.ObjectMeta.Namespace,
			Name:      "pod-" + tr.ObjectMeta.Name,
			UID:       types.UID(uid),
		},
	}

	return pod
}
