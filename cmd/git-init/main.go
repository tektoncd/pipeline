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
	url                    = flag.String("url", "", "The url of the Git repository to initialize.")
	revision               = flag.String("revision", "", "The Git revision to make the repository HEAD")
	path                   = flag.String("path", "", "Path of directory under which git repository will be copied")
	terminationMessagePath = flag.String("terminationMessagePath", "/dev/termination-log", "Location of file containing termination message")
	submodules             = flag.Bool("submodules", true, "Initialize and fetch git submodules")
)

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "git-init")
	defer logger.Sync()

	if err := git.Fetch(logger, *revision, *path, *url); err != nil {
		logger.Fatalf("Error fetching git repository: %s", err)
	}
	if *submodules {
		if err := git.SubmoduleFetch(logger, *path); err != nil {
			logger.Fatalf("Error initalizing or fetching the git submodules")
		}
	}

	commit, err := git.Commit(logger, *revision, *path)
	if err != nil {
		logger.Fatalf("Error parsing commit of git repository: %s", err)
	}
	output := []v1alpha1.PipelineResourceResult{
		{
			Key:   "commit",
			Value: commit,
		},
	}

	termination.WriteMessage(logger, *terminationMessagePath, output)
}
