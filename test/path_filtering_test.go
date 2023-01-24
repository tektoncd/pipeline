//go:build examples
// +build examples

/*
Copyright 2021 The Tekton Authors

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

package test

import (
	"strings"
	"testing"
)

func TestStablePathFilter(t *testing.T) {
	for _, tc := range []struct {
		path    string
		allowed bool
	}{{
		path:    "/test.yaml",
		allowed: true,
	}, {
		path:    "/alpha/test.yaml",
		allowed: false,
	}, {
		path:    "/beta/test.yaml",
		allowed: false,
	}, {
		path:    "/foo/test.yaml",
		allowed: true,
	}, {
		path:    "/v1alpha1/taskruns/test.yaml",
		allowed: true,
	}, {
		path:    "/v1alpha1/taskruns/alpha/test.yaml",
		allowed: false,
	}, {
		path:    "/v1beta1/taskruns/test.yaml",
		allowed: true,
	}, {
		path:    "/v1beta1/taskruns/alpha/test.yaml",
		allowed: false,
	}, {
		path:    "/v1beta1/taskruns/alpha/test.yaml",
		allowed: false,
	}, {
		path:    "/v1alpha1/pipelineruns/beta/test.yaml",
		allowed: false,
	}, {
		path:    "/v1alpha1/pipelineruns/beta/test.yaml",
		allowed: false,
	}} {
		name := strings.Replace(tc.path, "/", " ", -1)
		t.Run(name, func(t *testing.T) {
			if got := stablePathFilter(tc.path); got != tc.allowed {
				t.Errorf("path %q: want %t got %t", tc.path, tc.allowed, got)
			}
		})
	}
}

func TestAlphaPathFilter(t *testing.T) {
	for _, tc := range []struct {
		path string
	}{{
		path: "/test.yaml",
	}, {
		path: "/alpha/test.yaml",
	}, {
		path: "/foo/test.yaml",
	}, {
		path: "/v1alpha1/taskruns/test.yaml",
	}, {
		path: "/v1alpha1/taskruns/alpha/test.yaml",
	}, {
		path: "/v1beta1/taskruns/test.yaml",
	}, {
		path: "/v1beta1/taskruns/alpha/test.yaml",
	}, {
		path: "/v1beta1/taskruns/alpha/test.yaml",
	}, {
		path: "/v1alpha1/pipelineruns/beta/test.yaml",
	}, {
		path: "/v1alpha1/pipelineruns/beta/test.yaml",
	}} {
		name := strings.Replace(tc.path, "/", " ", -1)
		t.Run(name, func(t *testing.T) {
			if got := alphaPathFilter(tc.path); got != true {
				t.Errorf("path %q: want %t got %t", tc.path, true, got)
			}
		})
	}
}

func TestBetaPathFilter(t *testing.T) {
	for _, tc := range []struct {
		path    string
		allowed bool
	}{{
		path:    "/test.yaml",
		allowed: true,
	}, {
		path:    "/alpha/test.yaml",
		allowed: false,
	}, {
		path:    "/beta/test.yaml",
		allowed: true,
	}, {
		path:    "/foo/test.yaml",
		allowed: true,
	}, {
		path:    "/v1alpha1/taskruns/test.yaml",
		allowed: true,
	}, {
		path:    "/v1alpha1/taskruns/alpha/test.yaml",
		allowed: false,
	}, {
		path:    "/v1beta1/taskruns/test.yaml",
		allowed: true,
	}, {
		path:    "/v1beta1/taskruns/alpha/test.yaml",
		allowed: false,
	}, {
		path:    "/v1beta1/taskruns/alpha/test.yaml",
		allowed: false,
	}, {
		path:    "/v1alpha1/pipelineruns/beta/test.yaml",
		allowed: true,
	}, {
		path:    "/v1alpha1/pipelineruns/beta/test.yaml",
		allowed: true,
	}} {
		name := strings.Replace(tc.path, "/", " ", -1)
		t.Run(name, func(t *testing.T) {
			if got := betaPathFilter(tc.path); got != tc.allowed {
				t.Errorf("path %q: want %t got %t", tc.path, tc.allowed, got)
			}
		})
	}
}
