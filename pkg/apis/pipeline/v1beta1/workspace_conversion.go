/*
Copyright 2023 The Tekton Authors

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

package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (w WorkspaceDeclaration) convertTo(ctx context.Context, sink *v1.WorkspaceDeclaration) {
	sink.Name = w.Name
	sink.Description = w.Description
	sink.MountPath = w.MountPath
	sink.ReadOnly = w.ReadOnly
	sink.Optional = w.Optional
}

func (w *WorkspaceDeclaration) convertFrom(ctx context.Context, source v1.WorkspaceDeclaration) {
	w.Name = source.Name
	w.Description = source.Description
	w.MountPath = source.MountPath
	w.ReadOnly = source.ReadOnly
	w.Optional = source.Optional
}

func (w WorkspaceUsage) convertTo(ctx context.Context, sink *v1.WorkspaceUsage) {
	sink.Name = w.Name
	sink.MountPath = w.MountPath
}

func (w *WorkspaceUsage) convertFrom(ctx context.Context, source v1.WorkspaceUsage) {
	w.Name = source.Name
	w.MountPath = source.MountPath
}

func (w PipelineWorkspaceDeclaration) convertTo(ctx context.Context, sink *v1.PipelineWorkspaceDeclaration) {
	sink.Name = w.Name
	sink.Description = w.Description
	sink.Optional = w.Optional
}

func (w *PipelineWorkspaceDeclaration) convertFrom(ctx context.Context, source v1.PipelineWorkspaceDeclaration) {
	w.Name = source.Name
	w.Description = source.Description
	w.Optional = source.Optional
}

func (w WorkspacePipelineTaskBinding) convertTo(ctx context.Context, sink *v1.WorkspacePipelineTaskBinding) {
	sink.Name = w.Name
	sink.Workspace = w.Workspace
	sink.SubPath = w.SubPath
}

func (w *WorkspacePipelineTaskBinding) convertFrom(ctx context.Context, source v1.WorkspacePipelineTaskBinding) {
	w.Name = source.Name
	w.Workspace = source.Workspace
	w.SubPath = source.SubPath
}

func (w WorkspaceBinding) convertTo(ctx context.Context, sink *v1.WorkspaceBinding) {
	sink.Name = w.Name
	sink.SubPath = w.SubPath
	sink.VolumeClaimTemplate = w.VolumeClaimTemplate
	sink.PersistentVolumeClaim = w.PersistentVolumeClaim
	sink.EmptyDir = w.EmptyDir
	sink.ConfigMap = w.ConfigMap
	sink.Secret = w.Secret
	sink.Projected = w.Projected
	sink.CSI = w.CSI
}

// ConvertFrom converts v1beta1 Param from v1 Param
func (w *WorkspaceBinding) ConvertFrom(ctx context.Context, source v1.WorkspaceBinding) {
	w.Name = source.Name
	w.SubPath = source.SubPath
	w.VolumeClaimTemplate = source.VolumeClaimTemplate
	w.PersistentVolumeClaim = source.PersistentVolumeClaim
	w.EmptyDir = source.EmptyDir
	w.ConfigMap = source.ConfigMap
	w.Secret = source.Secret
	w.Projected = source.Projected
	w.CSI = source.CSI
}
