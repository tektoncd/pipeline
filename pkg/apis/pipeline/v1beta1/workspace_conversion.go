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
