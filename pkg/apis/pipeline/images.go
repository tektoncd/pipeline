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

package pipeline

// Images holds the images reference for a number of container images used
// across tektoncd pipelines.
type Images struct {
	// EntrypointImage is container image containing our entrypoint binary.
	EntrypointImage string
	// NopImage is the container image used to kill sidecars.
	NopImage string
	// AffinityAssistantImage is the container image used for the Affinity Assistant.
	AffinityAssistantImage string
	// GitImage is the container image with Git that we use to implement the Git source step.
	GitImage string
	// CredsImage is the container image used to initialize credentials before the build runs.
	CredsImage string
	// KubeconfigWriterImage is the container image containing our kubeconfig writer binary.
	KubeconfigWriterImage string
	// ShellImage is the container image containing bash shell.
	ShellImage string
	// GsutilImage is the container miage containing gsutil.
	GsutilImage string
	// BuildGCSFetcherImage is the container image containing our GCS fetcher binary.
	BuildGCSFetcherImage string
	// PRImage is the container image that we use to implement the PR source step.
	PRImage string
	// ImageDigestExporterImage is the container image containing our image digest exporter binary.
	ImageDigestExporterImage string
}
