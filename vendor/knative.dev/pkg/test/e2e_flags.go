/*
Copyright 2018 The Knative Authors

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

package test

import (
	"bytes"
	"flag"
	"os"
	"os/user"
	"path"
	"sync"
	"text/template"

	"knative.dev/pkg/test/logging"
)

const (
	// The recommended default log level https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md
	klogDefaultLogLevel = "2"
)

var (
	flagsSetupOnce = &sync.Once{}
	klogFlags      = flag.NewFlagSet("klog", flag.ExitOnError)
	// Flags holds the command line flags or defaults for settings in the user's environment.
	// See EnvironmentFlags for a list of supported fields.
	Flags = initializeFlags()
)

// EnvironmentFlags define the flags that are needed to run the e2e tests.
type EnvironmentFlags struct {
	Cluster         string // K8s cluster (defaults to cluster in kubeconfig)
	Kubeconfig      string // Path to kubeconfig (defaults to ./kube/config)
	Namespace       string // K8s namespace (blank by default, to be overwritten by test suite)
	IngressEndpoint string // Host to use for ingress endpoint
	ImageTemplate   string // Template to build the image reference (defaults to {{.Repository}}/{{.Name}}:{{.Tag}})
	DockerRepo      string // Docker repo (defaults to $KO_DOCKER_REPO)
	Tag             string // Tag for test images
}

func initializeFlags() *EnvironmentFlags {
	var f EnvironmentFlags
	flag.StringVar(&f.Cluster, "cluster", "",
		"Provide the cluster to test against. Defaults to the current cluster in kubeconfig.")

	// Use KUBECONFIG if available
	defaultKubeconfig := os.Getenv("KUBECONFIG")

	// If KUBECONFIG env var isn't set then look for $HOME/.kube/config
	if defaultKubeconfig == "" {
		if usr, err := user.Current(); err == nil {
			defaultKubeconfig = path.Join(usr.HomeDir, ".kube/config")
		}
	}

	// Allow for --kubeconfig on the cmd line to override the above logic
	flag.StringVar(&f.Kubeconfig, "kubeconfig", defaultKubeconfig,
		"Provide the path to the `kubeconfig` file you'd like to use for these tests. The `current-context` will be used.")

	flag.StringVar(&f.Namespace, "namespace", "",
		"Provide the namespace you would like to use for these tests.")

	flag.StringVar(&f.IngressEndpoint, "ingressendpoint", "", "Provide a static endpoint url to the ingress server used during tests.")

	flag.StringVar(&f.ImageTemplate, "imagetemplate", "{{.Repository}}/{{.Name}}:{{.Tag}}",
		"Provide a template to generate the reference to an image from the test. Defaults to `{{.Repository}}/{{.Name}}:{{.Tag}}`.")

	defaultRepo := os.Getenv("KO_DOCKER_REPO")
	flag.StringVar(&f.DockerRepo, "dockerrepo", defaultRepo,
		"Provide the uri of the docker repo you have uploaded the test image to using `uploadtestimage.sh`. Defaults to $KO_DOCKER_REPO")

	flag.StringVar(&f.Tag, "tag", "latest", "Provide the version tag for the test images.")

	return &f
}

// TODO(coryrc): Remove once other repos are moved to call logging.InitializeLogger() directly
func SetupLoggingFlags() {
	logging.InitializeLogger()
}

// ImagePath is a helper function to transform an image name into an image reference that can be pulled.
func ImagePath(name string) string {
	tpl, err := template.New("image").Parse(Flags.ImageTemplate)
	if err != nil {
		panic("could not parse image template: " + err.Error())
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, struct {
		Repository string
		Name       string
		Tag        string
	}{
		Repository: Flags.DockerRepo,
		Name:       name,
		Tag:        Flags.Tag,
	}); err != nil {
		panic("could not apply the image template: " + err.Error())
	}
	return buf.String()
}
