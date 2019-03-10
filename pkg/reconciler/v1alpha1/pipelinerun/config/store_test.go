/*
Copyright 2018 The Knative Authors.

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
	"github.com/tektoncd/pipeline/pkg/artifacts"

	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
)

func TestStoreLoadWithContext(t *testing.T) {
	store := NewStore(logtesting.TestLogger(t))
	bucketConfig := test.ConfigMapFromTestFile(t, "config-artifact-bucket")
	store.OnConfigChanged(bucketConfig)

	config := FromContext(store.ToContext(context.Background()))

	expected, _ := artifacts.NewArtifactBucketConfigFromConfigMap(bucketConfig)
	if diff := cmp.Diff(expected, config.ArtifactBucket); diff != "" {
		t.Errorf("Unexpected controller config (-want, +got): %v", diff)
	}
}
func TestStoreImmutableConfig(t *testing.T) {
	store := NewStore(logtesting.TestLogger(t))
	store.OnConfigChanged(test.ConfigMapFromTestFile(t, "config-artifact-bucket"))

	config := store.Load()

	config.ArtifactBucket.Location = "mutated"

	newConfig := store.Load()

	if newConfig.ArtifactBucket.Location == "mutated" {
		t.Error("Controller config is not immutable")
	}
}
