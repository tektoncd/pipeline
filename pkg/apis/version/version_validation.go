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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// ValidateEnabledAPIFieldsBasedOnOriginalObject determines the value of "enable-api-fields" when the object was first created
func ValidateEnabledAPIFieldsBasedOnOriginalObject(ctx context.Context, featureName string, wantVersion string, objectMeta *metav1.ObjectMeta) *apis.FieldError {
	if objectMeta.Annotations != nil {
		originalAPIVersion := objectMeta.Annotations[OriginalVersionKey]
		originalEnableAPIFields := objectMeta.Annotations[OriginalEnableAPIFieldsKey]

		if originalAPIVersion == "v1beta1" && originalEnableAPIFields == config.StableAPIFields {
			return validateEnabledAPIFields(config.BetaAPIFields, featureName, wantVersion)
		}
	}

	// Otherwise, we can use the current value of enable-api-fields.
	currentEnableAPIFields := config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields
	return validateEnabledAPIFields(currentEnableAPIFields, featureName, wantVersion)
}

// ValidateEnabledAPIFields checks that the enable-api-fields feature gate is set
// to a version at most as stable as wantVersion, if not, returns an error stating which feature
// is dependent on the version and what the current version actually is.
func ValidateEnabledAPIFields(ctx context.Context, featureName string, wantVersion string) *apis.FieldError {
	currentVersion := config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields
	return validateEnabledAPIFields(currentVersion, featureName, wantVersion)
}

func validateEnabledAPIFields(version string, featureName string, wantVersion string) *apis.FieldError {
	var errs *apis.FieldError
	message := `%s requires "enable-api-fields" feature gate to be %s but it is %q`
	switch wantVersion {
	case config.StableAPIFields:
		// If the feature is stable, it doesn't matter what the current version is
	case config.BetaAPIFields:
		// If the feature requires "beta" fields to be enabled, the current version may be "beta" or "alpha"
		if version != config.BetaAPIFields && version != config.AlphaAPIFields {
			message = fmt.Sprintf(message, featureName, fmt.Sprintf("%q or %q", config.AlphaAPIFields, config.BetaAPIFields), version)
			errs = apis.ErrGeneric(message)
		}
	case config.AlphaAPIFields:
		// If the feature requires "alpha" fields to be enabled, the current version must be "alpha"
		if version != wantVersion {
			message = fmt.Sprintf(message, featureName, fmt.Sprintf("%q", config.AlphaAPIFields), version)
			errs = apis.ErrGeneric(message)
		}
	default:
		errs = apis.ErrGeneric("invalid wantVersion %s, must be one of (%s, %s, %s)", wantVersion, config.AlphaAPIFields, config.BetaAPIFields, config.StableAPIFields)
	}
	return errs
}
