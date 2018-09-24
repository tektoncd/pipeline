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
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/knative/pkg/logging"
)

var (
	testLogger    *zap.SugaredLogger
	testLoggerMux sync.Mutex
)

// TestLogger gets a logger to use in unit and end to end tests
func TestLogger(t *testing.T) *zap.SugaredLogger {
	testLoggerMux.Lock()
	defer testLoggerMux.Unlock()

	if testLogger == nil {
		logger, err := zap.NewDevelopment()
		if err != nil {
			panic(err)
		}

		testLogger = logger.Sugar()
	}

	return testLogger.Named(t.Name())
}

// TestContextWithLogger returns a context with a logger to be used in tests
func TestContextWithLogger(t *testing.T) context.Context {
	return logging.WithLogger(context.TODO(), TestLogger(t))
}
