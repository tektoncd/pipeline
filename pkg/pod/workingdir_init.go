package pod

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const (
	workspaceDir   = "/workspace"
	workingDirInit = "working-dir-initializer"
)

// WorkingDirInit returns a Container that should be run as an init
// container to ensure that all steps' workingDirs relative to the workspace
// exist.
//
// If no such directories need to be created (i.e., no relative workingDirs
// are specified), this method returns nil, as no init container is necessary.
//
// TODO(jasonhall): This should take []corev1.Container instead of
// []corev1.Step, but this makes it easier to use in pod.go. When pod.go is
// cleaned up, this can take []corev1.Container.
func WorkingDirInit(shellImage string, steps []v1alpha1.Step) *corev1.Container {
	// Gather all unique workingDirs.
	workingDirs := map[string]struct{}{}
	for _, step := range steps {
		if step.WorkingDir != "" {
			workingDirs[step.WorkingDir] = struct{}{}
		}
	}
	if len(workingDirs) == 0 {
		return nil
	}

	// Sort unique workingDirs.
	var orderedDirs []string
	for wd := range workingDirs {
		orderedDirs = append(orderedDirs, wd)
	}
	sort.Strings(orderedDirs)

	// Clean and append each relative workingDir.
	var relativeDirs []string
	for _, wd := range orderedDirs {
		p := filepath.Clean(wd)
		if !filepath.IsAbs(p) || strings.HasPrefix(p, "/workspace/") {
			relativeDirs = append(relativeDirs, p)
		}
	}

	if len(relativeDirs) == 0 {
		return nil
	}

	return &corev1.Container{
		Name:       names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(workingDirInit),
		Image:      shellImage,
		Command:    []string{"sh"},
		Args:       []string{"-c", "mkdir -p " + strings.Join(relativeDirs, " ")},
		WorkingDir: workspaceDir,
	}
}
