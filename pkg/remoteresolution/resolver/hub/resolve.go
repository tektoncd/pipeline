/*
Copyright 2024 The Tekton Authors
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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	goversion "github.com/hashicorp/go-version"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	hub "github.com/tektoncd/pipeline/pkg/resolution/resolver/hub"
	"knative.dev/pkg/logging"
)

var errNoVersionFound = errors.New("no version found")

// Response types for hub API calls.

type tektonHubDataResponse struct {
	YAML string `json:"yaml"`
}

type tektonHubResponse struct {
	Data tektonHubDataResponse `json:"data"`
}

type artifactHubDataResponse struct {
	YAML string `json:"manifestRaw"`
}

type artifactHubResponse struct {
	Data artifactHubDataResponse `json:"data"`
}

type artifactHubavailableVersionsResults struct {
	Version    string `json:"version"`
	Prerelease bool   `json:"prerelease"`
}

type artifactHubListResult struct {
	AvailableVersions []artifactHubavailableVersionsResults `json:"available_versions"`
	Version           string                                `json:"version"`
}

type tektonHubListResultVersion struct {
	Version string `json:"version"`
}

type tektonHubListDataResult struct {
	Versions []tektonHubListResultVersion `json:"versions"`
}

type tektonHubListResult struct {
	Data tektonHubListDataResult `json:"data"`
}

// resolvedHubResource wraps the data we want to return to Pipelines.
type resolvedHubResource struct {
	URL     string
	Content []byte
}

var _ resolutionframework.ResolvedResource = &resolvedHubResource{}

func (rr *resolvedHubResource) Data() []byte {
	return rr.Content
}

func (*resolvedHubResource) Annotations() map[string]string {
	return nil
}

func (rr *resolvedHubResource) RefSource() *pipelinev1.RefSource {
	h := sha256.New()
	h.Write(rr.Content)
	sha256CheckSum := hex.EncodeToString(h.Sum(nil))

	return &pipelinev1.RefSource{
		URI: rr.URL,
		Digest: map[string]string{
			"sha256": sha256CheckSum,
		},
	}
}

// fetchHubResource fetches a resource from the hub API endpoint and unmarshals the response.
func fetchHubResource(ctx context.Context, apiEndpoint string, v any) error {
	// #nosec G107 -- URL cannot be constant in this case.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiEndpoint, nil)
	if err != nil {
		return fmt.Errorf("constructing request: %w", err)
	}

	// #nosec G704 -- URL cannot be constant in this case.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting resource from Hub: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("requested resource '%s' not found on hub", apiEndpoint)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	err = json.Unmarshal(body, v)
	if err != nil {
		return fmt.Errorf("error unmarshalling json response: %w", err)
	}
	return nil
}

// resolveVersion normalizes the version string for the given hub type.
func resolveVersion(version, hubType string) string {
	semVer := strings.Split(version, ".")
	if hubType == hub.ArtifactHubType && len(semVer) == 2 {
		return version + ".0"
	} else if hubType == hub.TektonHubType && len(semVer) > 2 {
		return strings.Join(semVer[0:2], ".")
	}
	return version
}

// resolveHubURLs returns the ordered list of hub URLs to try.
// Precedence: ConfigMap YAML list > env var URL.
func resolveHubURLs(ctx context.Context, conf map[string]string, envVarURL, configKey string) []string {
	if yamlStr, ok := conf[configKey]; ok && strings.TrimSpace(yamlStr) != "" {
		urls, err := parseURLList(yamlStr)
		if err != nil {
			logger := logging.FromContext(ctx)
			logger.Warnf("failed to parse %s ConfigMap value, falling back to env var URL: %v", configKey, err)
		} else if len(urls) > 0 {
			return urls
		}
	}
	if envVarURL != "" {
		return []string{envVarURL}
	}
	return nil
}

// fetchResourceFromURL fetches a resource from a single hub URL.
func fetchResourceFromURL(ctx context.Context, paramsMap map[string]string, baseURL string) (resolutionframework.ResolvedResource, error) {
	switch paramsMap[hub.ParamType] {
	case hub.ArtifactHubType:
		url := fmt.Sprintf(fmt.Sprintf("%s/%s", baseURL, hub.ArtifactHubYamlEndpoint),
			paramsMap[hub.ParamKind], paramsMap[hub.ParamCatalog], paramsMap[hub.ParamName], paramsMap[hub.ParamVersion])
		resp := artifactHubResponse{}
		if err := fetchHubResource(ctx, url, &resp); err != nil {
			return nil, fmt.Errorf("fail to fetch Artifact Hub resource: %w", err)
		}
		return &resolvedHubResource{
			URL:     url,
			Content: []byte(resp.Data.YAML),
		}, nil
	case hub.TektonHubType:
		url := fmt.Sprintf(fmt.Sprintf("%s/%s", baseURL, hub.TektonHubYamlEndpoint),
			paramsMap[hub.ParamCatalog], paramsMap[hub.ParamKind], paramsMap[hub.ParamName], paramsMap[hub.ParamVersion])
		resp := tektonHubResponse{}
		if err := fetchHubResource(ctx, url, &resp); err != nil {
			return nil, fmt.Errorf("fail to fetch Tekton Hub resource: %w", err)
		}
		return &resolvedHubResource{
			URL:     url,
			Content: []byte(resp.Data.YAML),
		}, nil
	}
	return nil, fmt.Errorf("hub resolver type: %s is not supported", paramsMap[hub.ParamType])
}

// fetchResourceWithFallback tries each URL in order, returns the first successful result.
// When there is only one URL, error messages are preserved exactly as before.
func fetchResourceWithFallback(ctx context.Context, paramsMap map[string]string, urls []string) (resolutionframework.ResolvedResource, error) {
	var errs []error
	for _, baseURL := range urls {
		result, err := fetchResourceFromURL(ctx, paramsMap, baseURL)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return result, nil
	}
	if len(errs) == 1 {
		return nil, errs[0]
	}
	return nil, fmt.Errorf("failed to fetch resource from any configured hub URL: %w", errors.Join(errs...))
}

// resolveVersionConstraintFromURL resolves a version constraint from a single hub URL.
func resolveVersionConstraintFromURL(ctx context.Context, paramsMap map[string]string, constraint goversion.Constraints, baseURL string) (*goversion.Version, error) {
	var ret *goversion.Version
	if paramsMap[hub.ParamType] == hub.ArtifactHubType {
		allVersionsURL := fmt.Sprintf("%s/%s", baseURL, fmt.Sprintf(
			hub.ArtifactHubListTasksEndpoint,
			paramsMap[hub.ParamKind], paramsMap[hub.ParamCatalog], paramsMap[hub.ParamName]))
		resp := artifactHubListResult{}
		if err := fetchHubResource(ctx, allVersionsURL, &resp); err != nil {
			return nil, fmt.Errorf("fail to fetch Artifact Hub resource: %w", err)
		}
		for _, vers := range resp.AvailableVersions {
			if vers.Prerelease {
				continue
			}
			checkV, err := goversion.NewVersion(vers.Version)
			if err != nil {
				return nil, fmt.Errorf("fail to parse version %s from %s: %w", hub.ArtifactHubType, vers.Version, err)
			}
			if checkV == nil {
				continue
			}
			if constraint.Check(checkV) {
				if ret != nil && ret.GreaterThan(checkV) {
					continue
				}
				ret = checkV
			}
		}
	} else if paramsMap[hub.ParamType] == hub.TektonHubType {
		allVersionsURL := fmt.Sprintf("%s/%s", baseURL,
			fmt.Sprintf(hub.TektonHubListTasksEndpoint,
				paramsMap[hub.ParamCatalog], paramsMap[hub.ParamKind], paramsMap[hub.ParamName]))
		resp := tektonHubListResult{}
		if err := fetchHubResource(ctx, allVersionsURL, &resp); err != nil {
			return nil, fmt.Errorf("fail to fetch Tekton Hub resource: %w", err)
		}
		for _, vers := range resp.Data.Versions {
			checkV, err := goversion.NewVersion(vers.Version)
			if err != nil {
				return nil, fmt.Errorf("fail to parse version %s from %s: %w", hub.TektonHubType, vers, err)
			}
			if checkV == nil {
				continue
			}
			if constraint.Check(checkV) {
				if ret != nil && ret.GreaterThan(checkV) {
					continue
				}
				ret = checkV
			}
		}
	}
	if ret == nil {
		return nil, fmt.Errorf("%w for constraint %s", errNoVersionFound, paramsMap[hub.ParamVersion])
	}
	return ret, nil
}

// resolveVersionConstraintWithFallback tries each URL in order for version constraint resolution.
// Returns the resolved version and the URL that satisfied the constraint, so the caller can
// pin subsequent fetches to the same hub.
func resolveVersionConstraintWithFallback(ctx context.Context, paramsMap map[string]string, constraint goversion.Constraints, urls []string) (*goversion.Version, string, error) {
	var errs []error
	for _, baseURL := range urls {
		ver, err := resolveVersionConstraintFromURL(ctx, paramsMap, constraint, baseURL)
		if err != nil {
			if errors.Is(err, errNoVersionFound) {
				// Hub was reachable but had no matching version — don't try other hubs.
				return nil, "", err
			}
			errs = append(errs, err)
			continue
		}
		return ver, baseURL, nil
	}
	if len(errs) == 1 {
		return nil, "", errs[0]
	}
	return nil, "", fmt.Errorf("failed to resolve version constraint from any configured hub URL: %w", errors.Join(errs...))
}
