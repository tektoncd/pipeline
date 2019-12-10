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

	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/git"
	"github.com/tektoncd/pipeline/pkg/termination"
	"knative.dev/pkg/logging"
)

var (
	fetchSpec              git.FetchSpec
	submodules             bool
	terminationMessagePath string
)

func init() {
	flag.StringVar(&fetchSpec.URL, "url", "", "Git origin URL to fetch")
	flag.StringVar(&fetchSpec.Revision, "revision", "", "The Git revision to make the repository HEAD")
	flag.StringVar(&fetchSpec.Path, "path", "", "Path of directory under which Git repository will be copied")
	flag.BoolVar(&fetchSpec.SSLVerify, "sslVerify", true, "Enable/Disable SSL verification in the git config")
	flag.BoolVar(&submodules, "submodules", true, "Initialize and fetch Git submodules")
	flag.UintVar(&fetchSpec.Depth, "depth", 1, "Perform a shallow clone to this depth")
	flag.StringVar(&terminationMessagePath, "terminationMessagePath", "/tekton/termination", "Location of file containing termination message")
}

func main() {
	flag.Parse()

	logger, _ := logging.NewLogger("", "git-init")
	defer func() {
		_ = logger.Sync()
	}()

	if err := git.Fetch(logger, fetchSpec); err != nil {
		logger.Fatalf("Error fetching git repository: %s", err)
	}
	if submodules {
		if err := git.SubmoduleFetch(logger, fetchSpec.Path); err != nil {
			logger.Fatalf("Error initializing or fetching the git submodules")
		}
	}

	commit, err := git.Commit(logger, fetchSpec.Revision, fetchSpec.Path)
	if err != nil {
		logger.Fatalf("Error parsing commit of git repository: %s", err)
	}
	output := []v1alpha1.PipelineResourceResult{
		{
			Key:   "commit",
			Value: commit,
		},
	}

	if err := termination.WriteMessage(terminationMessagePath, output); err != nil {
		logger.Fatalf("Error writing message to %s : %s", terminationMessagePath, err)
	}
}
