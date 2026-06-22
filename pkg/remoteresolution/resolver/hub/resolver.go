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
	"errors"
	"fmt"
	"slices"
	"strings"

	goversion "github.com/hashicorp/go-version"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	hub "github.com/tektoncd/pipeline/pkg/resolution/resolver/hub"
)

const (
	// LabelValueHubResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueHubResolverType = "hub"
	hubResolverName           = "Hub"
	configMapName             = "hubresolver-config"

	// ArtifactHubType is the value to use setting the type field to artifact
	ArtifactHubType = "artifact"

	// TektonHubType is the value to use setting the type field to tekton
	TektonHubType = "tekton"

	disabledError = "cannot handle resolution request, enable-hub-resolver feature flag not true"
)

var _ framework.Resolver = (*Resolver)(nil)
var _ resolutionframework.ConfigWatcher = (*Resolver)(nil)

// Resolver implements a framework.Resolver that can fetch files from OCI bundles.
type Resolver struct {
	// TektonHubURL is the URL for hub resolver with type tekton
	TektonHubURL string
	// ArtifactHubURL is the URL for hub resolver with type artifact
	ArtifactHubURL string
}

// Initialize sets up any dependencies needed by the resolver. None atm.
func (r *Resolver) Initialize(_ context.Context) error {
	return nil
}

// GetName returns a string name to refer to this resolver by.
func (r *Resolver) GetName(_ context.Context) string {
	return hubResolverName
}

// GetConfigName returns the name of the bundle resolver's configmap.
func (r *Resolver) GetConfigName(_ context.Context) string {
	return configMapName
}

// GetSelector returns a map of labels to match requests to this resolver.
func (r *Resolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueHubResolverType,
	}
}

// Validate ensures parameters from a request are as expected.
func (r *Resolver) Validate(ctx context.Context, req *v1beta1.ResolutionRequestSpec) error {
	if isDisabled(ctx) {
		return errors.New(disabledError)
	}

	paramsMap := extractParams(req.Params)

	// Validate URL param if provided.
	if paramURL := paramsMap[ParamURL]; paramURL != "" {
		if err := validateHubURL(paramURL); err != nil {
			return fmt.Errorf("failed to validate params: invalid url param: %w", err)
		}
	}

	// For TektonHub type with no env var URL, check if url param or ConfigMap URLs are available.
	hasURLOverride := paramsMap[ParamURL] != ""
	conf := resolutionframework.GetResolverConfigFromContext(ctx)
	hubType := paramsMap[hub.ParamType]
	if hubType == "" {
		hubType = conf[hub.ConfigType]
	}
	if hubType == hub.TektonHubType && r.TektonHubURL == "" {
		if hasURLOverride {
			// URL param overrides the env var, pass a placeholder to skip the env var check.
			return hub.ValidateParams(ctx, req.Params, "url-param-configured")
		}
		configURLs, _ := parseURLList(conf[ConfigTektonHubURLs])
		if len(configURLs) > 0 {
			// ConfigMap URLs are available, skip the env var check.
			return hub.ValidateParams(ctx, req.Params, "configmap-urls-configured")
		}
	}

	return hub.ValidateParams(ctx, req.Params, r.TektonHubURL)
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, req *v1beta1.ResolutionRequestSpec) (resolutionframework.ResolvedResource, error) {
	if isDisabled(ctx) {
		return nil, errors.New(disabledError)
	}

	paramsMap, err := populateDefaultParams(ctx, req.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to populate default params: %w", err)
	}
	if err := r.validateParamsForResolve(ctx, paramsMap); err != nil {
		return nil, fmt.Errorf("failed to validate params: %w", err)
	}

	// Determine ordered list of hub URLs to try.
	// Precedence: url param > ConfigMap YAML list > env var URL.
	var urls []string
	if paramURL := paramsMap[ParamURL]; paramURL != "" {
		urls = []string{strings.TrimRight(paramURL, "/")}
	} else {
		conf := resolutionframework.GetResolverConfigFromContext(ctx)
		switch paramsMap[hub.ParamType] {
		case hub.ArtifactHubType:
			urls = resolveHubURLs(ctx, conf, r.ArtifactHubURL, ConfigArtifactHubURLs)
		case hub.TektonHubType:
			urls = resolveHubURLs(ctx, conf, r.TektonHubURL, ConfigTektonHubURLs)
		}
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("no hub URL configured for type %s", paramsMap[hub.ParamType])
	}

	if constraint, err := goversion.NewConstraint(paramsMap[hub.ParamVersion]); err == nil {
		chosen, constraintURL, err := resolveVersionConstraintWithFallback(ctx, paramsMap, constraint, urls)
		if err != nil {
			return nil, err
		}
		paramsMap[hub.ParamVersion] = chosen.String()
		// Pin subsequent fetch to the same hub that satisfied the constraint.
		urls = []string{constraintURL}
	}

	resVer := resolveVersion(paramsMap[hub.ParamVersion], paramsMap[hub.ParamType])
	paramsMap[hub.ParamVersion] = resVer

	return fetchResourceWithFallback(ctx, paramsMap, urls)
}

