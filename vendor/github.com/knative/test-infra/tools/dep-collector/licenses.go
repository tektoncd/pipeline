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
	"fmt"
	gb "go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/licenseclassifier"
)

var LicenseNames = []string{
	"LICENCE",
	"LICENSE",
	"LICENSE.code",
	"LICENSE.md",
	"LICENSE.txt",
	"COPYING",
	"copyright",
}

const MatchThreshold = 0.9

type LicenseFile struct {
	EnclosingImportPath string
	LicensePath         string
}

func (lf *LicenseFile) Body() (string, error) {
	body, err := ioutil.ReadFile(lf.LicensePath)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (lt *LicenseFile) Classify(classifier *licenseclassifier.License) (string, error) {
	body, err := lt.Body()
	if err != nil {
		return "", err
	}
	m := classifier.NearestMatch(body)
	if m == nil {
		return "", fmt.Errorf("unable to classify license: %v", lt.EnclosingImportPath)
	}
	return m.Name, nil
}

func (lt *LicenseFile) Check(classifier *licenseclassifier.License) error {
	body, err := lt.Body()
	if err != nil {
		return err
	}
	ms := classifier.MultipleMatch(body, false)
	for _, m := range ms {
		return fmt.Errorf("Found matching forbidden license in %q: %v", lt.EnclosingImportPath, m.Name)
	}
	return nil
}

func (lt *LicenseFile) Entry() (string, error) {
	body, err := lt.Body()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`
===========================================================
Import: %s

%s
`, lt.EnclosingImportPath, body), nil
}

func (lt *LicenseFile) CSVRow(classifier *licenseclassifier.License) (string, error) {
	classification, err := lt.Classify(classifier)
	if err != nil {
		return "", err
	}
	parts := strings.Split(lt.EnclosingImportPath, "/vendor/")
	if len(parts) != 2 {
		return "", fmt.Errorf("wrong number of parts splitting import path on %q : %q", "/vendor/", lt.EnclosingImportPath)
	}
	return strings.Join([]string{
		parts[1],
		"Static",
		"", // TODO(mattmoor): Modifications?
		"https://" + parts[0] + "/blob/master/vendor/" + parts[1] + "/" + filepath.Base(lt.LicensePath),
		classification,
	}, ","), nil
}

func findLicense(ip string) (*LicenseFile, error) {
	pkg, err := gb.Import(ip, WorkingDir, gb.ImportComment)
	if err != nil {
		return nil, err
	}
	dir := pkg.Dir

	for {
		// When we reach the root of our workspace, stop searching.
		if dir == WorkingDir {
			return nil, fmt.Errorf("unable to find license for %q", pkg.ImportPath)
		}

		for _, name := range LicenseNames {
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); err != nil {
				continue
			}

			return &LicenseFile{
				EnclosingImportPath: ip,
				LicensePath:         p,
			}, nil
		}

		// Walk to the parent directory / import path
		dir = filepath.Dir(dir)
		ip = filepath.Dir(ip)
	}
}

type LicenseCollection []*LicenseFile

func (lc LicenseCollection) Entries() (string, error) {
	sections := make([]string, 0, len(lc))
	for _, key := range lc {
		entry, err := key.Entry()
		if err != nil {
			return "", err
		}
		sections = append(sections, entry)
	}
	return strings.Join(sections, "\n"), nil
}

func (lc LicenseCollection) CSV(classifier *licenseclassifier.License) (string, error) {
	sections := make([]string, 0, len(lc))
	for _, entry := range lc {
		row, err := entry.CSVRow(classifier)
		if err != nil {
			return "", err
		}
		sections = append(sections, row)
	}
	return strings.Join(sections, "\n"), nil
}

func (lc LicenseCollection) Check(classifier *licenseclassifier.License) error {
	errors := []string{}
	for _, entry := range lc {
		if err := entry.Check(classifier); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf("Errors validating licenses:\n%v", strings.Join(errors, "\n"))
}

func CollectLicenses(imports []string) (LicenseCollection, error) {
	// for each of the import paths, search for a license file.
	licenseFiles := make(map[string]*LicenseFile)
	for _, ip := range imports {
		lf, err := findLicense(ip)
		if err != nil {
			return nil, err
		}
		licenseFiles[lf.EnclosingImportPath] = lf
	}

	order := sort.StringSlice{}
	for key := range licenseFiles {
		order = append(order, key)
	}
	order.Sort()

	licenseTypes := LicenseCollection{}
	for _, key := range order {
		licenseTypes = append(licenseTypes, licenseFiles[key])
	}
	return licenseTypes, nil
}
