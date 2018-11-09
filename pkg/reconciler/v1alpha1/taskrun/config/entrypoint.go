/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	// EntrypointConfigName is the name of the configmap containing all
	// customizations for entrypoint.
	EntrypointConfigName = "config-entrypoint"

	// ImageKey is the name of the configuration entry that specifies
	// entrypoint image.
	ImageKey = "image"

	// DefaultEntrypointImage is the default value of the ImageKey.
	DefaultEntrypointImage = "gcr.io/k8s-prow/entrypoint@sha256:7c7cd8906ce4982ffee326218e9fc75da2d4896d53cabc9833b9cc8d2d6b2b8f"
)

// Entrypoint contains the entrypoint configuration defined in the
// entrypoint config map.
type Entrypoint struct {
	// Image specifies the entrypoint image which will used by taskrun
	// to capture logs.
	Image string
}

// NewEntrypointConfigFromConfigMap creates a Entrypoint from the supplied ConfigMap
func NewEntrypointConfigFromConfigMap(configMap *corev1.ConfigMap) (*Entrypoint, error) {
	c := &Entrypoint{}

	if configMap.Data == nil {
		c.Image = DefaultEntrypointImage
		return c, nil
	}
	if image, ok := configMap.Data[ImageKey]; !ok {
		// It is OK for this to be absent, we will use the default value.
		c.Image = DefaultEntrypointImage
	} else {
		c.Image = image
	}
	return c, nil
}
