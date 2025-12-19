// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package runtime // import "go.opentelemetry.io/contrib/instrumentation/runtime"

// Version is the current release version of the runtime instrumentation.
func Version() string {
	return "0.64.0"
	// This string is updated by the pre_release.sh script during release
}
