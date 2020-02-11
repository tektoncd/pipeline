/*
Copyright 2020 The Tekton Authors

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

// nolint: golint
package v1alpha1

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*ClusterTask)(nil)

// ConvertUp implements api.Convertible
func (source *ClusterTask) ConvertUp(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1alpha2.ClusterTask:
		sink.ObjectMeta = source.ObjectMeta
		return source.Spec.ConvertUp(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertDown implements api.Convertible
func (sink *ClusterTask) ConvertDown(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1alpha2.ClusterTask:
		sink.ObjectMeta = source.ObjectMeta
		return sink.Spec.ConvertDown(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}
