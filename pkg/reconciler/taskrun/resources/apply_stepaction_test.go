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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/pipeline/test/diff"
)

// TestReplacementsArrayIdxStepResults checks that an indexed reference to an array parameter that is
// bound to a Step Result is found in every field of the composed Step where parameter references are
// later interpolated, not only in command, args and env.
func TestReplacementsArrayIdxStepResults(t *testing.T) {
	want := map[string]string{"params.p[1]": "$(steps.s.results.arr[1])"}
	tcs := []struct {
		name string
		step v1.Step
		want map[string]string
	}{{
		name: "command",
		step: v1.Step{Command: []string{"echo $(params.p[1])"}},
		want: want,
	}, {
		name: "args",
		step: v1.Step{Args: []string{"echo $(params.p[1])"}},
		want: want,
	}, {
		name: "env",
		step: v1.Step{Env: []corev1.EnvVar{{Name: "E", Value: "value-$(params.p[1])"}}},
		want: want,
	}, {
		name: "script",
		step: v1.Step{Script: "echo $(params.p[1])"},
		want: want,
	}, {
		name: "image",
		step: v1.Step{Image: "img:$(params.p[1])"},
		want: want,
	}, {
		name: "workingDir",
		step: v1.Step{WorkingDir: "/wd/$(params.p[1])"},
		want: want,
	}, {
		name: "env valueFrom",
		step: v1.Step{Env: []corev1.EnvVar{{
			Name: "E",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef:    &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "sec-$(params.p[1])"}, Key: "k-$(params.p[1])"},
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "cm-$(params.p[1])"}, Key: "k-$(params.p[1])"},
			},
		}}},
		want: want,
	}, {
		name: "volumeMounts",
		step: v1.Step{VolumeMounts: []corev1.VolumeMount{{
			Name:      "vol-$(params.p[1])",
			MountPath: "/m/$(params.p[1])",
			SubPath:   "sp/$(params.p[1])",
		}}},
		want: want,
	}, {
		name: "no indexed usage",
		step: v1.Step{Script: "echo $(params.p[*])"},
		want: map[string]string{},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := replacementsArrayIdxStepResults(&tc.step, "p", "s", "arr")
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("replacementsArrayIdxStepResults() %s", diff.PrintWantGot(d))
			}
		})
	}
}

// TestGetStepActionsDataIndexedStepResultInScript composes a Step that references a StepAction whose only
// indexed use of the array parameter bound to a Step Result is in its script, and checks that the resolved
// script still carries the Step Result reference for the entrypoint to resolve at run time.
func TestGetStepActionsDataIndexedStepResultInScript(t *testing.T) {
	stepAction := &v1beta1.StepAction{
		ObjectMeta: metav1.ObjectMeta{Name: "echo", Namespace: "default"},
		Spec: v1beta1.StepActionSpec{
			Params: v1.ParamSpecs{{Name: "p", Type: v1.ParamTypeArray}},
			Script: "echo $(params.p[1])",
		},
	}
	taskSpec := v1.TaskSpec{
		Steps: []v1.Step{{
			Name:   "step1",
			Ref:    &v1.Ref{Name: "echo"},
			Params: v1.Params{{Name: "p", Value: *v1.NewStructuredValues("$(steps.pre.results.foo)")}},
		}},
	}
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "mytaskrun", Namespace: "default"},
		Spec:       v1.TaskRunSpec{TaskSpec: &taskSpec},
	}

	tektonclient := fake.NewSimpleClientset()
	if err := tektonclient.Tracker().Add(stepAction); err != nil {
		t.Fatal(err)
	}

	steps, err := GetStepActionsData(t.Context(), taskSpec, tr, tektonclient, nil, nil)
	if err != nil {
		t.Fatalf("GetStepActionsData() returned error: %v", err)
	}
	if want := "echo $(steps.pre.results.foo[1])"; steps[0].Script != want {
		t.Errorf("resolved step script: %s", diff.PrintWantGot(cmp.Diff(want, steps[0].Script)))
	}
}
