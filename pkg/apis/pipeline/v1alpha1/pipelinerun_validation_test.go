/*
Copyright 2018 The Knative Authors

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
package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var validURL = "http://www.google.com"

func validResultTarget(name string) ResultTarget {
	return ResultTarget{
		URL:  validURL,
		Name: name,
		Type: "gcs",
	}
}
func TestPipelineRun_Invalidate(t *testing.T) {
	tests := []struct {
		name string
		pr   PipelineRun
		want *apis.FieldError
	}{
		{
			name: "invalid pipelinerun",
			pr: PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prmetaname",
				},
			},
			want: apis.ErrMissingField("spec"),
		},
		{
			name: "invalid pipelinerun metadata",
			pr: PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinerun.name",
				},
			},
			want: &apis.FieldError{
				Message: "Invalid resource name: special character . must not be present",
				Paths:   []string{"metadata.name"},
			},
		}, {
			name: "no pipeline reference",
			pr: PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinelineName",
				},
				Spec: PipelineRunSpec{
					PipelineTriggerRef: PipelineTriggerRef{
						Type: PipelineTriggerTypeManual,
					},
				},
			},
			want: apis.ErrMissingField("pipelinerun.spec.Pipelineref.Name"),
		}, {
			name: "invalid trigger reference",
			pr: PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinelineName",
				},
				Spec: PipelineRunSpec{
					PipelineRef: PipelineRef{
						Name: "prname",
					},
					PipelineTriggerRef: PipelineTriggerRef{
						Type: "badtype",
					},
				},
			},
			want: apis.ErrInvalidValue("badtype", "pipelinerun.spec.triggerRef.type"),
		}, {
			name: "invalid results",
			pr: PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinelineName",
				},
				Spec: PipelineRunSpec{
					PipelineRef: PipelineRef{
						Name: "prname",
					},
					PipelineTriggerRef: PipelineTriggerRef{
						Type: PipelineTriggerTypeManual,
					},
					Results: &Results{
						Runs: ResultTarget{
							Name: "runs",
							URL:  "badurl",
							Type: "gcs",
						},
						Logs: validResultTarget("logs"),
					},
				},
			},
			want: apis.ErrInvalidValue("badurl", "pipelinerun.spec.Results.Runs.URL"),
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.pr.Validate()
			if d := cmp.Diff(err.Error(), ts.want.Error()); d != "" {
				t.Errorf("PipelineRun.Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}

func TestPipelineRun_Validate(t *testing.T) {
	tr := PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinelineName",
		},
		Spec: PipelineRunSpec{
			PipelineRef: PipelineRef{
				Name: "prname",
			},
			PipelineTriggerRef: PipelineTriggerRef{
				Type: "manual",
			},
			Results: &Results{
				Runs: validResultTarget("runs"),
				Logs: validResultTarget("logs"),
			},
		},
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("Unexpected PipelineRun.Validate() error = %v", err)
	}
}
