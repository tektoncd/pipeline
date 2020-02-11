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
package v1alpha2

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*Task)(nil)

// ConvertUp implements api.Convertible
func (source *Task) ConvertUp(ctx context.Context, sink apis.Convertible) error {
	return fmt.Errorf("v1alpha2 is the highest known version, got: %T", sink)
}

// ConvertDown implements api.Convertible
func (sink *Task) ConvertDown(ctx context.Context, source apis.Convertible) error {
	return fmt.Errorf("v1alpha2 is the highest know version, got: %T", source)
}
