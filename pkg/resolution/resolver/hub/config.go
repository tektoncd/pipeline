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

package hub

import (
	"fmt"
	"net/url"
	"strings"

	"sigs.k8s.io/yaml"
)

// ConfigTektonHubCatalog is the configuration field name for controlling
// the Tekton Hub catalog to fetch the remote resource from.
const ConfigTektonHubCatalog = "default-tekton-hub-catalog"

// ConfigArtifactHubTaskCatalog is the configuration field name for controlling
// the Artifact Hub Task catalog to fetch the remote resource from.
const ConfigArtifactHubTaskCatalog = "default-artifact-hub-task-catalog"

// ConfigArtifactHubPipelineCatalog is the configuration field name for controlling
// the Artifact Hub Pipeline catalog to fetch the remote resource from.
const ConfigArtifactHubPipelineCatalog = "default-artifact-hub-pipeline-catalog"

// ConfigKind is the configuration field name for controlling
// what the layer name in the hub image is.
const ConfigKind = "default-kind"

// ConfigType is the configuration field name for controlling
// the hub type to pull the resource from.
const ConfigType = "default-type"

// ConfigArtifactHubURLs is the configuration field name for controlling
// the Artifact Hub API URLs to fetch remote resources from.
// Value is a YAML list of URLs, tried in order; first success wins.
const ConfigArtifactHubURLs = "artifact-hub-urls"

// ConfigTektonHubURLs is the configuration field name for controlling
// the Tekton Hub API URLs to fetch remote resources from.
// Value is a YAML list of URLs, tried in order; first success wins.
const ConfigTektonHubURLs = "tekton-hub-urls"

// parseURLList parses a YAML list string from a ConfigMap value into
// a slice of URL strings. Each URL is trimmed of whitespace and trailing slashes.
// Returns nil if the input is empty or not a valid YAML list.
func parseURLList(yamlList string) ([]string, error) {
	yamlList = strings.TrimSpace(yamlList)
	if yamlList == "" {
		return nil, nil
	}
	var urls []string
	if err := yaml.Unmarshal([]byte(yamlList), &urls); err != nil {
		return nil, fmt.Errorf("failed to parse URL list: %w", err)
	}
	for i, u := range urls {
		urls[i] = strings.TrimRight(strings.TrimSpace(u), "/")
		if err := validateHubURL(urls[i]); err != nil {
			return nil, err
		}
	}
	return urls, nil
}

// validateHubURL checks that rawURL is a valid absolute http(s) URL.
func validateHubURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("url must be a valid absolute URL: %s", rawURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url must use http or https scheme: %s", rawURL)
	}
	return nil
}
