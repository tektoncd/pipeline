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
package main

import (
	"flag"
	"fmt"
	"github.com/knative/build-pipeline/cmd/git-init/utils"
	"github.com/knative/pkg/logging"
	"os"
	"strings"
)

var (
	url      = flag.String("url", "", "The url of the Git repository to initialize.")
	revision = flag.String("revision", "", "The Git revision to make the repository HEAD")
	path     = flag.String("path", "", "Path of directory under which git repository will be copied")
)

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "git-init")
	defer logger.Sync()

	// HACK HACK HACK
	// Git seems to ignore $HOME/.ssh and look in /root/.ssh for unknown reasons.
	// As a workaround, symlink /root/.ssh to where we expect the $HOME to land.
	// This means SSH auth only works for our built-in git support, and not
	// custom steps.
	err := os.Symlink("/builder/home/.ssh", "/root/.ssh")
	if err != nil {
		logger.Fatalf("Unexpected error creating symlink: %v", err)
	}
	if *revision == "" {
		*revision = "master"
	}
	if *path != "" {
		utils.RunOrFail(logger, "git", "init", *path)
		if _, err := os.Stat(*path); os.IsNotExist(err) {
			if err := os.Mkdir(*path, os.ModePerm); err != nil {
				logger.Debugf("Creating directory at path %s", *path)
			}
		}
		if err := os.Chdir(*path); err != nil {
			logger.Fatalf("Failed to change directory with path %s; err %v", path, err)
		}
	} else {
		utils.RunAndError(logger, "git", "init")
	}

	utils.RunAndError(logger, "git", "remote", "add", "origin", *url)
	utils.RunOrFail(logger, "git", "fetch", "--depth=1", "--recurse-submodules=yes", "origin", *revision)
	utils.RunOrFail(logger, "git", "reset", "--hard", "FETCH_HEAD")

	logger.Infof("Successfully cloned %q @ %q in path %q", *url, *revision, *path)


	if pullRefs := os.Getenv("PULL_REFS"); pullRefs != "" {
		logger.Infof("Found PULL_REFS=%s", pullRefs)
		branchSHAs, err := utils.ParsePullRefs(pullRefs)
		if err != nil {
			logger.Fatalf("Error %v parsing PULL_REFS=%s", err, pullRefs)
		}
		if len(branchSHAs.ToMerge) == 0 {
			logger.Fatalf("No commits to merge")
		}
		mergeStrs := make([]string, 0)
		for k, v := range branchSHAs.ToMerge {
			mergeStrs = append(mergeStrs, fmt.Sprintf("%s (from %s) ", v, k))
		}
		logger.Infof("Merging %s to %s (at revision %s)", strings.Join(mergeStrs, ", "), branchSHAs.BaseBranch,
			branchSHAs.BaseSha)
		utils.FetchAndMergeSHAs(branchSHAs, "origin", "", logger)

	}

}
