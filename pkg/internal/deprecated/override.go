/*
Copyright 2021 The Tekton Authors

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

package deprecated

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/pod"
	corev1 "k8s.io/api/core/v1"
)

// NewOverrideWorkingDirTransformer returns a pod.Transformer that will override the workingDir on pods if needed.
func NewOverrideWorkingDirTransformer(ctx context.Context) pod.Transformer {
	return func(p *corev1.Pod) (*corev1.Pod, error) {
		if shouldOverrideWorkingDir(ctx) {
			for i, c := range p.Spec.Containers {
				if pod.IsContainerStep(c.Name) {
					if c.WorkingDir == "" {
						p.Spec.Containers[i].WorkingDir = pipeline.WorkspaceDir
					}
				}
			}
		}
		return p, nil
	}
}

// shouldOverrideWorkingDir returns a bool indicating whether a Pod should have its
// working directory overwritten with /workspace or if it should be
// left unmodified.
//
// For further reference see https://github.com/tektoncd/pipeline/issues/1836
func shouldOverrideWorkingDir(ctx context.Context) bool {
	cfg := config.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.DisableWorkingDirOverwrite
}

// NewOverrideHomeTransformer returns a pod.Transformer that will override HOME if needed
func NewOverrideHomeTransformer(ctx context.Context) pod.Transformer {
	return func(p *corev1.Pod) (*corev1.Pod, error) {
		if shouldOverrideHomeEnv(ctx) {
			for i, c := range p.Spec.Containers {
				hasHomeEnv := false
				for _, e := range c.Env {
					if e.Name == "HOME" {
						hasHomeEnv = true
					}
				}
				if !hasHomeEnv {
					p.Spec.Containers[i].Env = append(p.Spec.Containers[i].Env, corev1.EnvVar{
						Name:  "HOME",
						Value: pipeline.HomeDir,
					})
				}
			}
		}
		return p, nil
	}
}

// shouldOverrideHomeEnv returns a bool indicating whether a Pod should have its
// $HOME environment variable overwritten with /tekton/home or if it should be
// left unmodified.
//
// For further reference see https://github.com/tektoncd/pipeline/issues/2013
func shouldOverrideHomeEnv(ctx context.Context) bool {
	cfg := config.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.DisableHomeEnvOverwrite
}
