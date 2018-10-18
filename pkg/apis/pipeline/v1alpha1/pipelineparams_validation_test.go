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

func validResultTarget(name string) ResultTarget {
	return ResultTarget{
		URL:  validURL,
		Name: name,
		Type: "gcs",
	}
}

func TestClusterValidation(t *testing.T) {
	tests := []struct {
		name      string
		Clusters  []Cluster
		shouldErr bool
	}{
		{
			name: "valid cluster",
			Clusters: []Cluster{
				Cluster{
					Name: "cluster",
					Type: "gke",
				},
			},
		},
		{
			name: "valid cluster with endpoint",
			Clusters: []Cluster{
				Cluster{
					Name:     "cluster",
					Endpoint: validURL,
					Type:     "gke",
				},
			},
		},
		{
			name: "invalid cluster.type",
			Clusters: []Cluster{
				Cluster{
					Name:     "cluster",
					Endpoint: validURL,
					Type:     "invalid",
				},
			},
			shouldErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PipelineParamsSpec{
				Clusters: tt.Clusters,
				Results: Results{
					Runs: validResultTarget("runs"),
					Logs: validResultTarget("logs"),
				},
			}
			err := ps.Validate()
			if err == nil && tt.shouldErr {
				t.Errorf("PipelineParamsSpec.Validate() did not return error.")
			}
			if err != nil && !tt.shouldErr {
				t.Errorf("PipelineParamsSpec.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestResultsValidation(t *testing.T) {
	type fields struct {
		Runs      ResultTarget
		Logs      ResultTarget
		Tests     *ResultTarget
		shouldErr bool
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "valid results",
			fields: fields{
				Runs: validResultTarget("runs"),
				Logs: validResultTarget("logs"),
			},
		},
		{
			name: "invalid results.runs.URL",
			fields: fields{
				Runs: ResultTarget{
					Name: "runs",
					URL:  "",
					Type: "gcs",
				},
				shouldErr: true,
			},
		},
		{
			name: "invalid results.runs.type",
			fields: fields{
				Runs: ResultTarget{
					Name: "runs",
					URL:  validURL,
					Type: "invalid",
				},
				shouldErr: true,
			},
		},
		{
			name: "invalid results.logs.type",
			fields: fields{
				Runs: validResultTarget("runs"),
				Logs: ResultTarget{
					Name: "logs",
					URL:  validURL,
					Type: "invalid",
				},
				shouldErr: true,
			},
		},
		{
			name: "invalid results.tests.type",
			fields: fields{
				Runs: validResultTarget("runs"),
				Logs: validResultTarget("logs"),
				Tests: &ResultTarget{
					Name: "tests",
					URL:  validURL,
					Type: "invalid",
				},
				shouldErr: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PipelineParamsSpec{
				Results: Results{
					Runs:  tt.fields.Runs,
					Logs:  tt.fields.Logs,
					Tests: tt.fields.Tests,
				},
			}
			err := ps.Validate()
			if err == nil && tt.fields.shouldErr {
				t.Errorf("PipelineParamsSpec.Validate() did not return error.")
			}
			if err != nil && !tt.fields.shouldErr {
				t.Errorf("PipelineParamsSpec.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
