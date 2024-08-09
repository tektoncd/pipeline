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

package cluster

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	common "github.com/tektoncd/pipeline/pkg/resolution/common"
	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/yaml"
)

const (
	disabledError = "cannot handle resolution request, enable-cluster-resolver feature flag not true"

	// LabelValueClusterResolverType is the value to use for the
	// resolution.tekton.dev/type label on resource requests
	LabelValueClusterResolverType string = "cluster"

	// ClusterResolverName is the name that the cluster resolver should be
	// associated with
	ClusterResolverName string = "Cluster"

	configMapName = "cluster-resolver-config"
)

var _ framework.Resolver = &Resolver{}

// Resolver implements a framework.Resolver that can fetch resources from other namespaces.
//
// Deprecated: Use [github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/cluster.Resolver] instead.
type Resolver struct {
	pipelineClientSet clientset.Interface
}

// Initialize performs any setup required by the cluster resolver.
func (r *Resolver) Initialize(ctx context.Context) error {
	r.pipelineClientSet = pipelineclient.Get(ctx)
	return nil
}

// GetName returns the string name that the cluster resolver should be
// associated with.
func (r *Resolver) GetName(_ context.Context) string {
	return ClusterResolverName
}

// GetSelector returns the labels that resource requests are required to have for
// the cluster resolver to process them.
func (r *Resolver) GetSelector(_ context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: LabelValueClusterResolverType,
	}
}

// ValidateParams returns an error if the given parameter map is not
// valid for a resource request targeting the cluster resolver.
func (r *Resolver) ValidateParams(ctx context.Context, params []pipelinev1.Param) error {
	return ValidateParams(ctx, params)
}

// Resolve performs the work of fetching a resource from a namespace with the given
// parameters.
func (r *Resolver) Resolve(ctx context.Context, origParams []pipelinev1.Param) (framework.ResolvedResource, error) {
	return ResolveFromParams(ctx, origParams, r.pipelineClientSet)
}

func ResolveFromParams(ctx context.Context, origParams []pipelinev1.Param, pipelineClientSet clientset.Interface) (framework.ResolvedResource, error) {
	if isDisabled(ctx) {
		return nil, errors.New(disabledError)
	}

	logger := logging.FromContext(ctx)

	params, err := populateParamsWithDefaults(ctx, origParams)
	if err != nil {
		logger.Infof("cluster resolver parameter(s) invalid: %v", err)
		return nil, err
	}

	var data []byte
	var spec []byte
	var sha256Checksum []byte
	var uid string
	groupVersion := pipelinev1.SchemeGroupVersion.String()

	switch params[KindParam] {
	case "stepaction":
		stepaction, err := pipelineClientSet.TektonV1beta1().StepActions(params[NamespaceParam]).Get(ctx, params[NameParam], metav1.GetOptions{})
		if err != nil {
			logger.Infof("failed to load stepaction %s from namespace %s: %v", params[NameParam], params[NamespaceParam], err)
			return nil, err
		}
		uid, data, sha256Checksum, spec, err = fetchStepaction(ctx, pipelinev1beta1.SchemeGroupVersion.String(), stepaction, params)
		if err != nil {
			return nil, err
		}
	case "task":
		task, err := pipelineClientSet.TektonV1().Tasks(params[NamespaceParam]).Get(ctx, params[NameParam], metav1.GetOptions{})
		if err != nil {
			logger.Infof("failed to load task %s from namespace %s: %v", params[NameParam], params[NamespaceParam], err)
			return nil, err
		}
		uid, data, sha256Checksum, spec, err = fetchTask(ctx, groupVersion, task, params)
		if err != nil {
			return nil, err
		}
	case "pipeline":
		pipeline, err := pipelineClientSet.TektonV1().Pipelines(params[NamespaceParam]).Get(ctx, params[NameParam], metav1.GetOptions{})
		if err != nil {
			logger.Infof("failed to load pipeline %s from namespace %s: %v", params[NameParam], params[NamespaceParam], err)
			return nil, err
		}
		uid, data, sha256Checksum, spec, err = fetchPipeline(ctx, groupVersion, pipeline, params)
		if err != nil {
			return nil, err
		}
	default:
		logger.Infof("unknown or invalid resource kind %s", params[KindParam])
		return nil, fmt.Errorf("unknown or invalid resource kind %s", params[KindParam])
	}

	return &ResolvedClusterResource{
		Content:    data,
		Spec:       spec,
		Name:       params[NameParam],
		Namespace:  params[NamespaceParam],
		Identifier: fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s@%s", groupVersion, params[NamespaceParam], params[KindParam], params[NameParam], uid),
		Checksum:   sha256Checksum,
	}, nil
}

