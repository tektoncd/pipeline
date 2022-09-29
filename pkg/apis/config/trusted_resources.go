/*
Copyright 2022 The Tekton Authors

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
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	cm "knative.dev/pkg/configmap"
)

// TrustedResources holds the collection of configurations that we attach to contexts.
// Configmap named with "config-trusted-resources" where cosign pub key path and
// KMS pub key path can be configured
// +k8s:deepcopy-gen=true
type TrustedResources struct {
	// Keys defines the name of the key in configmap data
	Keys sets.String
}

const (
	// DefaultPublicKeyPath is the default path of public key
	DefaultPublicKeyPath = ""
	// PublicKeys is the name of the public key keyref in configmap data
	PublicKeys = "publickeys"
	// TrustedTaskConfig is the name of the trusted resources configmap
	TrustedTaskConfig = "config-trusted-resources"
)

// NewTrustedResourcesConfigFromMap creates a Config from the supplied map
func NewTrustedResourcesConfigFromMap(data map[string]string) (*TrustedResources, error) {
	cfg := &TrustedResources{
		Keys: sets.NewString(DefaultPublicKeyPath),
	}
	if err := cm.Parse(data,
		cm.AsStringSet(PublicKeys, &cfg.Keys),
	); err != nil {
		return nil, fmt.Errorf("failed to parse data: %w", err)
	}
	return cfg, nil
}

// NewTrustedResourcesConfigFromConfigMap creates a Config from the supplied ConfigMap
func NewTrustedResourcesConfigFromConfigMap(configMap *corev1.ConfigMap) (*TrustedResources, error) {
	return NewTrustedResourcesConfigFromMap(configMap.Data)
}

// GetTrustedResourcesConfigName returns the name of TrustedResources ConfigMap
func GetTrustedResourcesConfigName() string {
	if e := os.Getenv("CONFIG_TRUSTED_RESOURCES_NAME"); e != "" {
		return e
	}
	return TrustedTaskConfig
}
