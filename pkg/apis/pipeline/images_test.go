package pipeline_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
)

func TestValidate(t *testing.T) {
	valid := pipeline.Images{
		EntrypointImage:          "set",
		SidecarLogResultsImage:   "set",
		NopImage:                 "set",
		GitImage:                 "set",
		ShellImage:               "set",
		ShellImageWin:            "set",
		GsutilImage:              "set",
		PRImage:                  "set",
		ImageDigestExporterImage: "set",
		WorkingDirInitImage:      "set",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid Images returned error: %v", err)
	}

	invalid := pipeline.Images{
		EntrypointImage:          "set",
		SidecarLogResultsImage:   "set",
		NopImage:                 "set",
		GitImage:                 "", // unset!
		ShellImage:               "", // unset!
		ShellImageWin:            "set",
		GsutilImage:              "set",
		PRImage:                  "", // unset!
		ImageDigestExporterImage: "set",
	}
	wantErr := "found unset image flags: [git-image pr-image shell-image workingdirinit-image]"
	if err := invalid.Validate(); err == nil {
		t.Error("invalid Images expected error, got nil")
	} else if err.Error() != wantErr {
		t.Errorf("Unexpected error message: got %q, want %q", err.Error(), wantErr)
	}
}
