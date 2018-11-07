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
	"strings"

	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (r *PipelineResource) Validate() *apis.FieldError {
	if err := validateObjectMetadata(r.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}

	if r.Spec.Type == PipelineResourceTypeCluster {
		var clusterNameFound, usernameFound, cadataFound bool
		for _, param := range r.Spec.Params {
			switch {
			case strings.EqualFold(param.Name, "URL"):
				if err := validateURL(param.Value, param.Value); err != nil {
					return err
				}
			case strings.EqualFold(param.Name, "clusterName"):
				clusterNameFound = true
			case strings.EqualFold(param.Name, "Username"):
				usernameFound = true
			case strings.EqualFold(param.Name, "CAData"):
				cadataFound = true
			}
		}

		if !clusterNameFound {
			return apis.ErrMissingField("clusterName param")
		}
		if !usernameFound {
			return apis.ErrMissingField("username param")
		}
		if !cadataFound {
			return apis.ErrMissingField("CAData param")
		}
	}
	return nil
}

func (rs *PipelineResourceSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(rs, &PipelineResourceSpec{}) {
		return apis.ErrMissingField(apis.CurrentField)
	}

	return nil
}
