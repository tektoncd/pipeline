/*
Copyright 2020 The Knative Authors

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

package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const allUsersFullPermission = 0777

// CreateDir creates dir if does not exist.
// The created dir will have the permission bits as 0777, which means everyone can read/write/execute it.
func CreateDir(dirPath string) error {
	return CreateDirWithFileMode(dirPath, allUsersFullPermission)
}

// CreateDirWithFileMode creates dir if does not exist.
// The created dir will have the permission bits as perm, which is the standard Unix rwxrwxrwx permissions.
func CreateDirWithFileMode(dirPath string, perm os.FileMode) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err = os.MkdirAll(dirPath, perm); err != nil {
			return fmt.Errorf("error creating directory: %w", err)
		}
	}
	return nil
}

// GetRootDir gets directory of git root
func GetRootDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// ChdirToRoot change directory to git root dir
func ChdirToRoot() error {
	d, err := GetRootDir()
	if err != nil {
		return err
	}
	return os.Chdir(d)
}
