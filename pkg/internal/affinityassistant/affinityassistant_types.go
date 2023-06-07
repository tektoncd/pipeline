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

	"github.com/tektoncd/pipeline/pkg/apis/config"
)

type AffinityAssitantBehavior string

const (
	AffinityAssistantDisabled                    = AffinityAssitantBehavior("AffinityAssistantDisabled")
	AffinityAssistantPerWorkspace                = AffinityAssitantBehavior("AffinityAssistantPerWorkspace")
	AffinityAssistantPerPipelineRun              = AffinityAssitantBehavior("AffinityAssistantPerPipelineRun")
	AffinityAssistantPerPipelineRunWithIsolation = AffinityAssitantBehavior("AffinityAssistantPerPipelineRunWithIsolation")
)

// GetAffinityAssistantBehavior returns an AffinityAssitantBehavior based on the
// combination of "disable-affinity-assistant" and "coschedule" feature flags
// TODO(#6740)(WIP): consume this function in the PipelineRun reconciler to determine Affinity Assistant behavior.
func GetAffinityAssistantBehavior(ctx context.Context) (AffinityAssitantBehavior, error) {
	cfg := config.FromContextOrDefaults(ctx)
	disableAA := cfg.FeatureFlags.DisableAffinityAssistant
	coschedule := cfg.FeatureFlags.Coschedule

	// at this point, we have validated that "coschedule" can only be "workspaces"
	// when "disable-affinity-assistant" is false
	if !disableAA {
		return AffinityAssistantPerWorkspace, nil
	}

	switch coschedule {
	case config.CoschedulePipelineRuns:
		return AffinityAssistantPerPipelineRun, nil
	case config.CoscheduleIsolatePipelineRun:
		return AffinityAssistantPerPipelineRunWithIsolation, nil
	case config.CoscheduleDisabled, config.CoscheduleWorkspaces:
		return AffinityAssistantDisabled, nil
	}

	return "", fmt.Errorf("unknown combination of disable-affinity-assistant: %v and coschedule: %v", disableAA, coschedule)
}
