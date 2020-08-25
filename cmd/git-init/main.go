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
package main

import (
	"flag"
	"os"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/git"
	"github.com/tektoncd/pipeline/pkg/termination"
	"go.uber.org/zap"
)

var (
	fetchSpec              git.FetchSpec
	terminationMessagePath string
)

func init() {
	flag.StringVar(&fetchSpec.URL, "url", "", "Git origin URL to fetch")
	flag.StringVar(&fetchSpec.Revision, "revision", "", "The Git revision to make the repository HEAD")
	flag.StringVar(&fetchSpec.Refspec, "refspec", "", "The Git refspec to fetch the revision from (optional)")
	flag.StringVar(&fetchSpec.Path, "path", "", "Path of directory under which Git repository will be copied")
	flag.BoolVar(&fetchSpec.SSLVerify, "sslVerify", true, "Enable/Disable SSL verification in the git config")
	flag.BoolVar(&fetchSpec.Submodules, "submodules", true, "Initialize and fetch Git submodules")
	flag.UintVar(&fetchSpec.Depth, "depth", 1, "Perform a shallow clone to this depth")
	flag.StringVar(&terminationMessagePath, "terminationMessagePath", "/tekton/termination", "Location of file containing termination message")
}

func main() {
	flag.Parse()
	prod, _ := zap.NewProduction()
	logger := prod.Sugar()
	defer func() {
		_ = logger.Sync()
	}()

	if err := git.Fetch(logger, fetchSpec); err != nil {
		logger.Fatalf("Error fetching git repository: %s", err)
	}

	commit, err := git.ShowCommit(logger, "HEAD", fetchSpec.Path)
	if err != nil {
		logger.Fatalf("Error parsing revision %s of git repository: %s", fetchSpec.Revision, err)
	}
	resourceName := os.Getenv("TEKTON_RESOURCE_NAME")
	output := []v1beta1.PipelineResourceResult{
		{
			Key:   "commit",
			Value: commit,
			ResourceRef: &v1beta1.PipelineResourceRef{
				Name: resourceName,
			},
			ResourceName: resourceName,
		},
		{
			Key:   "url",
			Value: fetchSpec.URL,
			ResourceRef: &v1beta1.PipelineResourceRef{
				Name: resourceName,
			},
			ResourceName: resourceName,
		},
	}

	if err := termination.WriteMessage(terminationMessagePath, output); err != nil {
		logger.Fatalf("Error writing message to %s : %s", terminationMessagePath, err)
	}
}
