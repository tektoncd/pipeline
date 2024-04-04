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
	"errors"
	"fmt"
	"sort"
	"strings"
)

// SpireConfig holds the images reference for a number of container images used
// across tektoncd pipelines.
// +k8s:deepcopy-gen=true
type SpireConfig struct {
	// The trust domain corresponds to the trust root of a SPIFFE identity provider.
	TrustDomain string
	// Path to the spire agent socket defined by the CSI driver
	SocketPath string
	// Spire server address
	ServerAddr string
	// Prefix to attach to the node name when registering it with the spire server
	NodeAliasPrefix string

	// MockSpire only to be used for testing the controller, will not exhibit
	// all characteristics of spire since it is only being used in the context
	// of process memory.
	MockSpire bool
}

// Validate returns an error if any image is not set.
func (c SpireConfig) Validate() error {
	var unset []string
	for _, f := range []struct {
		v, name string
	}{
		{c.TrustDomain, "spire-trust-domain"},
		{c.SocketPath, "spire-socket-path"},
		{c.ServerAddr, "spire-server-addr"},
		{c.NodeAliasPrefix, "spire-node-alias-prefix"},
	} {
		if f.v == "" {
			unset = append(unset, f.name)
		}
	}
	if len(unset) > 0 {
		sort.Strings(unset)
		return fmt.Errorf("found unset spire configuration flags: %s", unset)
	}

	if !strings.HasPrefix(c.NodeAliasPrefix, "/") {
		return errors.New("Spire node alias should start with a /")
	}

	return nil
}
