// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package licenseclassifier

import (
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	// LicenseDirectory is the directory where the prototype licenses are kept.
	LicenseDirectory = "src/github.com/google/licenseclassifier/licenses"
	// LicenseArchive is the name of the archive containing preprocessed
	// license texts.
	LicenseArchive = "licenses.db"
	// ForbiddenLicenseArchive is the name of the archive containing preprocessed
	// forbidden license texts only.
	ForbiddenLicenseArchive = "forbidden_licenses.db"
)

func findInGOPATH(rel string) (fullPath string, err error) {
	for _, path := range filepath.SplitList(build.Default.GOPATH) {
		fullPath := filepath.Join(path, rel)
		if _, err := os.Stat(fullPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		return fullPath, nil
	}
	return "", nil
}

// ReadLicenseFile locates and reads the license file.
func ReadLicenseFile(filename string) ([]byte, error) {
	archive, err := findInGOPATH(filepath.Join(LicenseDirectory, filename))
	if err != nil || archive == "" {
		return nil, err
	}
	return ioutil.ReadFile(archive)
}

// ReadLicenseDir reads directory containing the license files.
func ReadLicenseDir() ([]os.FileInfo, error) {
	filename, err := findInGOPATH(filepath.Join(LicenseDirectory, LicenseArchive))
	if err != nil || filename == "" {
		return nil, err
	}
	return ioutil.ReadDir(filepath.Dir(filename))
}
