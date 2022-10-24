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

package pod

import (
	"path/filepath"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// workingDirInit returns a Container that should be run as an init
// container to ensure that all steps' workingDirs relative to the workspace
// exist.
//
// If no such directories need to be created (i.e., no relative workingDirs
// are specified), this method returns nil, as no init container is necessary.
func workingDirInit(workingdirinitImage string, stepContainers []corev1.Container) *corev1.Container {
	// Gather all unique workingDirs.
	workingDirs := sets.NewString()
	for _, step := range stepContainers {
		if step.WorkingDir != "" {
			workingDirs.Insert(step.WorkingDir)
		}
	}
	if workingDirs.Len() == 0 {
		return nil
	}

	// Clean and append each relative workingDir.
	var relativeDirs []string
	for _, wd := range workingDirs.List() {
		p := filepath.Clean(wd)
		if !filepath.IsAbs(p) || strings.HasPrefix(p, "/workspace/") {
			relativeDirs = append(relativeDirs, p)
		}
	}

	if len(relativeDirs) == 0 {
		// There are no workingDirs to initialize.
		return nil
	}

	return &corev1.Container{
		Name:         "working-dir-initializer",
		Image:        workingdirinitImage,
		Command:      []string{"/ko-app/workingdirinit"},
		Args:         relativeDirs,
		WorkingDir:   pipeline.WorkspaceDir,
		VolumeMounts: implicitVolumeMounts,
	}
}
