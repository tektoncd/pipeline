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
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"go.uber.org/zap"
)

func run(logger *zap.SugaredLogger, dir string, args ...string) (string, error) {
	c := exec.Command("git", args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output
	// This is the optional working directory. If not set, it defaults to the current
	// working directory of the process.
	if dir != "" {
		c.Dir = dir
	}
	if err := c.Run(); err != nil {
		logger.Errorf("Error running git %v: %v\n%v", args, err, output.String())
		return "", err
	}
	return output.String(), nil
}

// FetchSpec describes how to initialize and fetch from a Git repository.
type FetchSpec struct {
	URL       string
	Revision  string
	Path      string
	Depth     uint
	SSLVerify bool
}

// Fetch fetches the specified git repository at the revision into path.
func Fetch(logger *zap.SugaredLogger, spec FetchSpec) error {
	if err := ensureHomeEnv(logger); err != nil {
		return err
	}

	if spec.Revision == "" {
		spec.Revision = "master"
	}
	if spec.Path != "" {
		if _, err := run(logger, "", "init", spec.Path); err != nil {
			return err
		}
		if err := os.Chdir(spec.Path); err != nil {
			return fmt.Errorf("failed to change directory with path %s; err: %w", spec.Path, err)
		}
	} else if _, err := run(logger, "", "init"); err != nil {
		return err
	}
	trimmedURL := strings.TrimSpace(spec.URL)
	if _, err := run(logger, "", "remote", "add", "origin", trimmedURL); err != nil {
		return err
	}
	if _, err := run(logger, "", "config", "--global", "http.sslVerify", strconv.FormatBool(spec.SSLVerify)); err != nil {
		logger.Warnf("Failed to set http.sslVerify in git config: %s", err)
		return err
	}

	fetchArgs := []string{"fetch", "--recurse-submodules=yes"}
	if spec.Depth > 0 {
		fetchArgs = append(fetchArgs, fmt.Sprintf("--depth=%d", spec.Depth))
	}
	fetchArgs = append(fetchArgs, "origin", spec.Revision)

	if _, err := run(logger, "", fetchArgs...); err != nil {
		// Fetch can fail if an old commitid was used so try git pull, performing regardless of error
		// as no guarantee that the same error is returned by all git servers gitlab, github etc...
		if _, err := run(logger, "", "pull", "--recurse-submodules=yes", "origin"); err != nil {
			logger.Warnf("Failed to pull origin : %s", err)
		}
		if _, err := run(logger, "", "checkout", spec.Revision); err != nil {
			return err
		}
	} else if _, err := run(logger, "", "reset", "--hard", "FETCH_HEAD"); err != nil {
		return err
	}
	logger.Infof("Successfully cloned %s @ %s in path %s", trimmedURL, spec.Revision, spec.Path)
	return nil
}

func Commit(logger *zap.SugaredLogger, revision, path string) (string, error) {
	output, err := run(logger, path, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(output, "\n"), nil
}

func SubmoduleFetch(logger *zap.SugaredLogger, path string) error {
	if err := ensureHomeEnv(logger); err != nil {
		return err
	}

	if path != "" {
		if err := os.Chdir(path); err != nil {
			return fmt.Errorf("failed to change directory with path %s; err: %w", path, err)
		}
	}
	if _, err := run(logger, "", "submodule", "init"); err != nil {
		return err
	}
	if _, err := run(logger, "", "submodule", "update", "--recursive"); err != nil {
		return err
	}
	logger.Infof("Successfully initialized and updated submodules in path %s", path)
	return nil
}

func ensureHomeEnv(logger *zap.SugaredLogger) error {
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
	return nil
}
