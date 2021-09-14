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

import (
	"flag"
	"fmt"
	"sort"
)

// FlagOptions holds options passed to the Tekton Pipeline controllers via command-line flags.
type FlagOptions struct {
	Images                        Images
	ExperimentalDisableResolution bool
}

func NewFlagOptions(fs *flag.FlagSet) *FlagOptions {
	fo := &FlagOptions{}
	fs.StringVar(&fo.Images.EntrypointImage, "entrypoint-image", "", "The container image containing our entrypoint binary.")
	fs.StringVar(&fo.Images.NopImage, "nop-image", "", "The container image used to stop sidecars")
	fs.StringVar(&fo.Images.GitImage, "git-image", "", "The container image containing our Git binary.")
	fs.StringVar(&fo.Images.KubeconfigWriterImage, "kubeconfig-writer-image", "", "The container image containing our kubeconfig writer binary.")
	fs.StringVar(&fo.Images.ShellImage, "shell-image", "", "The container image containing a shell")
	fs.StringVar(&fo.Images.ShellImageWin, "shell-image-win", "", "The container image containing a windows shell")
	fs.StringVar(&fo.Images.GsutilImage, "gsutil-image", "", "The container image containing gsutil")
	fs.StringVar(&fo.Images.PRImage, "pr-image", "", "The container image containing our PR binary.")
	fs.StringVar(&fo.Images.ImageDigestExporterImage, "imagedigest-exporter-image", "", "The container image containing our image digest exporter binary.")
	fs.BoolVar(&fo.ExperimentalDisableResolution, "experimental-disable-in-tree-resolution", false,
		"Disable resolution of taskrun and pipelinerun refs by the taskrun and pipelinerun reconcilers.")

	return fo
}

// Images holds the images reference for a number of container images used
// across tektoncd pipelines.
type Images struct {
	// EntrypointImage is container image containing our entrypoint binary.
	EntrypointImage string
	// NopImage is the container image used to kill sidecars.
	NopImage string
	// GitImage is the container image with Git that we use to implement the Git source step.
	GitImage string
	// KubeconfigWriterImage is the container image containing our kubeconfig writer binary.
	KubeconfigWriterImage string
	// ShellImage is the container image containing bash shell.
	ShellImage string
	// ShellImageWin is the container image containing powershell.
	ShellImageWin string
	// GsutilImage is the container image containing gsutil.
	GsutilImage string
	// PRImage is the container image that we use to implement the PR source step.
	PRImage string
	// ImageDigestExporterImage is the container image containing our image digest exporter binary.
	ImageDigestExporterImage string

	// NOTE: Make sure to add any new images to Validate below!
}

// Validate returns an error if any image is not set.
func (i Images) Validate() error {
	var unset []string
	for _, f := range []struct {
		v, name string
	}{
		{i.EntrypointImage, "entrypoint-image"},
		{i.NopImage, "nop-image"},
		{i.GitImage, "git-image"},
		{i.KubeconfigWriterImage, "kubeconfig-writer-image"},
		{i.ShellImage, "shell-image"},
		{i.ShellImageWin, "shell-image-win"},
		{i.GsutilImage, "gsutil-image"},
		{i.PRImage, "pr-image"},
		{i.ImageDigestExporterImage, "imagedigest-exporter-image"},
	} {
		if f.v == "" {
			unset = append(unset, f.name)
		}
	}
	if len(unset) > 0 {
		sort.Strings(unset)
		return fmt.Errorf("found unset image flags: %s", unset)
	}
	return nil
}
