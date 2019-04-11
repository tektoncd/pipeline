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
	"bytes"
	"flag"
	"os"
	"os/exec"
	"strings"

	"github.com/knative/pkg/logging"
	homedir "github.com/mitchellh/go-homedir"
	"go.uber.org/zap"
)

var (
	url      = flag.String("url", "", "The url of the Git repository to initialize.")
	revision = flag.String("revision", "", "The Git revision to make the repository HEAD")
	path     = flag.String("path", "", "Path of directory under which git repository will be copied")
)

func run(logger *zap.SugaredLogger, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output
	if err := c.Run(); err != nil {
		logger.Errorf("Error running %v %v: %v\n%v", cmd, args, err, output.String())
		return err
	}
	return nil
}

func runOrFail(logger *zap.SugaredLogger, cmd string, args ...string) {
	c := exec.Command(cmd, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output

	if err := c.Run(); err != nil {
		logger.Fatalf("Unexpected error running %v %v: %v\n%v", cmd, args, err, output.String())
	}
}

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "git-init")
	defer logger.Sync()

	// HACK: This is to get git+ssh to work since ssh doesn't respect the HOME
	// env variable.
	homepath, err := homedir.Dir()
	if err != nil {
		logger.Fatalf("Unexpected error: getting the user home directory: %v", err)
	}
	homeenv := os.Getenv("HOME")
	if homeenv != "" && homeenv != homepath {
		if _, err := os.Stat(homepath + "/.ssh"); os.IsNotExist(err) {
			err = os.Symlink(homeenv+"/.ssh", homepath+"/.ssh")
			if err != nil {
				// Only do a warning, in case we don't have a real home
				// directory writable in our image
				logger.Warnf("Unexpected error: creating symlink: %v", err)
			}
		}
	}

	if *revision == "" {
		*revision = "master"
	}
	if *path != "" {
		runOrFail(logger, "git", "init", *path)
		if err := os.Chdir(*path); err != nil {
			logger.Fatalf("Failed to change directory with path %s; err %v", path, err)
		}
	} else {
		runOrFail(logger, "git", "init")
	}
	trimmedURL := strings.TrimSpace(*url)
	runOrFail(logger, "git", "remote", "add", "origin", trimmedURL)
	if err := run(logger, "git", "fetch", "--depth=1", "--recurse-submodules=yes", "origin", *revision); err != nil {
		// Fetch can fail if an old commitid was used so try git pull, performing regardless of error
		// as no guarantee that the same error is returned by all git servers gitlab, github etc...
		if err := run(logger, "git", "pull", "--recurse-submodules=yes", "origin"); err != nil {
			logger.Warnf("Failed to pull origin : %s", err)
		}
		runOrFail(logger, "git", "checkout", *revision)
	} else {
		runOrFail(logger, "git", "reset", "--hard", "FETCH_HEAD")
	}

	logger.Infof("Successfully cloned %q @ %q in path %q", trimmedURL, *revision, *path)
}
