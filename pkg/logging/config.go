/*
Copyright 2019 The Tekton Authors

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

package logging

import (
	"go.uber.org/zap"

	"github.com/knative/pkg/logging"
)

const (
	// ConfigName is the name of the ConfigMap that the logging config will be stored in
	ConfigName = "config-logging"
)

// NewLogger creates a logger with the supplied configuration.
// In addition to the logger, it returns AtomicLevel that can
// be used to change the logging level at runtime.
// If configuration is empty, a fallback configuration is used.
// If configuration cannot be used to instantiate a logger,
// the same fallback configuration is used.
func NewLogger(configJSON string, levelOverride string) (*zap.SugaredLogger, zap.AtomicLevel) {
	return logging.NewLogger(configJSON, levelOverride)
}

// NewLoggerFromConfig creates a logger using the provided Config
func NewLoggerFromConfig(config *logging.Config, name string) (*zap.SugaredLogger, zap.AtomicLevel) {
	return logging.NewLoggerFromConfig(config, name)
}
