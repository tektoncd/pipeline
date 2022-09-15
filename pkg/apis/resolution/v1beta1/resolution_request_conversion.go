/*
 Copyright 2022 The Tekton Authors

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

package v1beta1

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*ResolutionRequest)(nil)

// ConvertTo implements apis.Convertible
func (rr *ResolutionRequest) ConvertTo(ctx context.Context, sink apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	return fmt.Errorf("v1beta1 is the highest known version, got: %T", sink)
}

// ConvertFrom implements apis.Convertible
func (rr *ResolutionRequest) ConvertFrom(ctx context.Context, source apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	return fmt.Errorf("v1beta1 is the highest known version, got: %T", source)
}
