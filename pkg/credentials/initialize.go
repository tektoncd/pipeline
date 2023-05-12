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

package credentials

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	corev1 "k8s.io/api/core/v1"
)

const (
	// credsDirPermissions are the persmission bits assigned to the directories
	// copied out of the /tekton/creds and into a Step's HOME.
	credsDirPermissions = 0700

	// credsFilePermissions are the persmission bits assigned to the files
	// copied out of /tekton/creds and into a Step's HOME.
	credsFilePermissions = 0600
)

// CredsInitCredentials is the complete list of credentials that the legacy credentials
// helper (aka "creds-init") can write to /tekton/creds.
var CredsInitCredentials = []string{".docker", ".gitconfig", ".git-credentials", ".ssh"}

// VolumePath is the path where build secrets are written.
// It is mutable and exported for testing.
var VolumePath = "/tekton/creds-secrets"

// Builder is the interface for a credential initializer of any type.
type Builder interface {
	// MatchingAnnotations extracts flags for the credential
	// helper from the supplied secret and returns a slice (of
	// length 0 or greater) of applicable domains.
	MatchingAnnotations(*corev1.Secret) []string

	// Write writes the credentials to the provided directory.
	Write(string) error
}

// VolumeName returns the full path to the secret, inside the VolumePath.
func VolumeName(secretName string) string {
	return fmt.Sprintf("%s/%s", VolumePath, secretName)
}

// SortAnnotations return sorted array of strings which has annotationPrefix
// as the prefix in secrets key
func SortAnnotations(secrets map[string]string, annotationPrefix string) []string {
	var mk []string
	for k, v := range secrets {
		if strings.HasPrefix(k, annotationPrefix) {
			mk = append(mk, v)
		}
	}
	sort.Strings(mk)
	return mk
}

// CopyCredsToHome copies credentials from the /tekton/creds directory into
// the current Step's HOME directory. A list of credential paths to try and
// copy is given as an arg, for example, []string{".docker", ".ssh"}. A missing
// /tekton/creds directory is not considered an error.
func CopyCredsToHome(credPaths []string) error {
	if info, err := os.Stat(pipeline.CredsDir); err != nil || !info.IsDir() {
		return nil //nolint:nilerr // safe to ignore error; no credentials available to copy
	}

	homepath, err := homedir.Dir()
	if err != nil {
		return fmt.Errorf("error getting the user's home directory: %w", err)
	}

	for _, cred := range credPaths {
		source := filepath.Join(pipeline.CredsDir, cred)
		destination := filepath.Join(homepath, cred)
		err := tryCopyCred(source, destination)
		if err != nil {
			log.Printf("warning: unsuccessful cred copy: %q from %q to %q: %v", cred, pipeline.CredsDir, homepath, err)
		}
	}
	return nil
}

// tryCopyCred will recursively copy a given source path to a given
// destination path. A missing source file is treated as normal behaviour
// and no error is returned.
func tryCopyCred(source, destination string) error {
	fromInfo, err := os.Lstat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("unable to read source file info: %w", err)
	}

	fromFile, err := os.Open(filepath.Clean(source))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("unable to open source: %w", err)
	}
	defer fromFile.Close()

	if fromInfo.IsDir() {
		err := os.MkdirAll(destination, credsDirPermissions)
		if err != nil {
			return fmt.Errorf("unable to create destination directory: %w", err)
		}
		subdirs, err := fromFile.Readdirnames(0)
		if err != nil {
			return fmt.Errorf("unable to read subdirectories of source: %w", err)
		}
		for _, subdir := range subdirs {
			src := filepath.Join(source, subdir)
			dst := filepath.Join(destination, subdir)
			if err := tryCopyCred(src, dst); err != nil {
				return err
			}
		}
	} else {
		flags := os.O_RDWR | os.O_CREATE | os.O_TRUNC
		toFile, err := os.OpenFile(destination, flags, credsFilePermissions)
		if err != nil {
			return fmt.Errorf("unable to open destination: %w", err)
		}
		defer toFile.Close()

		_, err = io.Copy(toFile, fromFile)
		if err != nil {
			return fmt.Errorf("error copying from source to destination: %w", err)
		}
	}
	return nil
}
