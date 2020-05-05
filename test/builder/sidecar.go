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

package builder

import (
	v1alpha1 "github.com/tektoncd/pipeline/internal/builder/v1alpha1"
)

var (
	// SidecarStateName sets the name of the Sidecar for the SidecarState.
	// Deprecated: moved to internal/builder/v1alpha1
	SidecarStateName = v1alpha1.SidecarStateName

	// SidecarStateImageID sets ImageID of Sidecar for SidecarState.
	// Deprecated: moved to internal/builder/v1alpha1
	SidecarStateImageID = v1alpha1.SidecarStateImageID

	// SidecarStateContainerName sets ContainerName of Sidecar for SidecarState.
	// Deprecated: moved to internal/builder/v1alpha1
	SidecarStateContainerName = v1alpha1.SidecarStateContainerName

	// SetSidecarStateTerminated sets Terminated state of a Sidecar.
	// Deprecated: moved to internal/builder/v1alpha1
	SetSidecarStateTerminated = v1alpha1.SetSidecarStateTerminated

	// SetSidecarStateRunning sets Running state of a Sidecar.
	// Deprecated: moved to internal/builder/v1alpha1
	SetSidecarStateRunning = v1alpha1.SetSidecarStateRunning

	// SetSidecarStateWaiting sets Waiting state of a Sidecar.
	// Deprecated: moved to internal/builder/v1alpha1
	SetSidecarStateWaiting = v1alpha1.SetSidecarStateWaiting
)
