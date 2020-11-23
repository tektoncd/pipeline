/*
Copyright 2020 The Tekton Authors

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
package environment

import (
	"flag"
	"fmt"
	"os"

	"github.com/tektoncd/pipeline/test/internal/clients"
	"k8s.io/apimachinery/pkg/version"
	knativetest "knative.dev/pkg/test"
)

var skipRootUserTests = false

func init() {
	flag.BoolVar(&skipRootUserTests, "skipRootUserTests", false, "Skip tests that require root user")
}

// Execution contains information about the current test execution and daemon
// under test
type Execution struct {
	ConfigPath  string
	ClusterName string
	Clients     *clients.Clients
	Info        *version.Info
	Platform    string

	koDockerRepo      string
	skipRootUserTests bool
}

// New creates a new Execution struct
// This is configured using the env client (see client.FromEnv)
func New() (*Execution, error) {
	c, err := clients.NewClients(knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster, "default")
	if err != nil {
		return nil, err
	}
	v, err := c.KubeClient.Kube.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	return &Execution{
		Clients:     c,
		Info:        v,
		Platform:    v.Platform,
		ConfigPath:  knativetest.Flags.Kubeconfig,
		ClusterName: knativetest.Flags.Cluster,

		skipRootUserTests: skipRootUserTests,
	}, nil
}

func (e *Execution) IsRunningOnGCP() bool {
	configFilePath := os.Getenv("GCP_SERVICE_ACCOUNT_KEY_PATH")
	return configFilePath != ""
}

func (e *Execution) SkipRootUserTests() bool {
	return e.skipRootUserTests
}

func (e *Execution) Print() {
	fmt.Fprintf(os.Stderr, "Using kubeconfig at `%s` with cluster `%s`\n", e.ConfigPath, e.ClusterName)
	fmt.Fprint(os.Stderr, "Cluster information\n")
	fmt.Fprintf(os.Stderr, "- Version: %s\n", e.Info.String())
	fmt.Fprintf(os.Stderr, "- BuildDate: %s\n", e.Info.BuildDate)
	fmt.Fprintf(os.Stderr, "- GoVersion: %s\n", e.Info.GoVersion)
	fmt.Fprintf(os.Stderr, "- Platform: %s\n", e.Info.Platform)
}
