// +build e2e

// Copyright © 2018 The Tekton Authors
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

package pkg

import (
	"testing"

	_ "github.com/tektoncd/plumbing/scripts"
)

func TestDummy(t *testing.T) {
	t.Log("This is required to make sure we get tektoncd/plumbing in the repostiory, folder vendor")
}
