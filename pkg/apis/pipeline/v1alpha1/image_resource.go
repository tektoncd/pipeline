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

package v1alpha1

import (
	"encoding/json"
	"strings"

	"golang.org/x/xerrors"
)

// NewImageResource creates a new ImageResource from a PipelineResource.
func NewImageResource(r *PipelineResource) (*ImageResource, error) {
	if r.Spec.Type != PipelineResourceTypeImage {
		return nil, xerrors.Errorf("ImageResource: Cannot create an Image resource from a %s Pipeline Resource", r.Spec.Type)
	}
	ir := &ImageResource{
		Name: r.Name,
		Type: PipelineResourceTypeImage,
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

// ImageResource defines an endpoint where artifacts can be stored, such as images.
type ImageResource struct {
	Name           string               `json:"name"`
	Type           PipelineResourceType `json:"type"`
	URL            string               `json:"url"`
	Digest         string               `json:"digest"`
	OutputImageDir string
}

// GetName returns the name of the resource
func (s ImageResource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "image"
func (s ImageResource) GetType() PipelineResourceType {
	return PipelineResourceTypeImage
}

// Replacements is used for template replacement on an ImageResource inside of a Taskrun.
func (s *ImageResource) Replacements() map[string]string {
	return map[string]string{
		"name":   s.Name,
		"type":   string(s.Type),
		"url":    s.URL,
		"digest": s.Digest,
	}
}

// GetInputTaskModifier returns the TaskModifier to be used when this resource is an input.
func (s *ImageResource) GetInputTaskModifier(_ *TaskSpec, _ string) (TaskModifier, error) {
	return &InternalTaskModifier{}, nil
}

// GetOutputTaskModifier returns a No-op TaskModifier.
func (s *ImageResource) GetOutputTaskModifier(_ *TaskSpec, _ string) (TaskModifier, error) {
	return &InternalTaskModifier{}, nil
}

func (s ImageResource) String() string {
	// the String() func implements the Stringer interface, and therefore
	// cannot return an error
	// if the Marshal func gives an error, the returned string will be empty
	json, _ := json.Marshal(s)
	return string(json)
}
