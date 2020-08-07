package pipeline_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
)

func TestValidate(t *testing.T) {
	valid := pipeline.Images{
		EntrypointImage:          "set",
		NopImage:                 "set",
		GitImage:                 "set",
		CredsImage:               "set",
		KubeconfigWriterImage:    "set",
		ShellImage:               "set",
		GsutilImage:              "set",
		BuildGCSFetcherImage:     "set",
		PRImage:                  "set",
		ImageDigestExporterImage: "set",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid Images returned error: %v", err)
	}

	invalid := pipeline.Images{
		EntrypointImage:          "set",
		NopImage:                 "set",
		GitImage:                 "", // unset!
		CredsImage:               "set",
		KubeconfigWriterImage:    "set",
		ShellImage:               "", // unset!
		GsutilImage:              "set",
		BuildGCSFetcherImage:     "", // unset!
		PRImage:                  "", // unset!
		ImageDigestExporterImage: "set",
	}
	wantErr := "found unset image flags: [build-gcs-fetcher git pr shell]"
	if err := invalid.Validate(); err == nil {
		t.Error("invalid Images expected error, got nil")
	} else if err.Error() != wantErr {
		t.Errorf("Unexpected error message: got %q, want %q", err.Error(), wantErr)
	}
}
