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

package git

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
)

const (
	// DefaultTimeoutKey is the configuration field name for controlling
	// the maximum duration of a resolution request for a file from git.
	DefaultTimeoutKey = "fetch-timeout"

	// DefaultURLKey is the configuration field name for controlling
	// the git url to fetch the remote resource from.
	DefaultURLKey = "default-url"

	// DefaultRevisionKey is the configuration field name for controlling
	// the revision to fetch the remote resource from.
	DefaultRevisionKey = "default-revision"

	// DefaultOrgKey is the configuration field name for setting a default organization when using the SCM API.
	DefaultOrgKey = "default-org"

	// ServerURLKey is the config map key for the SCM provider URL
	ServerURLKey = "server-url"
	// SCMTypeKey is the config map key for the SCM provider type
	SCMTypeKey = "scm-type"
	// APISecretNameKey is the config map key for the token secret's name
	APISecretNameKey = "api-token-secret-name"
	// APISecretKeyKey is the config map key for the containing the token within the token secret
	APISecretKeyKey = "api-token-secret-key"
	// APISecretNamespaceKey is the config map key for the token secret's namespace
	APISecretNamespaceKey = "api-token-secret-namespace"
)

type GitResolverConfig map[string]ScmConfig

type ScmConfig struct {
	Timeout            string `json:"fetch-timeout"`
	URL                string `json:"default-url"`
	Revision           string `json:"default-revision"`
	Org                string `json:"default-org"`
	ServerURL          string `json:"server-url"`
	SCMType            string `json:"scm-type"`
	GitToken           string `json:"git-token"`
	APISecretName      string `json:"api-token-secret-name"`
	APISecretKey       string `json:"api-token-secret-key"`
	APISecretNamespace string `json:"api-token-secret-namespace"`
}

func GetGitResolverConfig(ctx context.Context) (GitResolverConfig, error) {
	var scmConfig interface{} = &ScmConfig{}
	structType := reflect.TypeOf(scmConfig).Elem()
	gitResolverConfig := map[string]ScmConfig{}
	conf := framework.GetResolverConfigFromContext(ctx)
	for key, value := range conf {
		var configIdentifier, configKey string
		splittedKeyName := strings.Split(key, ".")
		switch len(splittedKeyName) {
		case 2:
			configKey = splittedKeyName[1]
			configIdentifier = splittedKeyName[0]
		case 1:
			configKey = key
			configIdentifier = "default"
		default:
			return nil, fmt.Errorf("key %s passed in git resolver configmap is invalid", key)
		}
		_, ok := gitResolverConfig[configIdentifier]
		if !ok {
			gitResolverConfig[configIdentifier] = ScmConfig{}
		}
		for i := range structType.NumField() {
			field := structType.Field(i)
			fieldName := field.Name
			jsonTag := field.Tag.Get("json")
			if configKey == jsonTag {
				tokenDetails := gitResolverConfig[configIdentifier]
				var scm interface{} = &tokenDetails
				structValue := reflect.ValueOf(scm).Elem()
				structValue.FieldByName(fieldName).SetString(value)
				gitResolverConfig[configIdentifier] = structValue.Interface().(ScmConfig)
			}
		}
	}
	return gitResolverConfig, nil
}
