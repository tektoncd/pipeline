/*
Copyright 2020 The Tekton Authors

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
package git

import "testing"

func TestValidateGitSSHURLFormat(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{
			url:  "git@github.com:user/project.git",
			want: true,
		},
		{
			url:  "git@127.0.0.1:user/project.git",
			want: true,
		},
		{
			url:  "http://github.com/user/project.git",
			want: false,
		},
		{
			url:  "https://github.com/user/project.git",
			want: false,
		},
		{
			url:  "http://127.0.0.1/user/project.git",
			want: false,
		},
		{
			url:  "https://127.0.0.1/user/project.git",
			want: false,
		},
		{
			url:  "http://host.xz/path/to/repo.git/",
			want: false,
		},
		{
			url:  "https://host.xz/path/to/repo.git/",
			want: false,
		},
		{
			url:  "ssh://user@host.xz:port/path/to/repo.git/",
			want: true,
		},
		{
			url:  "ssh://user@host.xz/path/to/repo.git/",
			want: true,
		},
		{
			url:  "ssh://host.xz:port/path/to/repo.git/",
			want: true,
		},
		{
			url:  "ssh://host.xz/path/to/repo.git/",
			want: true,
		},
		{
			url:  "git://host.xz/path/to/repo.git/",
			want: false,
		},
		{
			url:  "/path/to/repo.git/",
			want: false,
		},
		{
			url:  "file://~/path/to/repo.git/",
			want: false,
		},
		{
			url:  "user@host.xz:/path/to/repo.git/",
			want: true,
		},
		{
			url:  "host.xz:/path/to/repo.git/",
			want: true,
		},
		{
			url:  "user@host.xz:path/to/repo.git",
			want: true,
		},
	}

	for _, tt := range tests {
		got := ValidateGitSSHURLFormat(tt.url)
		if got != tt.want {
			t.Errorf("Validate URL(%v)'s SSH format got %v, want %v", tt.url, got, tt.want)
		}
	}
}
