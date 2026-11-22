/*
Copyright 2026 The Tekton Authors

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

package config_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
)

func TestPipelineRunOutcomesFeatureFlag(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    map[string]string
		want    bool
		wantErr bool
	}{{
		name: "disabled by default",
	}, {
		name: "global alpha does not enable outcomes",
		data: map[string]string{"enable-api-fields": "alpha"},
	}, {
		name: "explicit enable with stable API",
		data: map[string]string{"enable-api-fields": "stable", config.EnablePipelineRunOutcomes: "true"},
		want: true,
	}, {
		name: "explicit disable with alpha API",
		data: map[string]string{"enable-api-fields": "alpha", config.EnablePipelineRunOutcomes: "false"},
	}, {
		name:    "invalid boolean",
		data:    map[string]string{config.EnablePipelineRunOutcomes: "invalid"},
		wantErr: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := config.NewFeatureFlagsFromMap(tc.data)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewFeatureFlagsFromMap() error = %v, want error %v", err, tc.wantErr)
			}
			if err == nil && got.EnablePipelineRunOutcomes != tc.want {
				t.Errorf("EnablePipelineRunOutcomes = %v, want %v", got.EnablePipelineRunOutcomes, tc.want)
			}
		})
	}
}
