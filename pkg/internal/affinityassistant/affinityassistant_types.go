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

package affinityassistant

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/pod"

	"github.com/tektoncd/pipeline/pkg/apis/config"
)

type AffinityAssistantBehavior string

const (
	AffinityAssistantDisabled                    = AffinityAssistantBehavior("AffinityAssistantDisabled")
	AffinityAssistantPerWorkspace                = AffinityAssistantBehavior("AffinityAssistantPerWorkspace")
	AffinityAssistantPerPipelineRun              = AffinityAssistantBehavior("AffinityAssistantPerPipelineRun")
	AffinityAssistantPerPipelineRunWithIsolation = AffinityAssistantBehavior("AffinityAssistantPerPipelineRunWithIsolation")
)

// GetAffinityAssistantBehavior returns an AffinityAssistantBehavior based on the "coschedule" feature flags
func GetAffinityAssistantBehavior(ctx context.Context) (AffinityAssistantBehavior, error) {
	cfg := config.FromContextOrDefaults(ctx)
	coschedule := cfg.FeatureFlags.Coschedule

	switch coschedule {
	case config.CoschedulePipelineRuns:
		return AffinityAssistantPerPipelineRun, nil
	case config.CoscheduleIsolatePipelineRun:
		return AffinityAssistantPerPipelineRunWithIsolation, nil
	case config.CoscheduleWorkspaces:
		return AffinityAssistantPerWorkspace, nil
	case config.CoscheduleDisabled:
		return AffinityAssistantDisabled, nil
	}

	return "", fmt.Errorf("unknown affinity assistant coschedule: %v", coschedule)
}

// ContainerConfig defines AffinityAssistant container configuration
type ContainerConfig struct {
	Image                 string
	SecurityContextConfig pod.SecurityContextConfig
}
