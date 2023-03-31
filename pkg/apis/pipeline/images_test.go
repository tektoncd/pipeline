/*
Copyright 2023 The Tekton Authors

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

package pipeline_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
)

func TestValidate(t *testing.T) {
	valid := pipeline.Images{
		EntrypointImage:        "set",
		SidecarLogResultsImage: "set",
		NopImage:               "set",
		ShellImage:             "set",
		ShellImageWin:          "set",
		WorkingDirInitImage:    "set",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid Images returned error: %v", err)
	}

	invalid := pipeline.Images{
		EntrypointImage:        "set",
		SidecarLogResultsImage: "set",
		NopImage:               "set",
		ShellImage:             "", // unset!
		ShellImageWin:          "set",
	}
	wantErr := "found unset image flags: [shell-image workingdirinit-image]"
	if err := invalid.Validate(); err == nil {
		t.Error("invalid Images expected error, got nil")
	} else if err.Error() != wantErr {
		t.Errorf("Unexpected error message: got %q, want %q", err.Error(), wantErr)
	}
}
