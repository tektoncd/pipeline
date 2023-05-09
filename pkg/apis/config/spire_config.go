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

	sc "github.com/tektoncd/pipeline/pkg/spire/config"
	corev1 "k8s.io/api/core/v1"
)

const (
	// SpireConfigMapName is the name of the trusted resources configmap
	SpireConfigMapName = "config-spire"

	// SpireTrustDomain is the key to extract out the SPIRE trust domain to use
	SpireTrustDomain = "spire-trust-domain"
	// SpireSocketPath is the key to extract out the SPIRE agent socket for SPIFFE workload API
	SpireSocketPath = "spire-socket-path"
	// SpireServerAddr is the key to extract out the SPIRE server address for workload/node registration
	SpireServerAddr = "spire-server-addr"
	// SpireNodeAliasPrefix is the key to extract out the SPIRE node alias prefix to use
	SpireNodeAliasPrefix = "spire-node-alias-prefix"

	// SpireTrustDomainDefault is the default value for the SpireTrustDomain
	SpireTrustDomainDefault = "example.org"
	// SpireSocketPathDefault is the default value for the SpireSocketPath
	SpireSocketPathDefault = "unix:///spiffe-workload-api/spire-agent.sock"
	// SpireServerAddrDefault is the default value for the SpireServerAddr
	SpireServerAddrDefault = "spire-server.spire.svc.cluster.local:8081"
	// SpireNodeAliasPrefixDefault is the default value for the SpireNodeAliasPrefix
	SpireNodeAliasPrefixDefault = "/tekton-node/"
)

// DefaultSpire hols all the default configurations for the spire.
var DefaultSpire, _ = NewSpireConfigFromMap(map[string]string{})

// NewSpireConfigFromMap creates a Config from the supplied map
func NewSpireConfigFromMap(data map[string]string) (*sc.SpireConfig, error) {
	cfg := &sc.SpireConfig{}
	var ok bool
	if cfg.TrustDomain, ok = data[SpireTrustDomain]; !ok {
		cfg.TrustDomain = SpireTrustDomainDefault
	}
	if cfg.SocketPath, ok = data[SpireSocketPath]; !ok {
		cfg.SocketPath = SpireSocketPathDefault
	}
	if cfg.ServerAddr, ok = data[SpireServerAddr]; !ok {
		cfg.ServerAddr = SpireServerAddrDefault
	}
	if cfg.NodeAliasPrefix, ok = data[SpireNodeAliasPrefix]; !ok {
		cfg.NodeAliasPrefix = SpireNodeAliasPrefixDefault
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("failed to parse SPIRE configmap: %w", err)
	}
	return cfg, nil
}

// NewSpireConfigFromConfigMap creates a Config from the supplied ConfigMap
func NewSpireConfigFromConfigMap(configMap *corev1.ConfigMap) (*sc.SpireConfig, error) {
	return NewSpireConfigFromMap(configMap.Data)
}

// GetSpireConfigName returns the name of Spire ConfigMap
func GetSpireConfigName() string {
	if e := os.Getenv("CONFIG_SPIRE"); e != "" {
		return e
	}
	return SpireConfigMapName
}
