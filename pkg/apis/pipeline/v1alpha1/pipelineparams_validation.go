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
	"fmt"
	"net/url"

	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (p *PipelineParams) Validate() *apis.FieldError {
	if err := validateObjectMetadata(p.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	return nil
}

func (ps *PipelineParamsSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(ps, &PipelineParams{}) {
		return apis.ErrMissingField(apis.CurrentField)
	}
	// PipelineParams must specify a service account to run under
	if ps.ServiceAccount == "" {
		return apis.ErrMissingField("pipelineparamsspec.ServiceAccount")
	}
	for _, c := range ps.Clusters {
		// If Clusters are specified, their URLs should be valid
		if err := validateURL(c.Endpoint, fmt.Sprintf("pipelineparamsspec.Clusters.%s.Endpoint", c.Name)); err != nil {
			return err
		}
		// And the cluster type should be valid
		if err := validateClusterType(c.Type, fmt.Sprintf("pipelineparamsspec.Clusters.%s.Type", c.Name)); err != nil {
			return err
		}
	}

	// PipelineParams must have valid Results
	// Results.Runs should have a valid URL and ResultTargetType
	if err := validateURL(ps.Results.Runs.URL, "pipelineparamsspec.Results.Runs.URL"); err != nil {
		return err
	}
	if err := validateResultTargetType(ps.Results.Runs.Type, "pipelineparamsspec.Results.Runs.Type"); err != nil {
		return err
	}

	// Results.Logs should have a valid URL and ResultTargetType
	if err := validateURL(ps.Results.Logs.URL, "pipelineparamsspec.Results.Logs.URL"); err != nil {
		return err
	}
	if err := validateResultTargetType(ps.Results.Logs.Type, "pipelineparamsspec.Results.Logs.Type"); err != nil {
		return err
	}

	// If Results.Tests exists, it should have a valid URL and ResultTargetType
	if ps.Results.Tests != nil {
		if err := validateURL(ps.Results.Tests.URL, "pipelineparamsspec.Results.Tests.URL"); err != nil {
			return err
		}
		if err := validateResultTargetType(ps.Results.Tests.Type, "pipelineparamsspec.Results.Tests.Type"); err != nil {
			return err
		}
	}

	return nil
}

func validateURL(u, path string) *apis.FieldError {
	_, err := url.ParseRequestURI(u)
	if err != nil {
		return apis.ErrInvalidValue(u, path)
	}
	return nil
}

func validateClusterType(c ClusterType, path string) *apis.FieldError {
	for _, a := range AllClusterTypes {
		if a == c {
			return nil
		}
	}
	return apis.ErrInvalidValue(string(c), path)
}

func validateResultTargetType(r ResultTargetType, path string) *apis.FieldError {
	for _, a := range AllResultTargetTypes {
		if a == r {
			return nil
		}
	}
	return apis.ErrInvalidValue(string(r), path)
}
