// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package version

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tektoncd/cli/pkg/test"
)

func TestVersion(t *testing.T) {
	v := clientVersion
	defer func() { clientVersion = v }()

	clientVersion = "v0.0.1"
	version := Command()

	scenarios := []struct {
		name     string
		flag     string
		expected string
	}{
		{
			name:     "no flags",
			flag:     "",
			expected: fmt.Sprintf("Client version: %s\n", clientVersion),
		},
		{
			name:     "with a check flag",
			flag:     "-c",
			expected: fmt.Sprintf("Client version: %s\n", clientVersion),
		},
	}

	for i, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			got, err := test.ExecuteCommand(version, "version", s.flag)

			if err != nil {
				t.Errorf("#%d Unexpected error: %v", i, err)
			}

			if !strings.HasPrefix(got, s.expected) {
				t.Errorf("#%d Invalid output: %v", i, got)
			}
		})
	}
}
