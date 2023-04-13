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
	"github.com/tektoncd/pipeline/pkg/result"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// Simple task run sign/verify
func TestMock_TaskRunSign(t *testing.T) {
	spireMockClient := &MockClient{}
	var (
		cc ControllerAPIClient = spireMockClient
	)

	ctx := context.Background()
	var err error

	for _, tr := range testTaskRuns() {
		err = cc.AppendStatusInternalAnnotation(ctx, tr)
		if err != nil {
			t.Fatalf("failed to sign TaskRun: %v", err)
		}

		err = cc.VerifyStatusInternalAnnotation(ctx, tr, nil)
		if err != nil {
			t.Fatalf("failed to verify TaskRun: %v", err)
		}
	}
}

// test CheckSpireVerifiedFlag(tr *v1beta1.TaskRun) bool
func TestMock_CheckSpireVerifiedFlag(t *testing.T) {
	spireMockClient := &MockClient{}
	var (
		cc ControllerAPIClient = spireMockClient
	)

	trs := testTaskRuns()
	tr := trs[0]

	if !cc.CheckSpireVerifiedFlag(tr) {
		t.Fatalf("verified flag should be unset")
	}

	if tr.Status.Status.Annotations == nil {
		tr.Status.Status.Annotations = map[string]string{}
	}
	tr.Status.Status.Annotations[VerifiedAnnotation] = "no"

	if cc.CheckSpireVerifiedFlag(tr) {
		t.Fatalf("verified flag should be set")
	}
}

// Task run check signed status is not the same with two taskruns
func TestMock_CheckHashSimilarities(t *testing.T) {
	spireMockClient := &MockClient{}
	var (
		cc ControllerAPIClient = spireMockClient
	)

	ctx := context.Background()
	trs := testTaskRuns()
	tr1, tr2 := trs[0], trs[1]

	trs = testTaskRuns()
	tr1c, tr2c := trs[0], trs[1]

	tr2c.Status.Status.Annotations = map[string]string{"new": "value"}

	signTrs := []*v1beta1.TaskRun{tr1, tr1c, tr2, tr2c}

	for _, tr := range signTrs {
		err := cc.AppendStatusInternalAnnotation(ctx, tr)
		if err != nil {
			t.Fatalf("failed to sign TaskRun: %v", err)
		}
	}

	if getHash(tr1) != getHash(tr1c) {
		t.Fatalf("2 hashes of the same status should be same")
	}

	if getHash(tr1) == getHash(tr2) {
		t.Fatalf("2 hashes of different status should not be the same")
	}

	if getHash(tr2) != getHash(tr2c) {
		t.Fatalf("2 hashes of the same status should be same (ignoring Status.Status)")
	}
}

// Task run sign, modify signature/hash/svid/content and verify
func TestMock_CheckTamper(t *testing.T) {
	tests := []struct {
		// description of test case
		desc string
		// annotations to set
		setAnnotations map[string]string
		// modify the status
		modifyStatus bool
		// modify the hash to match the new status but not the signature
		modifyHashToMatch bool
		// if test should pass
		verify bool
	}{
		{
			desc:   "tamper nothing",
			verify: true,
		},
		{
			desc: "tamper unrelated hash",
			setAnnotations: map[string]string{
				"unrelated-hash": "change",
			},
			verify: true,
		},
		{
			desc: "tamper status hash",
			setAnnotations: map[string]string{
				TaskRunStatusHashAnnotation: "change-hash",
			},
			verify: false,
		},
		{
			desc: "tamper sig",
			setAnnotations: map[string]string{
				taskRunStatusHashSigAnnotation: "change-sig",
			},
			verify: false,
		},
		{
			desc: "tamper SVID",
			setAnnotations: map[string]string{
				controllerSvidAnnotation: "change-svid",
			},
			verify: false,
		},
		{
			desc: "delete status hash",
			setAnnotations: map[string]string{
				TaskRunStatusHashAnnotation: "",
			},
			verify: false,
		},
		{
			desc: "delete sig",
			setAnnotations: map[string]string{
				taskRunStatusHashSigAnnotation: "",
			},
			verify: false,
		},
		{
			desc: "delete SVID",
			setAnnotations: map[string]string{
				controllerSvidAnnotation: "",
			},
			verify: false,
		},
		{
			desc:         "tamper status",
			modifyStatus: true,
			verify:       false,
		},
		{
			desc:              "tamper status and status hash",
			modifyStatus:      true,
			modifyHashToMatch: true,
			verify:            false,
		},
	}
	for _, tt := range tests {
		spireMockClient := &MockClient{}
		var (
			cc ControllerAPIClient = spireMockClient
		)

		ctx := context.Background()
		for _, tr := range testTaskRuns() {
			err := cc.AppendStatusInternalAnnotation(ctx, tr)
			if err != nil {
				t.Fatalf("failed to sign TaskRun: %v", err)
			}

			if tr.Status.Status.Annotations == nil {
				tr.Status.Status.Annotations = map[string]string{}
			}

			if tt.setAnnotations != nil {
				for k, v := range tt.setAnnotations {
					tr.Status.Status.Annotations[k] = v
				}
			}

			if tt.modifyStatus {
				tr.Status.TaskRunStatusFields.Steps = append(tr.Status.TaskRunStatusFields.Steps, v1beta1.StepState{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: int32(54321)},
					}})
			}

			if tt.modifyHashToMatch {
				h, _ := hashTaskrunStatusInternal(tr)
				tr.Status.Status.Annotations[TaskRunStatusHashAnnotation] = h
			}

			verified := cc.VerifyStatusInternalAnnotation(ctx, tr, nil) == nil
			if verified != tt.verify {
				t.Fatalf("test %v expected verify %v, got %v", tt.desc, tt.verify, verified)
			}
		}
	}
}

