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

var validURL = "http://www.google.com"

var validServiceAccount = "myserviceaccount"

func validResultTarget(name string) ResultTarget {
	return ResultTarget{
		URL:  validURL,
		Name: name,
		Type: "gcs",
	}
}

func TestPipelineParamsSpec_Validate(t *testing.T) {
	type fields struct {
		ServiceAccount string
		Runs           ResultTarget
		Logs           ResultTarget
		Tests          *ResultTarget
		Clusters       []Cluster
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "valid service account and results",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs:           validResultTarget("runs"),
				Logs:           validResultTarget("logs"),
			},
		},
		{
			name: "valid with tests",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs:           validResultTarget("runs"),
				Logs:           validResultTarget("logs"),
				Tests: &ResultTarget{
					URL:  validURL,
					Name: "tests",
					Type: "gcs",
				},
			},
		},
		{
			name: "valid with clusters",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs:           validResultTarget("runs"),
				Logs:           validResultTarget("logs"),
				Clusters: []Cluster{
					Cluster{
						Endpoint: validURL,
						Name:     "cluster",
						Type:     "gke",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &PipelineParamsSpec{
				ServiceAccount: tt.fields.ServiceAccount,
				Results: Results{
					Runs:  tt.fields.Runs,
					Logs:  tt.fields.Logs,
					Tests: tt.fields.Tests,
				},
			}
			if err := ts.Validate(); err != nil {
				t.Errorf("PiplineParamsSpec.Validate() = %v", err)
			}
		})
	}
}

func TestPipelineParamsSpec_ValidateError(t *testing.T) {
	type fields struct {
		ServiceAccount string
		Runs           ResultTarget
		Logs           ResultTarget
		Tests          *ResultTarget
		Clusters       []Cluster
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "nil",
		},
		{
			name: "no service account",
			fields: fields{
				ServiceAccount: "",
			},
		},
		{
			name: "invalid results.runs.URL",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs: ResultTarget{
					Name: "runs",
					URL:  "",
					Type: "gcs",
				},
			},
		},
		{
			name: "invalid results.runs.type",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs: ResultTarget{
					Name: "runs",
					URL:  validURL,
					Type: "invalid",
				},
			},
		},
		{
			name: "invalid results.logs.URL",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs:           validResultTarget("runs"),
				Logs: ResultTarget{
					Name: "logs",
					URL:  "",
					Type: "gcs",
				},
			},
		},
		{
			name: "invalid results.logs.type",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs:           validResultTarget("runs"),
				Logs: ResultTarget{
					Name: "logs",
					URL:  validURL,
					Type: "invalid",
				},
			},
		},
		{
			name: "invalid results.tests.URL",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs:           validResultTarget("runs"),
				Logs:           validResultTarget("logs"),
				Tests: &ResultTarget{
					Name: "tests",
					URL:  "",
					Type: "gcs",
				},
			},
		},
		{
			name: "invalid results.tests.type",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs:           validResultTarget("runs"),
				Logs:           validResultTarget("logs"),
				Tests: &ResultTarget{
					Name: "tests",
					URL:  validURL,
					Type: "invalid",
				},
			},
		},
		{
			name: "invalid cluster.type",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs:           validResultTarget("runs"),
				Logs:           validResultTarget("logs"),
				Clusters: []Cluster{
					Cluster{
						Name:     "cluster",
						Endpoint: validURL,
						Type:     "invalid",
					},
				},
			},
		},
		{
			name: "invalid cluster.endpoint",
			fields: fields{
				ServiceAccount: validServiceAccount,
				Runs:           validResultTarget("runs"),
				Logs:           validResultTarget("logs"),
				Clusters: []Cluster{
					Cluster{
						Name:     "cluster",
						Endpoint: "",
						Type:     "gke",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PipelineParamsSpec{
				ServiceAccount: tt.fields.ServiceAccount,
				Clusters:       tt.fields.Clusters,
				Results: Results{
					Runs:  tt.fields.Runs,
					Logs:  tt.fields.Logs,
					Tests: tt.fields.Tests,
				},
			}
			if err := ps.Validate(); err == nil {
				t.Errorf("PipelineParamsSpec.Validate() did not return error.")
			}
		})
	}
}
