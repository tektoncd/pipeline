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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	goversion "github.com/hashicorp/go-version"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	common "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const (
	// LabelValueHubResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueHubResolverType string = "hub"

	// ArtifactHubType is the value to use setting the type field to artifact
	ArtifactHubType string = "artifact"

	// TektonHubType is the value to use setting the type field to tekton
	TektonHubType string = "tekton"

	disabledError = "cannot handle resolution request, enable-hub-resolver feature flag not true"
)

var supportedKinds = []string{"task", "pipeline", "stepaction"}

// Resolver implements a framework.Resolver that can fetch files from OCI bundles.
//
// Deprecated: Use [github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/hub.Resolver] instead.
type Resolver struct {
	// TektonHubURL is the URL for hub resolver with type tekton
	TektonHubURL string
	// ArtifactHubURL is the URL for hub resolver with type artifact
	ArtifactHubURL string
}

// Initialize sets up any dependencies needed by the resolver. None atm.
func (r *Resolver) Initialize(context.Context) error {
	return nil
}

// GetName returns a string name to refer to this resolver by.
func (r *Resolver) GetName(context.Context) string {
	return "Hub"
}

// GetConfigName returns the name of the bundle resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return "hubresolver-config"
}

// GetSelector returns a map of labels to match requests to this resolver.
func (r *Resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueHubResolverType,
	}
}

// ValidateParams ensures parameters from a request are as expected.
func (r *Resolver) ValidateParams(ctx context.Context, params []pipelinev1.Param) error {
	return ValidateParams(ctx, params, r.TektonHubURL)
}

func ValidateParams(ctx context.Context, params []pipelinev1.Param, tektonHubUrl string) error {
	if isDisabled(ctx) {
		return errors.New(disabledError)
	}

	paramsMap, err := populateDefaultParams(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to populate default params: %w", err)
	}
	if err := validateParams(ctx, paramsMap, tektonHubUrl); err != nil {
		return fmt.Errorf("failed to validate params: %w", err)
	}

	return nil
}

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

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, params []pipelinev1.Param) (framework.ResolvedResource, error) {
	return Resolve(ctx, params, r.TektonHubURL, r.ArtifactHubURL)
}

func Resolve(ctx context.Context, params []pipelinev1.Param, tektonHubURL, artifactHubURL string) (framework.ResolvedResource, error) {
	if isDisabled(ctx) {
		return nil, errors.New(disabledError)
	}

	paramsMap, err := populateDefaultParams(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to populate default params: %w", err)
	}
	if err := validateParams(ctx, paramsMap, tektonHubURL); err != nil {
		return nil, fmt.Errorf("failed to validate params: %w", err)
	}

	if constraint, err := goversion.NewConstraint(paramsMap[ParamVersion]); err == nil {
		chosen, err := resolveVersionConstraint(ctx, paramsMap, constraint, artifactHubURL, tektonHubURL)
		if err != nil {
			return nil, err
		}
		paramsMap[ParamVersion] = chosen.String()
	}

	resVer, err := resolveVersion(paramsMap[ParamVersion], paramsMap[ParamType])
	if err != nil {
		return nil, err
	}
	paramsMap[ParamVersion] = resVer

	// call hub API
	switch paramsMap[ParamType] {
	case ArtifactHubType:
		url := fmt.Sprintf(fmt.Sprintf("%s/%s", artifactHubURL, ArtifactHubYamlEndpoint),
			paramsMap[ParamKind], paramsMap[ParamCatalog], paramsMap[ParamName], paramsMap[ParamVersion])
		resp := artifactHubResponse{}
		if err := fetchHubResource(ctx, url, &resp); err != nil {
			return nil, fmt.Errorf("fail to fetch Artifact Hub resource: %w", err)
		}
		return &ResolvedHubResource{
			URL:     url,
			Content: []byte(resp.Data.YAML),
		}, nil
	case TektonHubType:
		url := fmt.Sprintf(fmt.Sprintf("%s/%s", tektonHubURL, TektonHubYamlEndpoint),
			paramsMap[ParamCatalog], paramsMap[ParamKind], paramsMap[ParamName], paramsMap[ParamVersion])
		resp := tektonHubResponse{}
		if err := fetchHubResource(ctx, url, &resp); err != nil {
			return nil, fmt.Errorf("fail to fetch Tekton Hub resource: %w", err)
		}
		return &ResolvedHubResource{
			URL:     url,
			Content: []byte(resp.Data.YAML),
		}, nil
	}

	return nil, fmt.Errorf("hub resolver type: %s is not supported", paramsMap[ParamType])
}

