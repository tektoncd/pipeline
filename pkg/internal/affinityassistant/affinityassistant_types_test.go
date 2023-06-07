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
	"testing"

	"github.com/google/go-cmp/cmp"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	"github.com/tektoncd/pipeline/test/diff"

	"github.com/tektoncd/pipeline/pkg/apis/config"
)

func Test_GetAffinityAssistantBehavior(t *testing.T) {
	tcs := []struct {
		name      string
		configMap map[string]string
		expect    AffinityAssitantBehavior
	}{{
		name: "affinity-assistant-enabled",
		configMap: map[string]string{
			"disable-affinity-assistant": "false",
		},
		expect: AffinityAssistantPerWorkspace,
	}, {
		name: "affinity-assistant-disabled-coschedule-workspaces",
		configMap: map[string]string{
			"disable-affinity-assistant": "true",
			"coschedule":                 config.CoscheduleWorkspaces,
		},
		expect: AffinityAssistantDisabled,
	}, {
		name: "affinity-assistant-disabled-coschedule-pipelineruns",
		configMap: map[string]string{
			"disable-affinity-assistant": "true",
			"coschedule":                 config.CoschedulePipelineRuns,
		},
		expect: AffinityAssistantPerPipelineRun,
	}, {
		name: "affinity-assistant-disabled-coschedule-isolate-pipelinerun",
		configMap: map[string]string{
			"disable-affinity-assistant": "true",
			"coschedule":                 config.CoscheduleIsolatePipelineRun,
		},
		expect: AffinityAssistantPerPipelineRunWithIsolation,
	}, {
		name: "affinity-assistant-disabled-coschedule-disabled",
		configMap: map[string]string{
			"disable-affinity-assistant": "true",
			"coschedule":                 config.CoscheduleDisabled,
		},
		expect: AffinityAssistantDisabled,
	}}

	for _, tc := range tcs {
		ctx := cfgtesting.SetFeatureFlags(context.Background(), t, tc.configMap)
		get, err := GetAffinityAssistantBehavior(ctx)
		if err != nil {
			t.Fatalf("unexpected error when getting affinity assistant behavior: %v", err)
		}

		if d := cmp.Diff(tc.expect, get); d != "" {
			t.Errorf("AffinityAssitantBehavior mismatch: %v", diff.PrintWantGot(d))
		}
	}
}
