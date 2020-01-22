package resources

import (
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

// ResolveWorkspaces searches a PipelineSpec for workspaces with From clauses.
// When a From clause is found the func initiates a walk following From clauses
// until a concrete workspace name is found. The concrete workspace name is then
// used in place of the original clause. A modified copy of the provided
// PipelineSpec is returned.
//
// An error can be returned if a From clause points at a nonexistent task or
// workspace, or if following From clauses never reaches a concrete workspace name.
func ResolveWorkspaces(p *v1alpha1.PipelineSpec) (*v1alpha1.PipelineSpec, error) {
	// Take a DeepCopy because the spec is going to be modified.
	spec := p.DeepCopy()

	pts := make(map[string]*v1alpha1.PipelineTask)
	for i := range spec.Tasks {
		pts[spec.Tasks[i].Name] = &spec.Tasks[i]
	}

	for taskIdx := range spec.Tasks {
		pt := &spec.Tasks[taskIdx]
		for wsIdx := range pt.Workspaces {
			ws := &pt.Workspaces[wsIdx]
			resolvedWorkspaceName, err := getResolvedWorkspaceName(ws, pts)
			if err != nil {
				return nil, fmt.Errorf("error resolving from workspace %q in pipeline task %q: %w", ws.Name, pt.Name, err)
			}
			pt.Workspaces[wsIdx] = v1alpha1.PipelineTaskWorkspaceBinding{
				Name:      pt.Workspaces[wsIdx].Name,
				Workspace: resolvedWorkspaceName,
			}
		}
	}
	return spec, nil
}

// getResolvedWorkspaceName searches for a concrete workspace name starting at a
// given workspace. It follows From clauses until it reaches a literal name or
// hits a limit on the number of search iterations.
func getResolvedWorkspaceName(ws *v1alpha1.PipelineTaskWorkspaceBinding, pts map[string]*v1alpha1.PipelineTask) (string, error) {
	for i := 0; i < len(pts); i++ {
		if ws.Workspace != "" {
			return ws.Workspace, nil
		}
		nextWorkspace, err := getNextWorkspace(ws, pts)
		switch {
		case err != nil:
			return "", err
		case nextWorkspace != nil:
			ws = nextWorkspace
		default:
			// this shouldn't be reachable - a missing workspace has been arrived at but no error
			// returned by `getNextWorkspace`
			break
		}
	}
	return "", fmt.Errorf("does not resolve to a workspace named in the pipeline")
}

// getNextWorkspace looks up the pipeline task from a given workspace's From clause and
// and then returns the workspace that the From clause points at.
func getNextWorkspace(ws *v1alpha1.PipelineTaskWorkspaceBinding, pts map[string]*v1alpha1.PipelineTask) (*v1alpha1.PipelineTaskWorkspaceBinding, error) {
	if ws.From.Task != "" {
		if nextTask, ok := pts[ws.From.Task]; ok {
			for wsi := range nextTask.Workspaces {
				if nextTask.Workspaces[wsi].Name == ws.From.Name {
					return &nextTask.Workspaces[wsi], nil
				}
			}
			return nil, fmt.Errorf("pipeline task %q has no workspace named %q", ws.From.Task, ws.From.Name)
		}
		return nil, fmt.Errorf("pipeline task %q does not exist", ws.From.Task)
	}
	return nil, fmt.Errorf("missing from clause")
}
