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
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	for i, d := range os.Args {
		// os.Args[0] is the path to this executable, so we should skip it
		if i == 0 {
			continue
		}

		ws := cleanPath("/workspace/")
		p := cleanPath(d)

		if !filepath.IsAbs(p) || strings.HasPrefix(p, ws+string(filepath.Separator)) {
			if err := os.MkdirAll(p, 0755); err != nil {
				log.Fatalf("Failed to mkdir %q: %v", p, err)
			}
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
