/*
Copyright 2023 The Tekton Authors
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

package http

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

const (
	// LabelValueHttpResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueHttpResolverType string = "http"

	disabledError = "cannot handle resolution request, enable-http-resolver feature flag not true"

	// httpResolverName The name of the resolver
	httpResolverName = "Http"

	// configMapName is the http resolver's config map
	configMapName = "http-resolver-config"

	// default Timeout value when fetching http resources in seconds
	defaultHttpTimeoutValue = "1m"

	// default key in the HTTP password secret
	defaultBasicAuthSecretKey = "password"
)

// Resolver implements a framework.Resolver that can fetch files from an HTTP URL
type Resolver struct {
	kubeClient kubernetes.Interface
	logger     *zap.SugaredLogger
}

func (r *Resolver) Initialize(ctx context.Context) error {
	r.kubeClient = kubeclient.Get(ctx)
	r.logger = logging.FromContext(ctx)
	return nil
}

// GetName returns a string name to refer to this resolver by.
func (r *Resolver) GetName(context.Context) string {
	return httpResolverName
}

// GetConfigName returns the name of the http resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return configMapName
}

// GetSelector returns a map of labels to match requests to this resolver.
func (r *Resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueHttpResolverType,
	}
}

// ValidateParams ensures parameters from a request are as expected.
func (r *Resolver) ValidateParams(ctx context.Context, params []pipelinev1.Param) error {
	if r.isDisabled(ctx) {
		return errors.New(disabledError)
	}
	_, err := populateDefaultParams(ctx, params)
	if err != nil {
		return err
	}
	return nil
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *Resolver) Resolve(ctx context.Context, oParams []pipelinev1.Param) (framework.ResolvedResource, error) {
	if r.isDisabled(ctx) {
		return nil, errors.New(disabledError)
	}

	params, err := populateDefaultParams(ctx, oParams)
	if err != nil {
		return nil, err
	}

	return r.fetchHttpResource(ctx, params)
}

func (r *Resolver) isDisabled(ctx context.Context) bool {
	cfg := resolverconfig.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.EnableHttpResolver
}

// resolvedHttpResource wraps the data we want to return to Pipelines
type resolvedHttpResource struct {
	URL     string
	Content []byte
}

var _ framework.ResolvedResource = &resolvedHttpResource{}

// Data returns the bytes of our hard-coded Pipeline
func (rr *resolvedHttpResource) Data() []byte {
	return rr.Content
}

// Annotations returns any metadata needed alongside the data. None atm.
func (*resolvedHttpResource) Annotations() map[string]string {
	return nil
}

// RefSource is the source reference of the remote data that records where the remote
// file came from including the url, digest and the entrypoint.
func (rr *resolvedHttpResource) RefSource() *pipelinev1.RefSource {
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

func populateDefaultParams(ctx context.Context, params []pipelinev1.Param) (map[string]string, error) {
	paramsMap := make(map[string]string)
	for _, p := range params {
		paramsMap[p.Name] = p.Value.StringVal
	}

	var missingParams []string

	if _, ok := paramsMap[urlParam]; !ok {
		missingParams = append(missingParams, urlParam)
	} else {
		u, err := url.ParseRequestURI(paramsMap[urlParam])
		if err != nil {
			return nil, fmt.Errorf("cannot parse url %s: %w", paramsMap[urlParam], err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("url %s is not a valid http(s) url", paramsMap[urlParam])
		}
	}

	if username, ok := paramsMap[httpBasicAuthUsername]; ok {
		if _, ok := paramsMap[httpBasicAuthSecret]; !ok {
			return nil, fmt.Errorf("missing required param %s when using %s", httpBasicAuthSecret, httpBasicAuthUsername)
		}
		if username == "" {
			return nil, fmt.Errorf("value %s cannot be empty", httpBasicAuthUsername)
		}
	}

	if secret, ok := paramsMap[httpBasicAuthSecret]; ok {
		if _, ok := paramsMap[httpBasicAuthUsername]; !ok {
			return nil, fmt.Errorf("missing required param %s when using %s", httpBasicAuthUsername, httpBasicAuthSecret)
		}
		if secret == "" {
			return nil, fmt.Errorf("value %s cannot be empty", httpBasicAuthSecret)
		}
	}

	if len(missingParams) > 0 {
		return nil, fmt.Errorf("missing required http resolver params: %s", strings.Join(missingParams, ", "))
	}

	return paramsMap, nil
}

func makeHttpClient(ctx context.Context) (*http.Client, error) {
	conf := framework.GetResolverConfigFromContext(ctx)
	timeout, _ := time.ParseDuration(defaultHttpTimeoutValue)
	if v, ok := conf[timeoutKey]; ok {
		var err error
		timeout, err = time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("error parsing timeout value %s: %w", v, err)
		}
	}
	return &http.Client{
		Timeout: timeout,
	}, nil
}

func (r *Resolver) fetchHttpResource(ctx context.Context, params map[string]string) (framework.ResolvedResource, error) {
	var targetURL string
	var ok bool

	httpClient, err := makeHttpClient(ctx)
	if err != nil {
		return nil, err
	}

	if targetURL, ok = params[urlParam]; !ok {
		return nil, fmt.Errorf("missing required params: %s", urlParam)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("constructing request: %w", err)
	}

	// NOTE(chmouel): We already made sure that username and secret was specified by the user
	if secret, ok := params[httpBasicAuthSecret]; ok && secret != "" {
		if encodedSecret, err := r.getBasicAuthSecret(ctx, params); err != nil {
			return nil, err
		} else {
			req.Header.Set("Authorization", encodedSecret)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching URL: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("requested URL '%s' is not found", targetURL)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return &resolvedHttpResource{
		Content: body,
		URL:     targetURL,
	}, nil
}

func (r *Resolver) getBasicAuthSecret(ctx context.Context, params map[string]string) (string, error) {
	secretName := params[httpBasicAuthSecret]
	userName := params[httpBasicAuthUsername]
	tokenSecretKey := defaultBasicAuthSecretKey
	if v, ok := params[httpBasicAuthSecretKey]; ok {
		if v != "" {
			tokenSecretKey = v
		}
	}
	secretNS := common.RequestNamespace(ctx)
	secret, err := r.kubeClient.CoreV1().Secrets(secretNS).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			notFoundErr := fmt.Errorf("cannot get API token, secret %s not found in namespace %s", secretName, secretNS)
			r.logger.Info(notFoundErr)
			return "", notFoundErr
		}
		wrappedErr := fmt.Errorf("error reading API token from secret %s in namespace %s: %w", secretName, secretNS, err)
		r.logger.Info(wrappedErr)
		return "", wrappedErr
	}
	secretVal, ok := secret.Data[tokenSecretKey]
	if !ok {
		err := fmt.Errorf("cannot get API token, key %s not found in secret %s in namespace %s", tokenSecretKey, secretName, secretNS)
		r.logger.Info(err)
		return "", err
	}
	return "Basic " + base64.StdEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%s:%s", userName, secretVal))), nil
}
