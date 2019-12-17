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

package contexts

import (
	"context"
	"testing"
)

func TestContexts(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		ctx   context.Context
		check func(context.Context) bool
		want  bool
	}{{
		name:  "has default config name",
		ctx:   WithDefaultConfigurationName(ctx),
		check: HasDefaultConfigurationName,
		want:  true,
	}, {
		name:  "doesn't have default config name",
		ctx:   ctx,
		check: HasDefaultConfigurationName,
		want:  false,
	}, {
		name:  "are upgrading via defaulting",
		ctx:   WithUpgradeViaDefaulting(ctx),
		check: IsUpgradeViaDefaulting,
		want:  true,
	}, {
		name:  "aren't upgrading via defaulting",
		ctx:   ctx,
		check: IsUpgradeViaDefaulting,
		want:  false,
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.check(tc.ctx)
			if tc.want != got {
				t.Errorf("check() = %v, wanted %v", tc.want, got)
			}
		})
	}
}
