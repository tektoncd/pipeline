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
	"flag"
	"log"

	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"knative.dev/pkg/environment"
)

// ParseAndGetRESTConfigOrDie parses the rest config flags and creates a client or
// dies by calling log.Fatalf.
func ParseAndGetRESTConfigOrDie() *rest.Config {
	env := new(environment.ClientConfig)
	env.InitFlags(flag.CommandLine)
	klog.InitFlags(flag.CommandLine)
	flag.Parse()
	cfg, err := env.GetRESTConfig()
	if err != nil {
		log.Fatal("Error building kubeconfig: ", err)
	}
	return cfg
}

// GetRESTConfig returns a rest.Config to be used for kubernetes client creation.
// Deprecated: use environment.ClientConfig package
func GetRESTConfig(serverURL, kubeconfig string) (*rest.Config, error) {
	env := environment.ClientConfig{
		Kubeconfig: kubeconfig,
		ServerURL:  serverURL,
	}
	return env.GetRESTConfig()
}
