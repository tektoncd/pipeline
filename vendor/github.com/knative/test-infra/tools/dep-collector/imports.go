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
	"path/filepath"
	"sort"
	"strings"
)

func CollectTransitiveImports(binaries []string) ([]string, error) {
	// Perform a simple DFS to collect the binaries' transitive dependencies.
	visited := make(map[string]struct{})
	for _, importpath := range binaries {
		if gb.IsLocalImport(importpath) {
			ip, err := qualifyLocalImport(importpath)
			if err != nil {
				return nil, err
			}
			importpath = ip
		}

		pkg, err := gb.Import(importpath, WorkingDir, gb.ImportComment)
		if err != nil {
			return nil, err
		}
		if err := visit(pkg, visited); err != nil {
			return nil, err
		}
	}

	// Sort the dependencies deterministically.
	var list sort.StringSlice
	for ip := range visited {
		if !strings.Contains(ip, "/vendor/") {
			// Skip files outside of vendor
			continue
		}
		list = append(list, ip)
	}
	list.Sort()

	return list, nil
}

func qualifyLocalImport(ip string) (string, error) {
	gopathsrc := filepath.Join(gb.Default.GOPATH, "src")
	if !strings.HasPrefix(WorkingDir, gopathsrc) {
		return "", fmt.Errorf("working directory must be on ${GOPATH}/src = %s", gopathsrc)
	}
	return filepath.Join(strings.TrimPrefix(WorkingDir, gopathsrc+string(filepath.Separator)), ip), nil
}

func visit(pkg *gb.Package, visited map[string]struct{}) error {
	if _, ok := visited[pkg.ImportPath]; ok {
		return nil
	}
	visited[pkg.ImportPath] = struct{}{}

	for _, ip := range pkg.Imports {
		if ip == "C" {
			// skip cgo
			continue
		}
		subpkg, err := gb.Import(ip, WorkingDir, gb.ImportComment)
		if err != nil {
			return fmt.Errorf("%v\n -> %v", pkg.ImportPath, err)
		}
		if !strings.HasPrefix(subpkg.Dir, WorkingDir) {
			// Skip import paths outside of our workspace (std library)
			continue
		}
		if err := visit(subpkg, visited); err != nil {
			return fmt.Errorf("%v (%v)\n -> %v", pkg.ImportPath, pkg.Dir, err)
		}
	}
	return nil
}
