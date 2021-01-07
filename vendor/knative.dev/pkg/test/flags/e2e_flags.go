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

// This file contains logic to encapsulate flags which are needed to specify
// what cluster, etc. to use for e2e tests.

package flags

import (
	"flag"
	"os"
	"time"
)

// TestEnvironment define the config that are needed to run the e2e tests.
type TestEnvironment struct {
	Cluster              string        // K8s cluster (defaults to cluster in kubeconfig)
	Namespace            string        // K8s namespace (blank by default, to be overwritten by test suite)
	IngressEndpoint      string        // Host to use for ingress endpoint
	ImageTemplate        string        // Template to build the image reference (defaults to {{.Repository}}/{{.Name}}:{{.Tag}})
	DockerRepo           string        // Docker repo (defaults to $KO_DOCKER_REPO)
	Tag                  string        // Tag for test images
	SpoofRequestInterval time.Duration // SpoofRequestInterval is the interval between requests in SpoofingClient
	SpoofRequestTimeout  time.Duration // SpoofRequestTimeout is the timeout for polling requests in SpoofingClient
}

var f *TestEnvironment

// InitFlags is for explicitly initializing the flags.
func InitFlags(flagset *flag.FlagSet) {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	f = new(TestEnvironment)

	flagset.StringVar(&f.Cluster, "cluster", "",
		"Provide the cluster to test against. Defaults to the current cluster in kubeconfig.")

	flagset.StringVar(&f.Namespace, "namespace", "",
		"Provide the namespace you would like to use for these tests.")

	flagset.StringVar(&f.IngressEndpoint, "ingressendpoint", "", "Provide a static endpoint url to the ingress server used during tests.")

	flagset.StringVar(&f.ImageTemplate, "imagetemplate", "{{.Repository}}/{{.Name}}:{{.Tag}}",
		"Provide a template to generate the reference to an image from the test. Defaults to `{{.Repository}}/{{.Name}}:{{.Tag}}`.")

	flagset.DurationVar(&f.SpoofRequestInterval, "spoofinterval", 1*time.Second,
		"Provide an interval between requests for the SpoofingClient")

	flagset.DurationVar(&f.SpoofRequestTimeout, "spooftimeout", 5*time.Minute,
		"Provide a request timeout for the SpoofingClient")

	defaultRepo := os.Getenv("KO_DOCKER_REPO")
	flagset.StringVar(&f.DockerRepo, "dockerrepo", defaultRepo,
		"Provide the uri of the docker repo you have uploaded the test image to using `uploadtestimage.sh`. Defaults to $KO_DOCKER_REPO")

	flagset.StringVar(&f.Tag, "tag", "latest", "Provide the version tag for the test images.")
}

// Flags returns the command line flags or defaults for settings in the user's
// environment. See TestEnvironment for a list of supported fields.
// Caller must call InitFlags() and flags.Parse() before calling Flags().
func Flags() *TestEnvironment {
	return f
}
