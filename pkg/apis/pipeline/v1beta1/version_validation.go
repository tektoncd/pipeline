/*
Copyright 2021 The Tekton Authors

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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"knative.dev/pkg/apis"
)

// ValidateEnabledAPIFields checks that the enable-api-fields feature gate is set
// to the wantVersion value and, if not, returns an error stating which feature
// is dependent on the version and what the current version actually is.
func ValidateEnabledAPIFields(ctx context.Context, featureName, wantVersion string) *apis.FieldError {
	currentVersion := config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields
	if currentVersion != wantVersion {
		var errs *apis.FieldError
		message := fmt.Sprintf(`%s requires "enable-api-fields" feature gate to be %q but it is %q`, featureName, wantVersion, currentVersion)
		return errs.Also(apis.ErrGeneric(message))
	}
	return nil
}
