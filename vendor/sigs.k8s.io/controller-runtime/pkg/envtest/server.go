/*
Copyright 2016 The Kubernetes Authors.

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

package envtest

import (
	"os"
	"path/filepath"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/testing_frameworks/integration"
)

// Default binary path for test framework
const (
	envKubeAPIServerBin    = "TEST_ASSET_KUBE_APISERVER"
	envEtcdBin             = "TEST_ASSET_ETCD"
	envKubectlBin          = "TEST_ASSET_KUBECTL"
	envKubebuilderPath     = "KUBEBUILDER_ASSETS"
	defaultKubebuilderPath = "/usr/local/kubebuilder/bin"
	StartTimeout           = 60
	StopTimeout            = 60
)

func defaultAssetPath(binary string) string {
	assetPath := os.Getenv(envKubebuilderPath)
	if assetPath == "" {
		assetPath = defaultKubebuilderPath
	}
	return filepath.Join(assetPath, binary)

}

// APIServerDefaultArgs are flags necessary to bring up apiserver.
// TODO: create test framework interface to append flag to default flags.
var defaultKubeAPIServerFlags = []string{
	"--etcd-servers={{ if .EtcdURL }}{{ .EtcdURL.String }}{{ end }}",
	"--cert-dir={{ .CertDir }}",
	"--insecure-port={{ if .URL }}{{ .URL.Port }}{{ end }}",
	"--insecure-bind-address={{ if .URL }}{{ .URL.Hostname }}{{ end }}",
	"--secure-port=0",
	"--admission-control=AlwaysAdmit",
}

// Environment creates a Kubernetes test environment that will start / stop the Kubernetes control plane and
// install extension APIs
type Environment struct {
	// ControlPlane is the ControlPlane including the apiserver and etcd
	ControlPlane integration.ControlPlane

	// Config can be used to talk to the apiserver
	Config *rest.Config

	// CRDs is a list of CRDs to install
	CRDs []*apiextensionsv1beta1.CustomResourceDefinition

	// CRDDirectoryPaths is a list of paths containing CRD yaml or json configs.
	CRDDirectoryPaths []string
}

// Stop stops a running server
func (te *Environment) Stop() error {
	return te.ControlPlane.Stop()
}

// Start starts a local Kubernetes server and updates te.ApiserverPort with the port it is listening on
func (te *Environment) Start() (*rest.Config, error) {
	te.ControlPlane = integration.ControlPlane{}
	te.ControlPlane.APIServer = &integration.APIServer{Args: defaultKubeAPIServerFlags}
	if os.Getenv(envKubeAPIServerBin) == "" {
		te.ControlPlane.APIServer.Path = defaultAssetPath("kube-apiserver")
	}
	if os.Getenv(envEtcdBin) == "" {
		te.ControlPlane.Etcd = &integration.Etcd{Path: defaultAssetPath("etcd")}
	}
	if os.Getenv(envKubectlBin) == "" {
		// we can't just set the path manually (it's behind a function), so set the environment variable instead
		if err := os.Setenv(envKubectlBin, defaultAssetPath("kubectl")); err != nil {
			return nil, err
		}
	}

	// Start the control plane - retry if it fails
	if err := te.ControlPlane.Start(); err != nil {
		return nil, err
	}

	// Create the *rest.Config for creating new clients
	te.Config = &rest.Config{
		Host: te.ControlPlane.APIURL().Host,
	}

	_, err := InstallCRDs(te.Config, CRDInstallOptions{
		Paths: te.CRDDirectoryPaths,
		CRDs:  te.CRDs,
	})
	return te.Config, err
}
