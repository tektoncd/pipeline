/*
Copyright 2020 The Tekton Authors

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

package workspace

const (
	// LabelInstance is used in combination with LabelComponent to configure PodAffinity for TaskRun pods
	LabelInstance = "app.kubernetes.io/instance"

	// LabelComponent is used to configure PodAntiAffinity to other Affinity Assistants
	LabelComponent = "app.kubernetes.io/component"
	// ComponentNameAffinityAssistant is the component name for an Affinity Assistant
	ComponentNameAffinityAssistant = "affinity-assistant"

	// AnnotationAffinityAssistantName is used to pass the instance name of an Affinity Assistant to TaskRun pods
	AnnotationAffinityAssistantName = "pipeline.tekton.dev/affinity-assistant"
)
