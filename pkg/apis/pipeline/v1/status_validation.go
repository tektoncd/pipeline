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

package v1

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"knative.dev/pkg/apis"
)

// ValidateEmbeddedStatus checks that the embedded-status feature gate is set to the wantEmbeddedStatus value and,
// if not, returns an error stating which feature is dependent on the status and what the current status actually is.
func ValidateEmbeddedStatus(ctx context.Context, featureName, wantEmbeddedStatus string) *apis.FieldError {
	embeddedStatus := config.FromContextOrDefaults(ctx).FeatureFlags.EmbeddedStatus
	if embeddedStatus != wantEmbeddedStatus {
		message := fmt.Sprintf(`%s requires "embedded-status" feature gate to be %q but it is %q`, featureName, wantEmbeddedStatus, embeddedStatus)
		return apis.ErrGeneric(message)
	}
	return nil
}
