/*
Copyright 2020 The Knative Authors

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

package injection

import (
	"errors"
	"flag"
	"log"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

// Environment holds the config for flag based config.
type Environment struct {
	// ServerURL - The address of the Kubernetes API server. Overrides any value in kubeconfig.
	ServerURL string
	// Kubeconfig - Path to a kubeconfig.
	Kubeconfig string // Note: named Kubeconfig because of legacy reasons vs KubeConfig.
	// Burst - Maximum burst for throttle.
	Burst int
	// QPS - Maximum QPS to the server from the client.
	QPS float64
}

var (
	env  *Environment
	once sync.Once
)

func Flags() *Environment {
	once.Do(func() {
		env = new(Environment)

		flag.StringVar(&env.ServerURL, "server", "",
			"The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

		flag.StringVar(&env.Kubeconfig, "kubeconfig", os.Getenv("KUBECONFIG"),
			"Path to a kubeconfig. Only required if out-of-cluster.")

		flag.IntVar(&env.Burst, "kube-api-burst", 0, "Maximum burst for throttle.")

		flag.Float64Var(&env.QPS, "kube-api-qps", 0, "Maximum QPS to the server from the client.")
	})
	return env
}

// ParseAndGetRESTConfigOrDie parses the rest config flags and creates a client or
// dies by calling log.Fatalf.
func ParseAndGetRESTConfigOrDie() *rest.Config {
	env := Flags()

	klog.InitFlags(flag.CommandLine)
	flag.Parse()
	if env.Burst < 0 {
		log.Fatal("Invalid burst value", env.Burst)
	}
	if env.QPS < 0 || env.QPS > math.MaxFloat32 {
		log.Fatal("Invalid QPS value", env.QPS)
	}

	cfg, err := GetRESTConfig(env.ServerURL, env.Kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig: ", err)
	}

	cfg.Burst = env.Burst
	cfg.QPS = float32(env.QPS)

	return cfg
}

// GetRESTConfig returns a rest.Config to be used for kubernetes client creation.
// It does so in the following order:
//   1. Use the passed kubeconfig/serverURL.
//   2. Fallback to the KUBECONFIG environment variable.
//   3. Fallback to in-cluster config.
//   4. Fallback to the ~/.kube/config.
func GetRESTConfig(serverURL, kubeconfig string) (*rest.Config, error) {
	// If we have an explicit indication of where the kubernetes config lives, read that.
	if kubeconfig != "" {
		c, err := clientcmd.BuildConfigFromFlags(serverURL, kubeconfig)
		if err != nil {
			return nil, err
		}
		return c, nil
	}

	// If not, try the in-cluster config.
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}

	// If no in-cluster config, try the default location in the user's home directory.
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, errors.New("could not create a valid kubeconfig")
}
