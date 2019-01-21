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
		Resources  []PipelineDeclaredResource
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
				Tasks: []PipelineTask{{
					Name: "foo",
				}, {
					Name: "foo",
				}},
			},
		},
		{
			name: "providedby task doesnt exist",
			fields: fields{
				Resources: []PipelineDeclaredResource{{
					Name: "great-resource",
					Type: "git",
				}},
				Tasks: []PipelineTask{{
					Name: "foo",
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:       "the-resource",
							Resource:   "great-resource",
							ProvidedBy: []string{"bar"},
						}},
					},
				}},
			},
		},
		{
			name: "providedby task is afterward",
			fields: fields{
				Resources: []PipelineDeclaredResource{{
					Name: "great-resource",
					Type: "git",
				}},
				Tasks: []PipelineTask{{
					Name: "foo",
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:       "the-resource",
							Resource:   "great-resource",
							ProvidedBy: []string{"bar"},
						}},
					},
				}, {
					Name: "bar",
					Resources: &PipelineTaskResources{
						Outputs: []PipelineTaskOutputResource{{
							Name:     "the-resource",
							Resource: "great-resource",
						}},
					},
				}},
			},
		},
		{
			name: "unused resources declared",
			fields: fields{
				Resources: []PipelineDeclaredResource{{
					Name: "great-resource",
					Type: "git",
				}, {
					Name: "extra-resource",
					Type: "image",
				}},
				Tasks: []PipelineTask{{
					Name: "foo",
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:     "the-resource",
							Resource: "great-resource",
						}},
					},
				}},
			},
		},
		{
			name: "output resources missing from declaration",
			fields: fields{
				Resources: []PipelineDeclaredResource{{
					Name: "great-resource",
					Type: "git",
				}},
				Tasks: []PipelineTask{{
					Name: "foo",
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:     "the-resource",
							Resource: "great-resource",
						}},
						Outputs: []PipelineTaskOutputResource{{
							Name:     "the-magic-resource",
							Resource: "missing-resource",
						}},
					},
				}},
			},
		},
		{
			name: "input resources missing from declaration",
			fields: fields{
				Resources: []PipelineDeclaredResource{{
					Name: "great-resource",
					Type: "git",
				}},
				Tasks: []PipelineTask{{
					Name: "foo",
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:     "the-resource",
							Resource: "missing-resource",
						}},
						Outputs: []PipelineTaskOutputResource{{
							Name:     "the-magic-resource",
							Resource: "great-resource",
						}},
					},
				}},
			},
		},
		{
			name: "providedBy resource isn't output by task",
			fields: fields{
				Resources: []PipelineDeclaredResource{{
					Name: "great-resource",
					Type: "git",
				}, {
					Name: "wonderful-resource",
					Type: "image",
				}},
				Tasks: []PipelineTask{{
					Name: "bar",
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:     "some-workspace",
							Resource: "great-resource",
						}},
					},
				}, {
					Name: "foo",
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:       "wow-image",
							Resource:   "wonderful-resource",
							ProvidedBy: []string{"bar"},
						}},
					},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PipelineSpec{
				Resources:  tt.fields.Resources,
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
		Resources  []PipelineDeclaredResource
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
				Tasks: []PipelineTask{{
					Name: "foo",
				}, {
					Name: "baz",
				}},
			},
		},
		{
			name: "valid resource declarations and usage",
			fields: fields{
				Resources: []PipelineDeclaredResource{{
					Name: "great-resource",
					Type: "git",
				}, {
					Name: "wonderful-resource",
					Type: "image",
				}},
				Tasks: []PipelineTask{{
					Name: "bar",
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:     "some-workspace",
							Resource: "great-resource",
						}},
						Outputs: []PipelineTaskOutputResource{{
							Name:     "some-image",
							Resource: "wonderful-resource",
						}},
					},
				}, {
					Name: "foo",
					Resources: &PipelineTaskResources{
						Inputs: []PipelineTaskInputResource{{
							Name:       "wow-image",
							Resource:   "wonderful-resource",
							ProvidedBy: []string{"bar"},
						}},
					},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PipelineSpec{
				Resources:  tt.fields.Resources,
				Tasks:      tt.fields.Tasks,
				Generation: tt.fields.Generation,
			}
			if err := ps.Validate(); err != nil {
				t.Errorf("PipelineSpec.Validate() returned error: %v", err)
			}
		})
	}
}
