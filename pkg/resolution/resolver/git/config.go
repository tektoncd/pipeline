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
	"strconv"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/logging"
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

	// ConfigBackoffDuration is the configuration field name for controlling
	// the initial duration of a backoff when a git resolution fails
	ConfigBackoffDuration  = "backoff-duration"
	DefaultBackoffDuration = 2 * time.Second
	// ConfigBackoffFactor is the configuration field name for controlling
	// the factor by which successive backoffs will increase when a git
	// resolution fails
	ConfigBackoffFactor  = "backoff-factor"
	DefaultBackoffFactor = 2.0
	// ConfigBackoffJitter is the configuration field name for controlling
	// the randomness applied to backoff durations when a git resolution fails
	ConfigBackoffJitter  = "backoff-jitter"
	DefaultBackoffJitter = 0.1
	// ConfigBackoffSteps is the configuration field name for controlling
	// the total number of resolution attempts when a git resolution fails
	ConfigBackoffSteps  = "backoff-steps"
	DefaultBackoffSteps = 2
	// ConfigBackoffCap is the configuration field name for controlling
	// the maximum duration to try when backing off
	ConfigBackoffCap  = "backoff-cap"
	DefaultBackoffCap = 10 * time.Second
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

// GetGitResolverBackoff returns a wait.Backoff to be used when retrying
// git resolution requests. This can be configured with the backoff-duration,
// backoff-factor, backoff-jitter, backoff-steps, and backoff-cap fields in
// the git-resolver-config ConfigMap. Invalid values are logged as warnings
// and replaced with their defaults so that retry behavior is preserved.
func GetGitResolverBackoff(ctx context.Context) wait.Backoff {
	logger := logging.FromContext(ctx)
	conf := framework.GetResolverConfigFromContext(ctx)

	backoff := wait.Backoff{
		Duration: DefaultBackoffDuration,
		Factor:   DefaultBackoffFactor,
		Jitter:   DefaultBackoffJitter,
		Steps:    DefaultBackoffSteps,
		Cap:      DefaultBackoffCap,
	}
	if v, ok := conf[ConfigBackoffDuration]; ok {
		duration, err := time.ParseDuration(v)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffDuration, v, DefaultBackoffDuration, err)
		case duration < 0:
			logger.Warnf("invalid %s value %q: must not be negative, using default %v", ConfigBackoffDuration, v, DefaultBackoffDuration)
		default:
			backoff.Duration = duration
		}
	}
	if v, ok := conf[ConfigBackoffFactor]; ok {
		factor, err := strconv.ParseFloat(v, 64)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffFactor, v, DefaultBackoffFactor, err)
		case factor < 0:
			logger.Warnf("invalid %s value %q: must not be negative, using default %v", ConfigBackoffFactor, v, DefaultBackoffFactor)
		default:
			backoff.Factor = factor
		}
	}
	if v, ok := conf[ConfigBackoffJitter]; ok {
		jitter, err := strconv.ParseFloat(v, 64)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffJitter, v, DefaultBackoffJitter, err)
		case jitter < 0:
			logger.Warnf("invalid %s value %q: must not be negative, using default %v", ConfigBackoffJitter, v, DefaultBackoffJitter)
		default:
			backoff.Jitter = jitter
		}
	}
	if v, ok := conf[ConfigBackoffSteps]; ok {
		steps, err := strconv.Atoi(v)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffSteps, v, DefaultBackoffSteps, err)
		case steps < 1:
			logger.Warnf("invalid %s value %q: must be at least 1, using default %v", ConfigBackoffSteps, v, DefaultBackoffSteps)
		default:
			backoff.Steps = steps
		}
	}
	if v, ok := conf[ConfigBackoffCap]; ok {
		cap, err := time.ParseDuration(v)
		switch {
		case err != nil:
			logger.Warnf("invalid %s value %q, using default %v: %v", ConfigBackoffCap, v, DefaultBackoffCap, err)
		case cap < 0:
			logger.Warnf("invalid %s value %q: must not be negative, using default %v", ConfigBackoffCap, v, DefaultBackoffCap)
		default:
			backoff.Cap = cap
		}
	}

	return backoff
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
