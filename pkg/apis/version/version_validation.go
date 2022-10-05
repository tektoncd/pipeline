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

package version

import (
	"context"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"knative.dev/pkg/apis"
)

// ValidateEnabledAPIFields checks that the enable-api-fields feature gate is set
// to a version at most as stable as wantVersion, if not, returns an error stating which feature
// is dependent on the version and what the current version actually is.
func ValidateEnabledAPIFields(ctx context.Context, featureName string, wantVersion string) *apis.FieldError {
	currentVersion := config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields
	var errs *apis.FieldError
	message := `%s requires "enable-api-fields" feature gate to be %s but it is %q`
	switch wantVersion {
	case config.StableAPIFields:
		// If the feature is stable, it doesn't matter what the current version is
	case config.BetaAPIFields:
		// If the feature requires "beta" fields to be enabled, the current version may be "beta" or "alpha"
		if currentVersion != config.BetaAPIFields && currentVersion != config.AlphaAPIFields {
			message = fmt.Sprintf(message, featureName, fmt.Sprintf("%q or %q", config.AlphaAPIFields, config.BetaAPIFields), currentVersion)
			errs = apis.ErrGeneric(message)
		}
	case config.AlphaAPIFields:
		// If the feature requires "alpha" fields to be enabled, the current version must be "alpha"
		if currentVersion != wantVersion {
			message = fmt.Sprintf(message, featureName, fmt.Sprintf("%q", config.AlphaAPIFields), currentVersion)
			errs = apis.ErrGeneric(message)
		}
	default:
		errs = apis.ErrGeneric("invalid wantVersion %s, must be one of (%s, %s, %s)", wantVersion, config.AlphaAPIFields, config.BetaAPIFields, config.StableAPIFields)
	}
	return errs
}
