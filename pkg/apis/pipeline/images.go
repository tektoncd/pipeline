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

package pipeline

// Images holds the images reference for a number of container images used accross tektoncd pipelines
type Images struct {
	// EntryPointImage is container image containing our entrypoint binary.
	EntryPointImage string
	// NopImage is the container image used to kill sidecars
	NopImage string
	// GitImage is the container image with Git that we use to implement the Git source step.
	GitImage string
}
