/*
Copyright 2019 The Tekton Authors.

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

package config

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	logtesting "github.com/knative/pkg/logging/testing"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
)

func TestStoreLoadWithContext(t *testing.T) {
	store := NewStore(logtesting.TestLogger(t))
	defaultConfig := test.ConfigMapFromTestFile(t, "config-defaults")
	store.OnConfigChanged(defaultConfig)

	config := FromContext(store.ToContext(context.Background()))

	expected, _ := NewDefaultsFromConfigMap(defaultConfig)
	if diff := cmp.Diff(config.Defaults, expected); diff != "" {
		t.Errorf("Unexpected default config (-want, +got): %v", diff)
	}
}
