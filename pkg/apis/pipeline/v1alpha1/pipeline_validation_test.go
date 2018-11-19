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
)

func TestPipelineSpec_Validate_Error(t *testing.T) {
	type fields struct {
		Tasks      []PipelineTask
		Generation int64
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "duplicate tasks",
			fields: fields{
				Tasks: []PipelineTask{
					{
						Name: "foo",
					},
					{
						Name: "foo",
					},
				},
			},
		},
		{
			name: "invalid constraint tasks",
			fields: fields{
				Tasks: []PipelineTask{
					{
						Name: "foo",
						InputSourceBindings: []SourceBinding{
							{
								ProvidedBy: []string{"bar"},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PipelineSpec{
				Tasks:      tt.fields.Tasks,
				Generation: tt.fields.Generation,
			}
			if err := ps.Validate(); err == nil {
				t.Error("PipelineSpec.Validate() did not return error, wanted error")
			}
		})
	}
}

func TestPipelineSpec_Validate_Valid(t *testing.T) {
	type fields struct {
		Tasks      []PipelineTask
		Generation int64
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "no duplicate tasks",
			fields: fields{
				Tasks: []PipelineTask{
					{
						Name: "foo",
					},
					{
						Name: "baz",
					},
				},
			},
		},
		{
			name: "valid constraint tasks",
			fields: fields{
				Tasks: []PipelineTask{
					{
						Name: "foo",
						InputSourceBindings: []SourceBinding{
							{
								ProvidedBy: []string{"bar"},
							},
						},
					},
					{
						Name: "bar",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PipelineSpec{
				Tasks:      tt.fields.Tasks,
				Generation: tt.fields.Generation,
			}
			if err := ps.Validate(); err != nil {
				t.Errorf("PipelineSpec.Validate() returned error: %v", err)
			}
		})
	}
}