// isDisabled checks if the hub resolver feature flag is disabled.
func isDisabled(ctx context.Context) bool {
	cfg := resolverconfig.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.EnableHubResolver
}

// extractParams converts a param slice to a simple map.
func extractParams(params []pipelinev1.Param) map[string]string {
	paramsMap := make(map[string]string)
	for _, p := range params {
		paramsMap[p.Name] = p.Value.StringVal
	}
	return paramsMap
}

// populateDefaultParams populates the params map with defaults from the resolver config.
func populateDefaultParams(ctx context.Context, params []pipelinev1.Param) (map[string]string, error) {
	conf := resolutionframework.GetResolverConfigFromContext(ctx)
	paramsMap := extractParams(params)

	// type
	if _, ok := paramsMap[hub.ParamType]; !ok {
		if typeString, ok := conf[hub.ConfigType]; ok {
			paramsMap[hub.ParamType] = typeString
		} else {
			return nil, errors.New("default type was not set during installation of the hub resolver")
		}
	}

	// kind
	if _, ok := paramsMap[hub.ParamKind]; !ok {
		if kindString, ok := conf[hub.ConfigKind]; ok {
			paramsMap[hub.ParamKind] = kindString
		} else {
			return nil, errors.New("default resource kind was not set during installation of the hub resolver")
		}
	}

	// catalog
	resCatName, err := resolveCatalogName(paramsMap, conf)
	if err != nil {
		return nil, err
	}
	paramsMap[hub.ParamCatalog] = resCatName

	return paramsMap, nil
}

// resolveCatalogName resolves the catalog name from params or config defaults.
func resolveCatalogName(paramsMap, conf map[string]string) (string, error) {
	var configTHCatalog, configAHTaskCatalog, configAHPipelineCatalog string
	var ok bool

	if configTHCatalog, ok = conf[hub.ConfigTektonHubCatalog]; !ok {
		return "", errors.New("default Tekton Hub catalog was not set during installation of the hub resolver")
	}
	if configAHTaskCatalog, ok = conf[hub.ConfigArtifactHubTaskCatalog]; !ok {
		return "", errors.New("default Artifact Hub task catalog was not set during installation of the hub resolver")
	}
	if configAHPipelineCatalog, ok = conf[hub.ConfigArtifactHubPipelineCatalog]; !ok {
		return "", errors.New("default Artifact Hub pipeline catalog was not set during installation of the hub resolver")
	}
	if _, ok := paramsMap[hub.ParamCatalog]; !ok {
		switch paramsMap[hub.ParamType] {
		case hub.ArtifactHubType:
			switch paramsMap[hub.ParamKind] {
			case "task":
				return configAHTaskCatalog, nil
			case "pipeline":
				return configAHPipelineCatalog, nil
			default:
				return "", fmt.Errorf("failed to resolve catalog name with kind: %s", paramsMap[hub.ParamKind])
			}
		case hub.TektonHubType:
			return configTHCatalog, nil
		default:
			return "", fmt.Errorf("failed to resolve catalog name with type: %s", paramsMap[hub.ParamType])
		}
	}

	return paramsMap[hub.ParamCatalog], nil
}

// validateParamsForResolve validates params for the resolve path (internal, after populateDefaultParams).
func (r *Resolver) validateParamsForResolve(ctx context.Context, paramsMap map[string]string) error {
	var missingParams []string
	if _, ok := paramsMap[hub.ParamName]; !ok {
		missingParams = append(missingParams, hub.ParamName)
	}
	if _, ok := paramsMap[hub.ParamVersion]; !ok {
		missingParams = append(missingParams, hub.ParamVersion)
	}
	if kind, ok := paramsMap[hub.ParamKind]; ok {
		supportedKinds := []string{"task", "pipeline", "stepaction"}
		if !slices.Contains(supportedKinds, kind) {
			return fmt.Errorf("kind param must be one of: %s", strings.Join(supportedKinds, ", "))
		}
	}

	if paramURL, ok := paramsMap[ParamURL]; ok && paramURL != "" {
		if err := validateHubURL(paramURL); err != nil {
			return fmt.Errorf("invalid url param: %w", err)
		}
	}

	hasURLOverride := paramsMap[ParamURL] != ""
	if hubType, ok := paramsMap[hub.ParamType]; ok {
		if hubType != hub.ArtifactHubType && hubType != hub.TektonHubType {
			return fmt.Errorf("type param must be %s or %s", hub.ArtifactHubType, hub.TektonHubType)
		}

		if hubType == hub.TektonHubType && r.TektonHubURL == "" && !hasURLOverride {
			conf := resolutionframework.GetResolverConfigFromContext(ctx)
			configURLs, _ := parseURLList(conf[ConfigTektonHubURLs])
			if len(configURLs) == 0 {
				return errors.New("please configure TEKTON_HUB_API env variable to use tekton type")
			}
		}
	}

	if len(missingParams) > 0 {
		return fmt.Errorf("missing required hub resolver params: %s", strings.Join(missingParams, ", "))
	}

	return nil
}
