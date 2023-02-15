/*
Copyright 2022 The Knative Authors

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

package changeset

import (
	"regexp"
	"runtime/debug"
	"strconv"
	"sync"
)

const Unknown = "unknown"

var (
	shaRegexp = regexp.MustCompile(`^[a-f0-9]{40,64}$`)
	rev       string
	once      sync.Once

	readBuildInfo = debug.ReadBuildInfo
)

// Get returns the 'vcs.revision' property from the embedded build information
// If there is no embedded information 'unknown' will be returned
//
// The result will have a '-dirty' suffix if the workspace was not clean
func Get() string {
	once.Do(func() {
		if rev == "" {
			rev = get()
		}
		// It has been set through ldflags, do nothing
	})

	return rev
}

func get() string {
	info, ok := readBuildInfo()
	if !ok {
		return Unknown
	}

	var revision string
	var modified bool

	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified, _ = strconv.ParseBool(s.Value)
		}
	}

	if revision == "" {
		return Unknown
	}

	if shaRegexp.MatchString(revision) {
		revision = revision[:7]
	}

	if modified {
		revision += "-dirty"
	}

	return revision
}