// Task result sign and verify
func TestMock_TaskRunResultsSign(t *testing.T) {
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

			for _, results := range testRunResults() {
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

					return cc.VerifyTaskRunResults(ctx, results, tr) == nil
				}()

				if success != tt.success {
					t.Fatalf("test %v expected verify %v, got %v", tt.desc, tt.success, success)
				}
			}

			if err = cc.DeleteEntry(ctx, tr, genPodObj(tr, "")); err != nil {
				t.Fatalf("unable to delete entry: %v", err)
			}
		}
	}
}

// Task result sign, modify signature/content and verify
func TestMock_TaskRunResultsSignTamper(t *testing.T) {
	spireMockClient := &MockClient{}
	var (
		cc ControllerAPIClient   = spireMockClient
		ec EntrypointerAPIClient = spireMockClient
	)

	genPr := func() []result.RunResult {
		return []result.RunResult{
			{
				Key:          "foo",
				Value:        "foo-value",
				ResourceName: "source-image",
				ResultType:   result.TaskRunResultType,
			},
			{
				Key:          "bar",
				Value:        "bar-value",
				ResourceName: "source-image2",
				ResultType:   result.TaskRunResultType,
			},
		}
	}

	testCases := []struct {
		// description of test
		desc string
		// function to tamper
		tamperFn func([]result.RunResult) []result.RunResult
		// whether sign/verify procedure should succeed
		success bool
	}{
		{
			desc:    "no tamper",
			success: true,
		},
		{
			desc: "non-intrusive tamper",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
				prs = append(prs, result.RunResult{
					Key:   "not-taskrun-result-type-add",
					Value: "abc:12345",
				})
				return prs
			},
			success: true,
		},
		{
			desc: "tamper SVID",
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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
			tamperFn: func(prs []result.RunResult) []result.RunResult {
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

				return cc.VerifyTaskRunResults(ctx, results, tr) == nil
			}()

			if success != tt.success {
				t.Fatalf("test %v expected verify %v, got %v", tt.desc, tt.success, success)
			}

			if err = cc.DeleteEntry(ctx, tr, genPodObj(tr, "")); err != nil {
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
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{
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
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{
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

func testRunResults() [][]result.RunResult {
	return [][]result.RunResult{
		// Single result
		{
			{
				Key:          "digest",
				Value:        "sha256:12345",
				ResourceName: "source-image",
				ResultType:   result.TaskRunResultType,
			},
		},
		// array result
		{
			{
				Key:          "resultName",
				Value:        "[\"hello\",\"world\"]",
				ResourceName: "source-image",
				ResultType:   result.TaskRunResultType,
			},
		},
		// array result
		{
			{
				Key:          "resultArray",
				Value:        "{\"key1\":\"var1\",\"key2\":\"var2\"}",
				ResourceName: "source-image",
				ResultType:   result.TaskRunResultType,
			},
		},
		// multi result
		{
			{
				Key:          "foo",
				Value:        "abc",
				ResourceName: "source-image",
				ResultType:   result.TaskRunResultType,
			},
			{
				Key:          "bar",
				Value:        "xyz",
				ResourceName: "source-image2",
				ResultType:   result.TaskRunResultType,
			},
		},
		// mix result type
		{
			{
				Key:          "foo",
				Value:        "abc",
				ResourceName: "source-image",
				ResultType:   result.TaskRunResultType,
			},
			{
				Key:          "bar",
				Value:        "xyz",
				ResourceName: "source-image2",
				ResultType:   result.TaskRunResultType,
			},
			{
				Key:          "resultName",
				Value:        "[\"hello\",\"world\"]",
				ResourceName: "source-image3",
				ResultType:   result.TaskRunResultType,
			},
			{
				Key:          "resultName2",
				Value:        "{\"key1\":\"var1\",\"key2\":\"var2\"}",
				ResourceName: "source-image4",
				ResultType:   result.TaskRunResultType,
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

func getHash(tr *v1beta1.TaskRun) string {
	return tr.Status.Status.Annotations[TaskRunStatusHashAnnotation]
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
