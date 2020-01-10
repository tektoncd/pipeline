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

package system

import (
	"os"
	"testing"
)

func TestGetNamespace(t *testing.T) {
	testcases := []struct {
		envVar       string
		expectEnvVar string
	}{{
		envVar:       "",
		expectEnvVar: DefaultNamespace,
	}, {
		envVar:       "test",
		expectEnvVar: "test",
	}}

	value := os.Getenv(SystemNamespaceEnvVar)
	defer func() { os.Setenv(SystemNamespaceEnvVar, value) }()

	for _, ts := range testcases {
		if err := os.Setenv(SystemNamespaceEnvVar, ts.envVar); err != nil {
			t.Fatalf("Failed to set ENV: %v", err)
		}

		if got, want := GetNamespace(), ts.expectEnvVar; got != want {
			t.Fatalf("Invalid namespace: got: %v, want: %v", got, want)
		}
	}
}
