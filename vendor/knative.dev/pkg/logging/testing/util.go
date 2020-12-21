/*
Copyright 2018 The Knative Authors.

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

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"knative.dev/pkg/logging"
)

// TestLogger gets a logger to use in unit and end to end tests
func TestLogger(t zaptest.TestingT) *zap.SugaredLogger {
	opts := zaptest.WrapOptions(
		zap.AddCaller(),
		zap.Development(),
	)

	return zaptest.NewLogger(t, opts).Sugar()
}

// TestContextWithLogger returns a context with a logger to be used in tests
func TestContextWithLogger(t zaptest.TestingT) context.Context {
	return logging.WithLogger(context.Background(), TestLogger(t))
}
