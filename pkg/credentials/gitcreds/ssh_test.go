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

package gitcreds

import (
	"testing"
)

func TestSSHKeyScanArgs(t *testing.T) {
	for _, tc := range []struct {
		url          string
		expectedHost string
		expectedPort string
	}{{
		url:          "github.com",
		expectedHost: "github.com",
	}, {
		url:          "customdomain.com",
		expectedHost: "customdomain.com",
	}, {
		url:          "customdomain.com:22",
		expectedHost: "customdomain.com",
		expectedPort: "22",
	}, {
		url:          "git.customdomain.com:7779",
		expectedHost: "git.customdomain.com",
		expectedPort: "7779",
	}} {
		host, port := sshKeyScanArgs(tc.url)
		if host != tc.expectedHost {
			t.Errorf("expected host %q received %q", tc.expectedHost, host)
		}
		if port != tc.expectedPort {
			t.Errorf("expected port %q received %q", tc.expectedPort, port)
		}
	}
}
