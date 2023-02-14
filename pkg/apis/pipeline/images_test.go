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
