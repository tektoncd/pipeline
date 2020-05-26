/*
Copyright 2019-2020 The Tekton Authors

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

package image

import (
	"encoding/json"
	"fmt"
	"strings"

	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
)

// Resource defines an endpoint where artifacts can be stored, such as images.
type Resource struct {
	Name           string                                `json:"name"`
	Type           resourcev1alpha1.PipelineResourceType `json:"type"`
	URL            string                                `json:"url"`
	Digest         string                                `json:"digest"`
	OutputImageDir string
}

// NewResource creates a new ImageResource from a PipelineResourcev1alpha1.
func NewResource(name string, r *resourcev1alpha1.PipelineResource) (*Resource, error) {
	if r.Spec.Type != resourcev1alpha1.PipelineResourceTypeImage {
		return nil, fmt.Errorf("ImageResource: Cannot create an Image resource from a %s Pipeline Resource", r.Spec.Type)
	}
	ir := &Resource{
		Name: name,
		Type: resourcev1alpha1.PipelineResourceTypeImage,
	}

	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "URL"):
			ir.URL = param.Value
		case strings.EqualFold(param.Name, "Digest"):
			ir.Digest = param.Value
		}
	}

	return ir, nil
}

// GetName returns the name of the resource
func (s Resource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "image"
func (s Resource) GetType() resourcev1alpha1.PipelineResourceType {
	return resourcev1alpha1.PipelineResourceTypeImage
}

// Replacements is used for template replacement on an ImageResource inside of a Taskrun.
func (s *Resource) Replacements() map[string]string {
	return map[string]string{
		"name":   s.Name,
		"type":   s.Type,
		"url":    s.URL,
		"digest": s.Digest,
	}
}

// GetInputTaskModifier returns the TaskModifier to be used when this resource is an input.
func (s *Resource) GetInputTaskModifier(_ *pipelinev1beta1.TaskSpec, _ string) (pipelinev1beta1.TaskModifier, error) {
	return &pipelinev1beta1.InternalTaskModifier{}, nil
}

// GetOutputTaskModifier returns a No-op TaskModifier.
func (s *Resource) GetOutputTaskModifier(_ *pipelinev1beta1.TaskSpec, _ string) (pipelinev1beta1.TaskModifier, error) {
	return &pipelinev1beta1.InternalTaskModifier{}, nil
}

func (s Resource) String() string {
	// the String() func implements the Stringer interface, and therefore
	// cannot return an error
	// if the Marshal func gives an error, the returned string will be empty
	json, _ := json.Marshal(s)
	return string(json)
}
