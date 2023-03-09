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

package testing

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

// SetDefaults sets the default ConfigMap values in an existing context (for use in testing)
func SetDefaults(ctx context.Context, t *testing.T, data map[string]string) context.Context {
	t.Helper()
	s := config.NewStore(logtesting.TestLogger(t))
	s.OnConfigChanged(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.GetDefaultsConfigName(),
		},
		Data: data,
	})
	return s.ToContext(ctx)
}
