/*
Copyright 2019 The Tekton Authors.

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
	"strings"
)

// CloudEventResource is an event sink to which events are delivered when a TaskRun has finished
type CloudEventResource struct {
	// Name is the name used to reference to the PipelineResource
	Name string `json:"name"`
	// Type must be `PipelineResourceTypeCloudEvent`
	Type PipelineResourceType `json:"type"`
	// TargetURI is the URI of the sink which the cloud event is develired to
	TargetURI string `json:"targetURI"`
}

// NewCloudEventResource creates a new CloudEvent resource to pass to a Task
func NewCloudEventResource(r *PipelineResource) (*CloudEventResource, error) {
	if r.Spec.Type != PipelineResourceTypeCloudEvent {
		return nil, fmt.Errorf("CloudEventResource: Cannot create a Cloud Event resource from a %s Pipeline Resource", r.Spec.Type)
	}
	var targetURI string
	var targetURISpecified bool

	for _, param := range r.Spec.Params {
		if strings.EqualFold(param.Name, "TargetURI") {
			targetURI = param.Value
			if param.Value != "" {
				targetURISpecified = true
			}
		}
	}

	if !targetURISpecified {
		return nil, fmt.Errorf("CloudEventResource: Need URI to be specified in order to create a CloudEvent resource %s", r.Name)
	}
	return &CloudEventResource{
		Name:      r.Name,
		Type:      r.Spec.Type,
		TargetURI: targetURI,
	}, nil
}

// GetName returns the name of the resource
func (s CloudEventResource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "cloudEvent"
func (s CloudEventResource) GetType() PipelineResourceType {
	return PipelineResourceTypeCloudEvent
}

// Replacements is used for template replacement on an CloudEventResource inside of a Taskrun.
func (s *CloudEventResource) Replacements() map[string]string {
	return map[string]string{
		"name":       s.Name,
		"type":       string(s.Type),
		"target-uri": s.TargetURI,
	}
}

// GetInputTaskModifier returns the TaskModifier to be used when this resource is an input.
func (s *CloudEventResource) GetInputTaskModifier(_ *TaskSpec, _ string) (TaskModifier, error) {
	return &InternalTaskModifier{}, nil
}

// GetOutputTaskModifier returns a No-op TaskModifier.
func (s *CloudEventResource) GetOutputTaskModifier(_ *TaskSpec, _ string) (TaskModifier, error) {
	return &InternalTaskModifier{}, nil
}
