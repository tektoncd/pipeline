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

package pullrequest

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewSCMHandler(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		skipTLSVerify bool
		wantBaseURL   string
		wantRepo      string
		wantNum       int
		wantErr       bool
	}{
		{
			name:          "github",
			raw:           "https://github.com/foo/bar/pull/1",
			skipTLSVerify: false,
			wantBaseURL:   "https://api.github.com/",
			wantRepo:      "foo/bar",
			wantNum:       1,
			wantErr:       false,
		},
		{
			name:          "custom github",
			raw:           "https://github.tekton.dev/foo/baz/pull/2",
			skipTLSVerify: false,
			wantBaseURL:   "https://github.tekton.dev/api/v3/",
			wantRepo:      "foo/baz",
			wantNum:       2,
			wantErr:       false,
		},
		{
			name:          "gitlab",
			raw:           "https://gitlab.com/foo/bar/merge_requests/3",
			skipTLSVerify: false,
			wantBaseURL:   "https://gitlab.com/",
			wantRepo:      "foo/bar",
			wantNum:       3,
			wantErr:       false,
		},
		{
			name:          "gitlab multi-level",
			raw:           "https://gitlab.com/foo/bar/baz/merge_requests/3",
			skipTLSVerify: true,
			wantBaseURL:   "https://gitlab.com/",
			wantRepo:      "foo/bar/baz",
			wantNum:       3,
			wantErr:       false,
		},
		{
			name:          "unsupported",
			raw:           "https://unsupported.com/foo/baz/merge_requests/3",
			skipTLSVerify: true,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, _ := observer.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			got, err := NewSCMHandler(logger, tt.raw, "", "", tt.skipTLSVerify)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("NewSCMHandler() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if got.prNum != tt.wantNum {
				t.Errorf("NewSCMHandler() [pr num] = %v, want %v", got, tt.wantNum)
			}
			if got.repo != tt.wantRepo {
				t.Errorf("NewSCMHandler() [repo] = %v, want %v", got, tt.wantRepo)
			}
			if baseURL := got.client.BaseURL.String(); baseURL != tt.wantBaseURL {
				t.Errorf("NewSCMHandler() [base url] = %v, want %v", baseURL, tt.wantBaseURL)
			}
			switch transport := got.client.Client.Transport.(type) {
			case *http.Transport:
				if transport.TLSClientConfig.InsecureSkipVerify != tt.skipTLSVerify {
					t.Errorf("NewSCMHandler() [InsecureSkipVerify] = %v, want %v", strconv.FormatBool(transport.TLSClientConfig.InsecureSkipVerify), tt.skipTLSVerify)
				}
			default:
				t.Errorf("NewSCMHandler() client.Client.Transport was not of type http.Transport")
			}
		})
	}
}

func TestGuessProvider(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "github",
			url:  "https://github.com/foo/bar",
			want: "github",
		},
		{
			name: "nested github",
			url:  "https://github.foo.com/foo/bar",
			want: "github",
		},
		{
			name: "gitlab",
			url:  "https://gitlab.com/foo/bar",
			want: "gitlab",
		},
		{
			name: "nested gitlab",
			url:  "https://gitlab.foo.com/foo/bar",
			want: "gitlab",
		},
		{
			name:    "err",
			url:     "https://foo.com/foo/bar",
			wantErr: true,
		},
		{
			name:    "gitlab out of host",
			url:     "https://foo.com/foo/github",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			if err != nil {
				t.Errorf("parsing url: %v, err: %v", tt.url, err)
			}
			got, err := guessProvider(u)
			if (err != nil) != tt.wantErr {
				t.Errorf("guessProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("guessProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}
