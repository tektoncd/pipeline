/*
Copyright 2019 The Tekton Authors

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
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

// Resource represents a cluster configuration (kubeconfig)
// that can be accessed by tasks in the pipeline
type Resource struct {
	Name string                        `json:"name"`
	Type resource.PipelineResourceType `json:"type"`
	// URL must be a host string
	URL      string `json:"url"`
	Revision string `json:"revision"`
	// Server requires Basic authentication
	Username  string `json:"username"`
	Password  string `json:"password"`
	Namespace string `json:"namespace"`
	// Server requires Bearer authentication. This client will not attempt to use
	// refresh tokens for an OAuth2 flow.
	// Token overrides userame and password
	Token string `json:"token"`
	// Server should be accessed without verifying the TLS certificate. For testing only.
	Insecure bool
	// CAData holds PEM-encoded bytes (typically read from a root certificates bundle).
	// CAData takes precedence over CAFile
	CAData []byte `json:"cadata"`
	// ClientKeyData contains PEM-encoded data from a client key file for TLS.
	ClientKeyData []byte `json:"clientKeyData"`
	// ClientCertificateData contains PEM-encoded data from a client cert file for TLS.
	ClientCertificateData []byte `json:"clientCertificateData"`
	//Secrets holds a struct to indicate a field name and corresponding secret name to populate it
	Secrets []resource.SecretParam `json:"secrets"`

	KubeconfigWriterImage string `json:"-"`
	ShellImage            string `json:"-"`
}

// NewResource create a new k8s cluster resource to pass to a pipeline task
func NewResource(name string, kubeconfigWriterImage, shellImage string, r *resource.PipelineResource) (*Resource, error) {
	if r.Spec.Type != resource.PipelineResourceTypeCluster {
		return nil, fmt.Errorf("cluster.Resource: Cannot create a Cluster resource from a %s Pipeline Resource", r.Spec.Type)
	}
	clusterResource := Resource{
		Type:                  r.Spec.Type,
		KubeconfigWriterImage: kubeconfigWriterImage,
		ShellImage:            shellImage,
		Name:                  name,
	}
	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "URL"):
			clusterResource.URL = param.Value
		case strings.EqualFold(param.Name, "Revision"):
			clusterResource.Revision = param.Value
		case strings.EqualFold(param.Name, "Username"):
			clusterResource.Username = param.Value
		case strings.EqualFold(param.Name, "Namespace"):
			clusterResource.Namespace = param.Value
		case strings.EqualFold(param.Name, "Password"):
			clusterResource.Password = param.Value
		case strings.EqualFold(param.Name, "Token"):
			clusterResource.Token = param.Value
		case strings.EqualFold(param.Name, "Insecure"):
			b, _ := strconv.ParseBool(param.Value)
			clusterResource.Insecure = b
		case strings.EqualFold(param.Name, "CAData"):
			if param.Value != "" {
				sDec, _ := b64.StdEncoding.DecodeString(param.Value)
				clusterResource.CAData = sDec
			}
		case strings.EqualFold(param.Name, "ClientKeyData"):
			if param.Value != "" {
				sDec, _ := b64.StdEncoding.DecodeString(param.Value)
				clusterResource.ClientKeyData = sDec
			}
		case strings.EqualFold(param.Name, "ClientCertificateData"):
			if param.Value != "" {
				sDec, _ := b64.StdEncoding.DecodeString(param.Value)
				clusterResource.ClientCertificateData = sDec
			}
		}
	}
	clusterResource.Secrets = r.Spec.SecretParams

	if len(clusterResource.CAData) == 0 {
		clusterResource.Insecure = true
		for _, secret := range clusterResource.Secrets {
			if strings.EqualFold(secret.FieldName, "CAData") {
				clusterResource.Insecure = false
				break
			}
		}
	}

	return &clusterResource, nil
}

// GetName returns the name of the resource
func (s Resource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "cluster"
func (s Resource) GetType() resource.PipelineResourceType {
	return resource.PipelineResourceTypeCluster
}

// GetURL returns the url to be used with this resource
func (s *Resource) GetURL() string {
	return s.URL
}

// Replacements is used for template replacement on a ClusterResource inside of a Taskrun.
func (s *Resource) Replacements() map[string]string {
	return map[string]string{
		"name":                  s.Name,
		"type":                  s.Type,
		"url":                   s.URL,
		"revision":              s.Revision,
		"username":              s.Username,
		"password":              s.Password,
		"namespace":             s.Namespace,
		"token":                 s.Token,
		"insecure":              strconv.FormatBool(s.Insecure),
		"cadata":                string(s.CAData),
		"clientKeyData":         string(s.ClientKeyData),
		"clientCertificateData": string(s.ClientCertificateData),
	}
}

func (s Resource) String() string {
	json, _ := json.Marshal(s)
	return string(json)
}

// GetOutputTaskModifier returns a No-op TaskModifier.
func (s *Resource) GetOutputTaskModifier(_ *v1beta1.TaskSpec, _ string) (v1beta1.TaskModifier, error) {
	return &v1beta1.InternalTaskModifier{}, nil
}

// GetInputTaskModifier returns the TaskModifier to be used when this resource is an input.
func (s *Resource) GetInputTaskModifier(ts *v1beta1.TaskSpec, path string) (v1beta1.TaskModifier, error) {
	var envVars []corev1.EnvVar
	for _, sec := range s.Secrets {
		ev := corev1.EnvVar{
			Name: strings.ToUpper(sec.FieldName),
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: sec.SecretName,
					},
					Key: sec.SecretKey,
				},
			},
		}
		envVars = append(envVars, ev)
	}
	step := v1beta1.Step{Container: corev1.Container{
		Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("kubeconfig"),
		Image:   s.KubeconfigWriterImage,
		Command: []string{"/ko-app/kubeconfigwriter"},
		Args: []string{
			"-clusterConfig", s.String(),
		},
		Env: envVars,
	}}
	return &v1beta1.InternalTaskModifier{
		StepsToPrepend: []v1beta1.Step{
			step,
		},
	}, nil
}