var _ framework.ConfigWatcher = &Resolver{}

// GetConfigName returns the name of the cluster resolver's configmap.
func (r *Resolver) GetConfigName(context.Context) string {
	return configMapName
}

// ResolvedClusterResource implements framework.ResolvedResource and returns
// the resolved file []byte data and an annotation map for any metadata.
type ResolvedClusterResource struct {
	// Content is the actual resolved resource data.
	Content []byte
	// Spec is the data in the resolved task/pipeline CRD spec.
	Spec []byte
	// Name is the resolved resource name in the cluster
	Name string
	// Namespace is the namespace in the cluster under which the resolved resource was created.
	Namespace string
	// Identifier is the unique identifier for the resource in the cluster.
	// It is in the format of <resource uri>@<uid>.
	// Resource URI is the namespace-scoped uri i.e. /apis/GROUP/VERSION/namespaces/NAMESPACE/RESOURCETYPE/NAME.
	// https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-uris
	Identifier string
	// Sha256 Checksum of the cluster resource
	Checksum []byte
}

var _ framework.ResolvedResource = &ResolvedClusterResource{}

// Data returns the bytes of the file resolved from git.
func (r *ResolvedClusterResource) Data() []byte {
	return r.Content
}

// Annotations returns the metadata that accompanies the resource fetched from the cluster.
func (r *ResolvedClusterResource) Annotations() map[string]string {
	return map[string]string{
		ResourceNameAnnotation:      r.Name,
		ResourceNamespaceAnnotation: r.Namespace,
	}
}

// RefSource is the source reference of the remote data that records where the remote
// file came from including the url, digest and the entrypoint.
func (r ResolvedClusterResource) RefSource() *pipelinev1.RefSource {
	return &pipelinev1.RefSource{
		URI: r.Identifier,
		Digest: map[string]string{
			"sha256": hex.EncodeToString(r.Checksum),
		},
	}
}

func populateParamsWithDefaults(ctx context.Context, origParams []pipelinev1.Param) (map[string]string, error) {
	conf := framework.GetResolverConfigFromContext(ctx)

	paramsMap := make(map[string]pipelinev1.ParamValue)
	for _, p := range origParams {
		paramsMap[p.Name] = p.Value
	}

	params := make(map[string]string)

	var missingParams []string

	if pKind, ok := paramsMap[KindParam]; !ok || pKind.StringVal == "" {
		if kindVal, ok := conf[DefaultKindKey]; !ok {
			missingParams = append(missingParams, KindParam)
		} else {
			params[KindParam] = kindVal
		}
	} else {
		params[KindParam] = pKind.StringVal
	}
	if kindVal, ok := params[KindParam]; ok && kindVal != "task" && kindVal != "pipeline" {
		return nil, fmt.Errorf("unknown or unsupported resource kind '%s'", kindVal)
	}

	if pName, ok := paramsMap[NameParam]; !ok || pName.StringVal == "" {
		missingParams = append(missingParams, NameParam)
	} else {
		params[NameParam] = pName.StringVal
	}

	if pNS, ok := paramsMap[NamespaceParam]; !ok || pNS.StringVal == "" {
		if nsVal, ok := conf[DefaultNamespaceKey]; !ok {
			missingParams = append(missingParams, NamespaceParam)
		} else {
			params[NamespaceParam] = nsVal
		}
	} else {
		params[NamespaceParam] = pNS.StringVal
	}

	if len(missingParams) > 0 {
		return nil, fmt.Errorf("missing required cluster resolver params: %s", strings.Join(missingParams, ", "))
	}

	if conf[BlockedNamespacesKey] != "" && isInCommaSeparatedList(params[NamespaceParam], conf[BlockedNamespacesKey]) {
		return nil, fmt.Errorf("access to specified namespace %s is blocked", params[NamespaceParam])
	}

	if conf[AllowedNamespacesKey] != "" && isInCommaSeparatedList(params[NamespaceParam], conf[AllowedNamespacesKey]) {
		return params, nil
	}

	if conf[BlockedNamespacesKey] != "" && conf[BlockedNamespacesKey] == "*" {
		return nil, errors.New("only explicit allowed access to namespaces is allowed")
	}

	if conf[AllowedNamespacesKey] != "" && !isInCommaSeparatedList(params[NamespaceParam], conf[AllowedNamespacesKey]) {
		return nil, fmt.Errorf("access to specified namespace %s is not allowed", params[NamespaceParam])
	}

	return params, nil
}

