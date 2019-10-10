/*
Copyright 2019 The Knative Authors

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

package config

import (
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/configmap"
)

const (
	// ConfigName is the name of the config map for mako options.
	ConfigName = "config-mako"
)

// Config defines the mako configuration options.
type Config struct {
	// Organization holds the name of the organization for the current repository.
	Organization string

	// Repository holds the name of the repository that runs the benchmarks.
	Repository string

	// Environment holds the name of the environement,
	// where the test runs, e.g. `dev`.
	Environment string

	// List of additional tags to apply to the run.
	AdditionalTags []string

	// SlackConfig holds the slack configurations for the benchmarks,
	// it's used to determine which slack channels to alert on if there is performance regression.
	SlackConfig string
}

// NewConfigFromMap creates a Config from the supplied map
func NewConfigFromMap(data map[string]string) (*Config, error) {
	lc := &Config{
		Environment:    "dev",
		AdditionalTags: []string{},
	}

	if raw, ok := data["organization"]; ok {
		lc.Organization = raw
	}
	if raw, ok := data["repository"]; ok {
		lc.Repository = raw
	}
	if raw, ok := data["environment"]; ok {
		lc.Environment = raw
	}
	if raw, ok := data["additionalTags"]; ok && raw != "" {
		lc.AdditionalTags = strings.Split(raw, ",")
	}
	if raw, ok := data["slackConfig"]; ok {
		lc.SlackConfig = raw
	}

	return lc, nil
}

// NewConfigFromConfigMap creates a Config from the supplied ConfigMap
func NewConfigFromConfigMap(configMap *corev1.ConfigMap) (*Config, error) {
	return NewConfigFromMap(configMap.Data)
}

func loadConfig() (*Config, error) {
	makoCM, err := configmap.Load(filepath.Join("/etc", ConfigName))
	if err != nil {
		return nil, err
	}
	cfg, err := NewConfigFromMap(makoCM)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
