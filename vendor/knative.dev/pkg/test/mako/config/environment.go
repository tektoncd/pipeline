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
	"log"
)

// TODO: perhaps cache the loaded CM.

const defaultOrg = "knative"

// GetOrganization returns the organization from the configmap.
// It will return the defaultOrg if any error happens or it's empty.
func GetOrganization() string {
	cfg, err := loadConfig()
	if err != nil {
		return defaultOrg
	}
	if cfg.Organization == "" {
		return defaultOrg
	}
	return cfg.Organization
}

// GetRepository returns the repository from the configmap.
// It will return an empty string if any error happens.
func GetRepository() string {
	cfg, err := loadConfig()
	if err != nil {
		return ""
	}
	return cfg.Repository
}

// MustGetTags returns the additional tags from the configmap, or dies.
func MustGetTags() []string {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("unable to load config from the configmap: %v", err)
	}
	return cfg.AdditionalTags
}

// getEnvironment fetches the Mako config environment to which this cluster should publish.
func getEnvironment() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	return cfg.Environment, nil
}
