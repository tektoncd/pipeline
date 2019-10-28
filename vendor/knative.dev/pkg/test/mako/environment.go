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

package mako

import (
	"log"
	"path/filepath"

	"knative.dev/pkg/configmap"
)

// TODO: perhaps cache the loaded CM.

// MustGetTags returns the additional tags from the configmap, or dies.
func MustGetTags() []string {
	makoCM, err := configmap.Load(filepath.Join("/etc", ConfigName))
	if err != nil {
		log.Fatalf("unable to load configmap: %v", err)
	}
	cfg, err := NewConfigFromMap(makoCM)
	if err != nil {
		log.Fatalf("unable to parse configmap: %v", err)
	}
	return cfg.AdditionalTags
}

// getEnvironment fetches the Mako config environment to which this cluster should publish.
func getEnvironment() (string, error) {
	makoCM, err := configmap.Load(filepath.Join("/etc", ConfigName))
	if err != nil {
		return "", err
	}
	cfg, err := NewConfigFromMap(makoCM)
	if err != nil {
		return "", err
	}
	return cfg.Environment, nil
}
