/*
Copyright 2021 The Knative Authors

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
	"math"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ClientConfig holds the information about the environment and can be configured with flags
type ClientConfig struct {
	Cluster    string  // K8s cluster (defaults to cluster in kubeconfig)
	ServerURL  string  // ServerURL - The address of the Kubernetes API server. Overrides any value in kubeconfig.
	Burst      int     // Burst - Maximum burst for throttle.
	QPS        float64 // QPS - Maximum QPS to the server from the client.
	Kubeconfig string  // Kubeconfig - Path to a kubeconfig. Current casing is present for backwards compatibility
}

func (c *ClientConfig) InitFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Cluster, "cluster", "", "Defaults to the current cluster in kubeconfig.")

	fs.StringVar(&c.ServerURL, "server", "",
		"The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	fs.StringVar(&c.Kubeconfig, "kubeconfig", os.Getenv("KUBECONFIG"),
		"Path to a kubeconfig. Only required if out-of-cluster.")

	fs.IntVar(&c.Burst, "kube-api-burst", 0, "Maximum burst for throttle.")

	fs.Float64Var(&c.QPS, "kube-api-qps", 0, "Maximum QPS to the server from the client.")
}

func (c *ClientConfig) GetRESTConfig() (*rest.Config, error) {
	if c.Burst < 0 {
		return nil, fmt.Errorf("provided burst value %d must be > 0", c.Burst)
	}
	if c.QPS < 0 || c.QPS > math.MaxFloat32 {
		return nil, fmt.Errorf("provided QPS value %f must be >0 and <3.4+e38", c.QPS)
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}

	if c.Kubeconfig != "" {
		loadingRules.ExplicitPath = c.Kubeconfig
	}
	if c.Cluster != "" {
		overrides.Context = clientcmdapi.Context{Cluster: c.Cluster}
	} else if c.ServerURL != "" {
		overrides.ClusterInfo = clientcmdapi.Cluster{Server: c.ServerURL}
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		overrides,
	).ClientConfig()

	if err != nil {
		return nil, fmt.Errorf("failed to create client config: %w", err)
	}

	config.QPS = float32(c.QPS)
	config.Burst = c.Burst

	return config, nil
}
