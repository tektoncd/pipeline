/*
Copyright 2024 The Tekton Authors

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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestArtifactsMerge(t *testing.T) {
	type testCase struct {
		name     string
		a1       Artifacts
		a2       Artifacts
		expected Artifacts
	}

	testCases := []testCase{
		{
			name: "Merges inputs and outputs with deduplication",
			a1: Artifacts{
				Inputs: []Artifact{
					{
						Name: "input1",
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
								Uri:    "pkg:maven/org.apache.commons/commons-lang3/3.12.0",
							},
						},
					},
					{
						Name: "input2",
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "d596377f2d54b3f8b4619f137d08892989893b886742759144582c94157526f1"},
								Uri:    "pkg:pypi/requests/2.28.2",
							},
						},
					},
				},
				Outputs: []Artifact{
					{
						Name: "output1",
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "47de7a85905970a45132f48a9247879a15c483477e23a637504694e611135b40e"},
								Uri:    "pkg:npm/lodash/4.17.21",
							},
						},
					},
				},
			},
			a2: Artifacts{
				Inputs: []Artifact{
					{
						Name: "input1",
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
								Uri:    "pkg:maven/org.apache.commons/commons-lang3/3.12.0",
							},
							{
								Digest: map[Algorithm]string{"sha256": "97c13e1812b666824266111701398e56e30d14418a2d9b35987f516a66e2129f"},
								Uri:    "pkg:nuget/Microsoft.Extensions.Logging/7.0.0",
							},
						},
					},
					{
						Name: "input3",
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "13c2b709e3a100726680e53e19666656a89a2f2490e917ba15d6b15475ab7b79"},
								Uri:    "pkg:debian/openssl/1.1.1",
							},
						},
					},
				},
				Outputs: []Artifact{
					{
						Name:        "output1",
						BuildOutput: true,
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "698c4539633943f7889f41605003d7fa63833722ebd2b37c7e75df1d3d06941a"},
								Uri:    "pkg:nuget/Newtonsoft.Json/13.0.3",
							},
						},
					},
					{
						Name:        "output2",
						BuildOutput: true,
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "7e406d83706c7193df3e38b66d350e55df6f13d2a28a1d35917a043533a70f5c"},
								Uri:    "pkg:pypi/pandas/2.0.1",
							},
						},
					},
				},
			},
			expected: Artifacts{
				Inputs: []Artifact{
					{
						Name: "input1",
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
								Uri:    "pkg:maven/org.apache.commons/commons-lang3/3.12.0",
							},
							{
								Digest: map[Algorithm]string{"sha256": "97c13e1812b666824266111701398e56e30d14418a2d9b35987f516a66e2129f"},
								Uri:    "pkg:nuget/Microsoft.Extensions.Logging/7.0.0",
							},
						},
					},
					{
						Name: "input2",
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "d596377f2d54b3f8b4619f137d08892989893b886742759144582c94157526f1"},
								Uri:    "pkg:pypi/requests/2.28.2",
							},
						},
					},
					{
						Name: "input3",
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "13c2b709e3a100726680e53e19666656a89a2f2490e917ba15d6b15475ab7b79"},
								Uri:    "pkg:debian/openssl/1.1.1",
							},
						},
					},
				},
				Outputs: []Artifact{
					{
						Name:        "output1",
						BuildOutput: true,
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "47de7a85905970a45132f48a9247879a15c483477e23a637504694e611135b40e"},
								Uri:    "pkg:npm/lodash/4.17.21",
							},
							{
								Digest: map[Algorithm]string{"sha256": "698c4539633943f7889f41605003d7fa63833722ebd2b37c7e75df1d3d06941a"},
								Uri:    "pkg:nuget/Newtonsoft.Json/13.0.3",
							},
						},
					},
					{
						Name:        "output2",
						BuildOutput: true,
						Values: []ArtifactValue{
							{
								Digest: map[Algorithm]string{"sha256": "7e406d83706c7193df3e38b66d350e55df6f13d2a28a1d35917a043533a70f5c"},
								Uri:    "pkg:pypi/pandas/2.0.1",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.a1.Merge(&tc.a2)
			got := tc.a1
			if d := cmp.Diff(tc.expected, got, cmpopts.SortSlices(func(a, b Artifact) bool { return a.Name > b.Name })); d != "" {
				t.Errorf("TestArtifactsMerge() did not produce expected artifacts for test %s: %s", tc.name, diff.PrintWantGot(d))
			}
		})
	}
}
