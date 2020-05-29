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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// WorkspaceDeclaration is a declaration of a volume that a Task requires.
type WorkspaceDeclaration = v1beta1.WorkspaceDeclaration

// WorkspaceBinding maps a Task's declared workspace to a Volume.
type WorkspaceBinding = v1beta1.WorkspaceBinding

// PipelineWorkspaceDeclaration creates a named slot in a Pipeline that a PipelineRun
// is expected to populate with a workspace binding.
type PipelineWorkspaceDeclaration = v1beta1.PipelineWorkspaceDeclaration

// WorkspacePipelineTaskBinding describes how a workspace passed into the pipeline should be
// mapped to a task's declared workspace.
type WorkspacePipelineTaskBinding = v1beta1.WorkspacePipelineTaskBinding
