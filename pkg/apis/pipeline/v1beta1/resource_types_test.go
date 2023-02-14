/*
Copyright 2019 The Tekton Authors
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

package v1beta1_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
)

var (
	prependStep = v1beta1.Step{
		Name:    "prepend-step",
		Image:   "dummy",
		Command: []string{"doit"},
		Args:    []string{"stuff", "things"},
	}
	appendStep = v1beta1.Step{
		Name:    "append-step",
		Image:   "dummy",
		Command: []string{"doit"},
		Args:    []string{"other stuff", "other things"},
	}
	volume = corev1.Volume{
		Name: "magic-volume",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "some-claim"},
		},
	}
)

type TestTaskModifier struct{}

func (tm *TestTaskModifier) GetStepsToPrepend() []v1beta1.Step {
	return []v1beta1.Step{prependStep}
}

func (tm *TestTaskModifier) GetStepsToAppend() []v1beta1.Step {
	return []v1beta1.Step{appendStep}
}

func (tm *TestTaskModifier) GetVolumes() []corev1.Volume {
	return []corev1.Volume{volume}
}

func TestPipelineResourceResult_UnmarshalJSON(t *testing.T) {
	testcases := []struct {
		name string
		data string
		pr   v1beta1.PipelineResourceResult
	}{{
		name: "type defined as string - TaskRunResult",
		data: "{\"key\":\"resultName\",\"value\":\"resultValue\", \"type\": \"TaskRunResult\"}",
		pr:   v1beta1.PipelineResourceResult{Key: "resultName", Value: "resultValue", ResultType: v1beta1.TaskRunResultType},
	},
		{
			name: "type defined as string - InternalTektonResult",
			data: "{\"key\":\"resultName\",\"value\":\"\", \"type\": \"InternalTektonResult\"}",
			pr:   v1beta1.PipelineResourceResult{Key: "resultName", Value: "", ResultType: v1beta1.InternalTektonResultType},
		}, {
			name: "type defined as int",
			data: "{\"key\":\"resultName\",\"value\":\"\", \"type\": 1}",
			pr:   v1beta1.PipelineResourceResult{Key: "resultName", Value: "", ResultType: v1beta1.TaskRunResultType},
		}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			pipRes := v1beta1.PipelineResourceResult{}
			if err := json.Unmarshal([]byte(tc.data), &pipRes); err != nil {
				t.Errorf("Unexpected error when unmarshalling the json into PipelineResourceResult")
			}
			if d := cmp.Diff(tc.pr, pipRes); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}
