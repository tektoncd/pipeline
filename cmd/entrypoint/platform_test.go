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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSelectCommandForPlatform(t *testing.T) {
	for _, c := range []struct {
		desc    string
		m       map[string][]string
		plat    string
		want    []string
		wantErr bool
	}{{
		desc: "platform exists",
		m: map[string][]string{
			"linux/amd64": {"my", "command"},
			"linux/s390x": {"other", "one"},
		},
		plat: "linux/amd64",
		want: []string{"my", "command"},
	}, {
		desc: "platform not found",
		m: map[string][]string{
			"linux/amd64": {"my", "command"},
			"linux/s390x": {"other", "one"},
		},
		plat:    "linux/ppc64le",
		wantErr: true,
	}, {
		desc: "platform fallback",
		m: map[string][]string{
			"linux/amd64": {"my", "command"},
			"linux/s390x": {"other", "one"},
		},
		plat: "linux/amd64/v8",
		want: []string{"my", "command"},
	}, {
		desc: "platform fallback not needed",
		m: map[string][]string{
			"linux/amd64":    {"other", "one"},
			"linux/amd64/v8": {"my", "command"},
		},
		plat: "linux/amd64/v8",
		want: []string{"my", "command"},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got, err := selectCommandForPlatform(c.m, c.plat)
			if err != nil {
				if c.wantErr {
					return
				}
				t.Fatalf("Unexpected error: %v", err)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("Diff(-want,+got):\n%s", d)
			}
		})
	}
}
