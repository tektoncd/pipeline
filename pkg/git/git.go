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
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"go.uber.org/zap"
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

// Fetch fetches the specified git repository at the revision into path.
func Fetch(logger *zap.SugaredLogger, revision, path, url string) error {
	// HACK: This is to get git+ssh to work since ssh doesn't respect the HOME
	// env variable.
	homepath, err := homedir.Dir()
	if err != nil {
		logger.Errorf("Unexpected error: getting the user home directory: %v", err)
		return err
	}
	homeenv := os.Getenv("HOME")
	euid := os.Geteuid()
	// Special case the root user/directory
	if euid == 0 {
		if err := os.Symlink(homeenv+"/.ssh", "/root/.ssh"); err != nil {
			// Only do a warning, in case we don't have a real home
			// directory writable in our image
			logger.Warnf("Unexpected error: creating symlink: %v", err)
		}
	} else if homeenv != "" && homeenv != homepath {
		if _, err := os.Stat(homepath + "/.ssh"); os.IsNotExist(err) {
			if err := os.Symlink(homeenv+"/.ssh", homepath+"/.ssh"); err != nil {
				// Only do a warning, in case we don't have a real home
				// directory writable in our image
				logger.Warnf("Unexpected error: creating symlink: %v", err)
			}
		}
	}

	if revision == "" {
		revision = "master"
	}
	if path != "" {
		if err := run(logger, "git", "init", path); err != nil {
			return err
		}
		if err := os.Chdir(path); err != nil {
			return fmt.Errorf("Failed to change directory with path %s; err %v", path, err)
		}
	} else {
		if err := run(logger, "git", "init"); err != nil {
			return err
		}
	}
	trimmedURL := strings.TrimSpace(url)
	if err := run(logger, "git", "remote", "add", "origin", trimmedURL); err != nil {
		return err
	}
	if err := run(logger, "git", "fetch", "--depth=1", "--recurse-submodules=yes", "origin", revision); err != nil {
		// Fetch can fail if an old commitid was used so try git pull, performing regardless of error
		// as no guarantee that the same error is returned by all git servers gitlab, github etc...
		if err := run(logger, "git", "pull", "--recurse-submodules=yes", "origin"); err != nil {
			logger.Warnf("Failed to pull origin : %s", err)
		}
		if err := run(logger, "git", "checkout", revision); err != nil {
			return err
		}
	} else {
		if err := run(logger, "git", "reset", "--hard", "FETCH_HEAD"); err != nil {
			return err
		}
	}
	logger.Infof("Successfully cloned %s @ %s in path %s", trimmedURL, revision, path)
	return nil
}
