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

package git_test

import (
	"github.com/tektoncd/pipeline/pkg/remote/git"
	"reflect"
	"testing"
)

func TestParseGitURI(t *testing.T) {
	testCases := []struct {
		text       string
		want       *git.GitURI
		wantErr    bool
		wantString string
	}{
		{
			text: "myowner/myrepo@v1",
			want: &git.GitURI{
				Owner:      "myowner",
				Repository: "myrepo",
				Path:       "",
				SHA:        "v1",
			},
		},
		{
			text: "myowner/myrepo/@v1",
			want: &git.GitURI{
				Owner:      "myowner",
				Repository: "myrepo",
				Path:       "",
				SHA:        "v1",
			},
			wantString: "myowner/myrepo@v1",
		},
		{
			text: "myowner/myrepo/myfile.yaml@v1",
			want: &git.GitURI{
				Owner:      "myowner",
				Repository: "myrepo",
				Path:       "myfile.yaml",
				SHA:        "v1",
			},
		},
		{
			text: "myowner/myrepo/javascript/pullrequest.yaml@v1",
			want: &git.GitURI{
				Owner:      "myowner",
				Repository: "myrepo",
				Path:       "javascript/pullrequest.yaml",
				SHA:        "v1",
			},
		},
		{
			text: "foo.yaml",
			want: nil,
		},
		{
			text: "foo/bar/thingy.yaml",
			want: nil,
		},
	}

	for _, tc := range testCases {
		text := tc.text
		got, err := git.ParseGitURI(text)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("should have failed to parse %s", text)
			} else {
				t.Logf("parsing %s got want error: %s\n", text, err.Error())
			}
		} else {
			if err != nil {
				t.Fatalf("failed to parse %s", text)
			}
			if !reflect.DeepEqual(tc.want, got) {
				t.Fatalf("ParseGitURI(%s) got %s ; want %s", text, got, tc.want)
			}
			if tc.want != nil {
				gotString := tc.want.String()
				wantString := tc.wantString
				if wantString == "" {
					wantString = text
				}

				if gotString != wantString {
					t.Fatalf("ParseGitURI().String() got %s ; want %s", gotString, tc.wantString)
				}
			}
		}
	}
}
