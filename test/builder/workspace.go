package builder

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// TaskWorkspace adds a workspace declaration.
func TaskWorkspace(name, desc, mountPath string, readOnly bool) TaskSpecOp {
	return func(spec *v1alpha1.TaskSpec) {
		spec.Workspaces = append(spec.Workspaces, v1alpha1.WorkspaceDeclaration{
			Name:        name,
			Description: desc,
			MountPath:   mountPath,
			ReadOnly:    readOnly,
		})
	}
}

// TaskRunWorkspaceEmptyDir adds a workspace binding to an empty dir volume source.
func TaskRunWorkspaceEmptyDir(name, subPath string) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		spec.Workspaces = append(spec.Workspaces, v1alpha1.WorkspaceBinding{
			Name:     name,
			SubPath:  subPath,
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		})
	}
}

// TaskRunWorkspacePVC adds a workspace binding to a PVC volume source.
func TaskRunWorkspacePVC(name, subPath, claimName string) TaskRunSpecOp {
	return func(spec *v1alpha1.TaskRunSpec) {
		spec.Workspaces = append(spec.Workspaces, v1alpha1.WorkspaceBinding{
			Name:    name,
			SubPath: subPath,
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: claimName,
			},
		})
	}
}

// PipelineWorkspaceDeclaration adds a Workspace to the workspaces listed in the pipeline spec.
func PipelineWorkspaceDeclaration(names ...string) PipelineSpecOp {
	return func(spec *v1alpha1.PipelineSpec) {
		for _, name := range names {
			spec.Workspaces = append(spec.Workspaces, v1alpha1.PipelineWorkspaceDeclaration{Name: name})
		}
	}
}

// PipelineRunWorkspaceBindingEmptyDir adds an EmptyDir Workspace to the workspaces of a pipelinerun spec.
func PipelineRunWorkspaceBindingEmptyDir(name string) PipelineRunSpecOp {
	return func(spec *v1alpha1.PipelineRunSpec) {
		spec.Workspaces = append(spec.Workspaces, v1alpha1.WorkspaceBinding{
			Name:     name,
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		})
	}
}

// PipelineTaskWorkspaceBindingOp is an operation which modify a PipelineTaskWorkspaceBinding struct.
type PipelineTaskWorkspaceBindingOp func(*v1alpha1.PipelineTaskWorkspaceBinding)

// PipelineTaskWorkspaceBinding adds a workspace binding to a pipeline task
func PipelineTaskWorkspaceBinding(name string, ops ...PipelineTaskWorkspaceBindingOp) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		ws := &v1alpha1.PipelineTaskWorkspaceBinding{
			Name: name,
		}
		for _, op := range ops {
			op(ws)
		}
		pt.Workspaces = append(pt.Workspaces, *ws)
	}
}

// PipelineTaskWorkspaceBindingWorkspace adds a workspace name to a PipelineTaskWorkspaceBinding
func PipelineTaskWorkspaceBindingWorkspace(workspace string) PipelineTaskWorkspaceBindingOp {
	return func(wb *v1alpha1.PipelineTaskWorkspaceBinding) {
		wb.Workspace = workspace
	}
}

// PipelineTaskWorkspaceBindingFrom adds a from clause to a PipelineTaskWorkspaceBinding
func PipelineTaskWorkspaceBindingFrom(task, name string) PipelineTaskWorkspaceBindingOp {
	return func(wb *v1alpha1.PipelineTaskWorkspaceBinding) {
		wb.From.Task = task
		wb.From.Name = name
	}
}
