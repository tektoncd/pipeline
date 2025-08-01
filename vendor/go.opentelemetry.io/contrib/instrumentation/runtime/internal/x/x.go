// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package x contains support for OTel runtime instrumentation experimental features.
//
// This package should only be used for features defined in the specification.
// It should not be used for experiments or new project ideas.
package x // import "go.opentelemetry.io/contrib/instrumentation/runtime/internal/x"

import (
	"os"
	"strconv"
)

// DeprecatedRuntimeMetrics is an experimental feature flag that defines if the deprecated
// runtime metrics should be produced. During development of the new
// conventions, it is enabled by default.
//
// To enable this feature set the OTEL_GO_X_DEPRECATED_RUNTIME_METRICS environment variable
// to the case-insensitive string value of "true" (i.e. "True" and "TRUE"
// will also enable this).
var DeprecatedRuntimeMetrics = newFeature("DEPRECATED_RUNTIME_METRICS", false)

// BoolFeature is an experimental feature control flag. It provides a uniform way
// to interact with these feature flags and parse their values.
type BoolFeature struct {
	key        string
	defaultVal bool
}

func newFeature(suffix string, defaultVal bool) BoolFeature {
	const envKeyRoot = "OTEL_GO_X_"
	return BoolFeature{
		key:        envKeyRoot + suffix,
		defaultVal: defaultVal,
	}
}

// Key returns the environment variable key that needs to be set to enable the
// feature.
func (f BoolFeature) Key() string { return f.key }

// Enabled returns if the feature is enabled.
func (f BoolFeature) Enabled() bool {
	v := os.Getenv(f.key)

	val, err := strconv.ParseBool(v)
	if err != nil {
		return f.defaultVal
	}

	return val
}
