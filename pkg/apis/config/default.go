/*
Copyright 2019 The Tekton Authors.

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
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	// ConfigName is the name of the configmap
	DefaultsConfigName       = "config-defaults"
	DefaultTimeoutMinutes    = 60
	defaultTimeoutMinutesKey = "default-timeout-minutes"
)

// ConfigDefault holds the default configurations
// +k8s:deepcopy-gen=true
type ConfigDefault struct {
	DefaultTimeoutMinutes int
}

// Equals returns true if two Configs are identical
func (cfg *ConfigDefault) Equals(other *ConfigDefault) bool {
	return other.DefaultTimeoutMinutes == cfg.DefaultTimeoutMinutes
}

// NewConfigDefaultFromMap returns a Config given a map corresponding to a ConfigMap
func NewConfigDefaultFromMap(cfgMap map[string]string) (*ConfigDefault, error) {
	tc := ConfigDefault{
		DefaultTimeoutMinutes: DefaultTimeoutMinutes,
	}
	if defaultTimeoutMin, ok := cfgMap[defaultTimeoutMinutesKey]; ok {
		timeout, err := strconv.ParseInt(defaultTimeoutMin, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("failed parsing tracing config %q", defaultTimeoutMinutesKey)
		}
		tc.DefaultTimeoutMinutes = int(timeout)
	}

	return &tc, nil
}

// NewConfigDefaultFromConfigMap returns a Config for the given configmap
func NewConfigDefaultFromConfigMap(config *corev1.ConfigMap) (*ConfigDefault, error) {
	return NewConfigDefaultFromMap(config.Data)
}