func isInCommaSeparatedList(checkVal string, commaList string) bool {
	for _, s := range strings.Split(commaList, ",") {
		if s == checkVal {
			return true
		}
	}
	return false
}

func isDisabled(ctx context.Context) bool {
	cfg := resolverconfig.FromContextOrDefaults(ctx)
	return !cfg.FeatureFlags.EnableClusterResolver
}

func ValidateParams(ctx context.Context, params []pipelinev1.Param) error {
	if isDisabled(ctx) {
		return errors.New(disabledError)
	}

	_, err := populateParamsWithDefaults(ctx, params)
	return err
}

func fetchStepaction(ctx context.Context, groupVersion string, stepaction *pipelinev1beta1.StepAction, params map[string]string) (string, []byte, []byte, []byte, error) {
	logger := logging.FromContext(ctx)
	uid := string(stepaction.UID)
	stepaction.Kind = "StepAction"
	stepaction.APIVersion = groupVersion
	data, err := yaml.Marshal(stepaction)
	if err != nil {
		logger.Infof("failed to marshal stepaction %s from namespace %s: %v", params[NameParam], params[NamespaceParam], err)
		return "", nil, nil, nil, err
	}
	sha256Checksum, err := stepaction.Checksum()
	if err != nil {
		return "", nil, nil, nil, err
	}

	spec, err := yaml.Marshal(stepaction.Spec)
	if err != nil {
		logger.Infof("failed to marshal the spec of the task %s from namespace %s: %v", params[NameParam], params[NamespaceParam], err)
		return "", nil, nil, nil, err
	}
	return uid, data, sha256Checksum, spec, nil
}

func fetchTask(ctx context.Context, groupVersion string, task *pipelinev1.Task, params map[string]string) (string, []byte, []byte, []byte, error) {
	logger := logging.FromContext(ctx)
	uid := string(task.UID)
	task.Kind = "Task"
	task.APIVersion = groupVersion
	data, err := yaml.Marshal(task)
	if err != nil {
		logger.Infof("failed to marshal task %s from namespace %s: %v", params[NameParam], params[NamespaceParam], err)
		return "", nil, nil, nil, err
	}
	sha256Checksum, err := task.Checksum()
	if err != nil {
		return "", nil, nil, nil, err
	}

	spec, err := yaml.Marshal(task.Spec)
	if err != nil {
		logger.Infof("failed to marshal the spec of the task %s from namespace %s: %v", params[NameParam], params[NamespaceParam], err)
		return "", nil, nil, nil, err
	}
	return uid, data, sha256Checksum, spec, nil
}

func fetchPipeline(ctx context.Context, groupVersion string, pipeline *pipelinev1.Pipeline, params map[string]string) (string, []byte, []byte, []byte, error) {
	logger := logging.FromContext(ctx)
	uid := string(pipeline.UID)
	pipeline.Kind = "Pipeline"
	pipeline.APIVersion = groupVersion
	data, err := yaml.Marshal(pipeline)
	if err != nil {
		logger.Infof("failed to marshal pipeline %s from namespace %s: %v", params[NameParam], params[NamespaceParam], err)
		return "", nil, nil, nil, err
	}

	sha256Checksum, err := pipeline.Checksum()
	if err != nil {
		return "", nil, nil, nil, err
	}

	spec, err := yaml.Marshal(pipeline.Spec)
	if err != nil {
		logger.Infof("failed to marshal the spec of the pipeline %s from namespace %s: %v", params[NameParam], params[NamespaceParam], err)
		return "", nil, nil, nil, err
	}
	return uid, data, sha256Checksum, spec, nil
}
