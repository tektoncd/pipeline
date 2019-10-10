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

package gke

import (
	"fmt"
	"strings"

	container "google.golang.org/api/container/v1beta1"
)

const (
	// Define all supported addons here
	istio = "istio"
)

// GetAddonsConfig gets AddonsConfig from a slice of addon names, contains the logic of
// converting string argument to typed AddonsConfig, for example `IstioConfig`.
// Currently supports istio
func GetAddonsConfig(addons []string) *container.AddonsConfig {
	ac := &container.AddonsConfig{}
	for _, name := range addons {
		switch strings.ToLower(name) {
		case istio:
			ac.IstioConfig = &container.IstioConfig{Disabled: false}
		default:
			panic(fmt.Sprintf("addon type %q not supported. Has to be one of: %q", name, istio))
		}
	}

	return ac
}
