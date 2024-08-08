/*
Copyright 2022 The Tekton Authors

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
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// exists returns whether the given file or directory exists
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getPermissions(path string) (fs.FileMode, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fs.FileMode(0o666), err
	}
	return fileInfo.Mode(), nil
}

func createPath(path string) error {
	// Check if the path already exists.
	ex, err := exists(path)
	if err != nil {
		return fmt.Errorf("error accessing path %s: %w", path, err)
	}
	// Path already exists, so no subdirectory to create. Skip the rest.
	if ex {
		return nil
	}
	// Find the innermost parent directory to get the permissions to apply to the child directories.
	parts := strings.Split(path, string(filepath.Separator))
	parent := path
	i := 0
	// If we reach the file system root then we could not find any existing Dir.
	// In this case, workingDir Init cannot handle it. Let k8s handle the creation instead.
	for parent != "/" {
		parent = filepath.Dir(parent)
		if ex, err := exists(parent); err != nil {
			return fmt.Errorf("error accessing path %s: %w", parent, err)
		} else if ex {
			// We need to get its permissions and apply it to the child directories.
			perm, err := getPermissions(parent)
			if err != nil {
				return fmt.Errorf("error accessing path %s: %w", parent, err)
			}
			// Create the path
			if err := os.MkdirAll(path, perm); err != nil {
				return fmt.Errorf("failed to mkdir %q: %w", path, err)
			}
			// walk up again and set the permissions of the parent to the child directories
			d := parent
			idx := len(parts) - i - 1
			for j := idx; j < len(parts); j++ {
				d = filepath.Join(d, parts[j])
				if err := os.Chmod(d, perm); err != nil {
					return fmt.Errorf("failed to chmod %q: %w", d, err)
				}
			}
			return nil
		}
		i += 1
	}
	return nil
}

func main() {
	for i, d := range os.Args {
		// os.Args[0] is the path to this executable, so we should skip it
		if i == 0 {
			continue
		}
		p := cleanPath(d)
		err := createPath(p)
		if err != nil {
			log.Fatalf("Failed to create path %s: %v", p, err)
		}
	}
}

func cleanPath(path string) string {
	p := filepath.Clean(path)

	if runtime.GOOS == "windows" {
		// Append 'C:' if the path is absolute (i.e. it begins with a single '\')
		if strings.HasPrefix(p, "\\") && !strings.HasPrefix(p, "\\\\") {
			p = "C:" + p
		}
	}

	return p
}