// ResolvedHubResource wraps the data we want to return to Pipelines
type ResolvedHubResource struct {
	URL     string
	Content []byte
}

var _ framework.ResolvedResource = &ResolvedHubResource{}

// Data returns the bytes of our hard-coded Pipeline
func (rr *ResolvedHubResource) Data() []byte {
	return rr.Content
}

// Annotations returns any metadata needed alongside the data. None atm.
func (*ResolvedHubResource) Annotations() map[string]string {
	return nil
}

// RefSource is the source reference of the remote data that records where the remote
// file came from including the url, digest and the entrypoint.
func (rr *ResolvedHubResource) RefSource() *pipelinev1.RefSource {
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

func isDisabled(ctx context.Context) bool {
	cfg := resolverconfig.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.EnableHubResolver
}

func fetchHubResource(ctx context.Context, apiEndpoint string, v interface{}) error {
	// #nosec G107 -- URL cannot be constant in this case.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiEndpoint, nil)
	if err != nil {
		return fmt.Errorf("constructing request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting resource from Hub: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("requested resource '%s' not found on hub", apiEndpoint)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
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

func resolveCatalogName(paramsMap, conf map[string]string) (string, error) {
	var configTHCatalog, configAHTaskCatalog, configAHPipelineCatalog string
	var ok bool

	if configTHCatalog, ok = conf[ConfigTektonHubCatalog]; !ok {
		return "", errors.New("default Tekton Hub catalog was not set during installation of the hub resolver")
	}
	if configAHTaskCatalog, ok = conf[ConfigArtifactHubTaskCatalog]; !ok {
		return "", errors.New("default Artifact Hub task catalog was not set during installation of the hub resolver")
	}
	if configAHPipelineCatalog, ok = conf[ConfigArtifactHubPipelineCatalog]; !ok {
		return "", errors.New("default Artifact Hub pipeline catalog was not set during installation of the hub resolver")
	}
	if _, ok := paramsMap[ParamCatalog]; !ok {
		switch paramsMap[ParamType] {
		case ArtifactHubType:
			switch paramsMap[ParamKind] {
			case "task":
				return configAHTaskCatalog, nil
			case "pipeline":
				return configAHPipelineCatalog, nil
			default:
				return "", fmt.Errorf("failed to resolve catalog name with kind: %s", paramsMap[ParamKind])
			}
		case TektonHubType:
			return configTHCatalog, nil
		default:
			return "", fmt.Errorf("failed to resolve catalog name with type: %s", paramsMap[ParamType])
		}
	}

	return paramsMap[ParamCatalog], nil
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

// the Artifact Hub follows the semVer (i.e. <major-version>.<minor-version>.0)
// the Tekton Hub follows the simplified semVer (i.e. <major-version>.<minor-version>)
// for resolution request with "artifact" type, we append ".0" suffix if the input version is simplified semVer
// for resolution request with "tekton" type, we only use <major-version>.<minor-version> part of the input if it is semVer
func resolveVersion(version, hubType string) (string, error) {
	semVer := strings.Split(version, ".")
	resVer := version

	if hubType == ArtifactHubType && len(semVer) == 2 {
		resVer = version + ".0"
	} else if hubType == TektonHubType && len(semVer) > 2 {
		resVer = strings.Join(semVer[0:2], ".")
	}

	return resVer, nil
}

func populateDefaultParams(ctx context.Context, params []pipelinev1.Param) (map[string]string, error) {
	conf := framework.GetResolverConfigFromContext(ctx)
	paramsMap := make(map[string]string)
	for _, p := range params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	// type
	if _, ok := paramsMap[ParamType]; !ok {
		if typeString, ok := conf[ConfigType]; ok {
			paramsMap[ParamType] = typeString
		} else {
			return nil, errors.New("default type was not set during installation of the hub resolver")
		}
	}

	// kind
	if _, ok := paramsMap[ParamKind]; !ok {
		if kindString, ok := conf[ConfigKind]; ok {
			paramsMap[ParamKind] = kindString
		} else {
			return nil, errors.New("default resource kind was not set during installation of the hub resolver")
		}
	}

	// catalog
	resCatName, err := resolveCatalogName(paramsMap, conf)
	if err != nil {
		return nil, err
	}
	paramsMap[ParamCatalog] = resCatName

	return paramsMap, nil
}

func validateParams(ctx context.Context, paramsMap map[string]string, tektonHubURL string) error {
	var missingParams []string
	if _, ok := paramsMap[ParamName]; !ok {
		missingParams = append(missingParams, ParamName)
	}
	if _, ok := paramsMap[ParamVersion]; !ok {
		missingParams = append(missingParams, ParamVersion)
	}
	if kind, ok := paramsMap[ParamKind]; ok {
		if !isSupportedKind(kind) {
			return fmt.Errorf("kind param must be one of: %s", strings.Join(supportedKinds, ", "))
		}
	}
	if hubType, ok := paramsMap[ParamType]; ok {
		if hubType != ArtifactHubType && hubType != TektonHubType {
			return fmt.Errorf("type param must be %s or %s", ArtifactHubType, TektonHubType)
		}

		if hubType == TektonHubType && tektonHubURL == "" {
			return errors.New("please configure TEKTON_HUB_API env variable to use tekton type")
		}
	}

	if len(missingParams) > 0 {
		return fmt.Errorf("missing required hub resolver params: %s", strings.Join(missingParams, ", "))
	}

	return nil
}

func resolveVersionConstraint(ctx context.Context, paramsMap map[string]string, constraint goversion.Constraints, artifactHubURL, tektonHubURL string) (*goversion.Version, error) {
	var ret *goversion.Version
	if paramsMap[ParamType] == ArtifactHubType {
		allVersionsURL := fmt.Sprintf("%s/%s", artifactHubURL, fmt.Sprintf(
			ArtifactHubListTasksEndpoint,
			paramsMap[ParamKind], paramsMap[ParamCatalog], paramsMap[ParamName]))
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
				return nil, fmt.Errorf("fail to parse version %s from %s: %w", ArtifactHubType, vers.Version, err)
			}
			if checkV == nil {
				continue
			}
			if constraint.Check(checkV) {
				if ret != nil && ret.GreaterThan(checkV) {
					continue
				}
				// TODO(chmouel): log constraint result in controller
				ret = checkV
			}
		}
	} else if paramsMap[ParamType] == TektonHubType {
		allVersionsURL := fmt.Sprintf("%s/%s", tektonHubURL,
			fmt.Sprintf(TektonHubListTasksEndpoint,
				paramsMap[ParamCatalog], paramsMap[ParamKind], paramsMap[ParamName]))
		resp := tektonHubListResult{}
		if err := fetchHubResource(ctx, allVersionsURL, &resp); err != nil {
			return nil, fmt.Errorf("fail to fetch Tekton Hub resource: %w", err)
		}
		for _, vers := range resp.Data.Versions {
			checkV, err := goversion.NewVersion(vers.Version)
			if err != nil {
				return nil, fmt.Errorf("fail to parse version %s from %s: %w", TektonHubType, vers, err)
			}
			if checkV == nil {
				continue
			}
			if constraint.Check(checkV) {
				if ret != nil && ret.GreaterThan(checkV) {
					continue
				}
				// TODO(chmouel): log constraint result in controller
				ret = checkV
			}
		}
	}
	if ret == nil {
		return nil, fmt.Errorf("no version found for constraint %s", paramsMap[ParamVersion])
	}
	return ret, nil
}

func isSupportedKind(kindValue string) bool {
	return slices.Contains[[]string, string](supportedKinds, kindValue)
}
